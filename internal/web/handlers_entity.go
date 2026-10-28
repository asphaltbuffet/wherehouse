package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// detailData is the view model for the entity detail pane.
type detailData struct {
	Entity            app.EntityResult
	DateAdded         string
	History           []app.HistoryResult
	CanAddChild       bool
	CanMarkMissing    bool
	IsMissing         bool          // true when Status == EntityStatusMissing
	CanReturn         bool          // borrowed or loaned
	ReturnConfirm     string        // non-empty only for borrowed: returning removes the entity
	CanLoan           bool          // ok/missing and not locked (ADR 0024)
	LoanBlockedReason string        // non-empty when Loan is shown disabled
	LoanForm          *loanFormData // non-nil re-opens the loan form (validation error)
	CanMove           bool          // not locked; ok or borrowed (ADR 0034)
	MoveBlockedReason string        // non-empty when Move is shown disabled
	MoveForm          *moveFormData // non-nil re-opens the move form (validation error)
	LockAction        attrAction    // Lock / Unlock button
	DiscreteAction    attrAction    // Mark discrete / Clear discrete button
	// CanRemove is false for borrowed: a borrowed entity exits via Return (ADR 0022).
	CanRemove           bool
	RemoveBlockedReason string          // non-empty when Remove is shown disabled
	RemoveForm          *removeFormData // non-nil re-opens the remove form (guard tripped at submit)
	Breadcrumbs         []Breadcrumb
	Error               string // populated by edit POST handlers; rendered inline above the detail dl
	Removed             bool   // read-only view of a removed entity: last location + history, no actions
}

// loanFormData is the view model for the inline loan form.
type loanFormData struct {
	EntityID  string
	Recipient string
	Note      string
	Error     string
}

// loanLockedReason is the tooltip on a disabled Loan button.
const loanLockedReason = "Locked entities cannot be loaned"

// moveFormData is the view model for the inline move form.
type moveFormData struct {
	EntityID string
	AtRoot   bool // hides the "move to root" choice: it would be a no-op
	Note     string
	Error    string
}

// Tooltips on a disabled Move button; each names the one action that unblocks it.
const (
	moveLockedReason  = "Locked entities cannot be moved"
	moveMissingReason = "Missing entities cannot be moved — mark found first"
	moveLoanedReason  = "Loaned entities cannot be moved — return it first"
)

func moveBlockedReason(e app.EntityResult) string {
	if e.Locked {
		return moveLockedReason
	}
	if e.Status == inventory.EntityStatusMissing {
		return moveMissingReason
	}
	if e.Status == inventory.EntityStatusLoaned {
		return moveLoanedReason
	}
	return ""
}

// removeFormData is the view model for the inline remove form.
type removeFormData struct {
	EntityID    string
	DisplayName string
	Note        string
	Error       string
}

// removeHasChildrenReason is the tooltip on a disabled Remove button (#287).
const removeHasChildrenReason = "Contains other entities — remove or move them first"

// attrAction is the view model for one attribute button on the detail panel.
// The button posts Field=Value: the explicit target, never a toggle, so a
// stale click fails with ErrNoChange instead of flipping the attribute.
type attrAction struct {
	EntityID      string
	Label         string
	Field         string // "locked" or "discrete"
	Value         string // "true" or "false"
	BlockedReason string // non-empty => rendered disabled with this tooltip
}

const (
	lockBlockedReason     = "Missing or loaned entities cannot be locked"
	discreteBlockedReason = "Entities with children cannot be marked discrete"
	attrStaleMessage      = "Already changed elsewhere; showing current state."
	attrFailedMessage     = "attribute change failed"
)

func lockAction(e app.EntityResult) attrAction {
	if e.Locked {
		return attrAction{EntityID: e.EntityID, Label: "Unlock", Field: "locked", Value: "false"}
	}
	a := attrAction{EntityID: e.EntityID, Label: "Lock", Field: "locked", Value: "true"}
	if e.Status != inventory.EntityStatusOk && e.Status != inventory.EntityStatusBorrowed {
		a.BlockedReason = lockBlockedReason
	}
	return a
}

// discreteAction builds the Mark/Clear discrete button. The caller sets
// BlockedReason when the entity has live children.
func discreteAction(e app.EntityResult) attrAction {
	if e.Discrete {
		return attrAction{EntityID: e.EntityID, Label: "Clear discrete", Field: "discrete", Value: "false"}
	}
	return attrAction{EntityID: e.EntityID, Label: "Mark discrete", Field: "discrete", Value: "true"}
}

// hasLiveChildren reports whether e has non-removed children. A discrete
// entity never does, so it skips the query. A failed count reports false: the
// buttons stay enabled and the app-layer guards still reject the write.
func (s *Server) hasLiveChildren(ctx context.Context, e app.EntityResult) bool {
	if e.Discrete {
		return false
	}
	has, err := s.cfg.App.HasLiveChildren(ctx, e.EntityID)
	if err != nil {
		s.cfg.Logger.Warn("count live children", "error", err, "entity_id", e.EntityID)
		return false
	}
	return has
}

// parseAttrForm reads locked and/or discrete as explicit booleans. ok is false
// when neither is present or a value does not parse.
func parseAttrForm(form url.Values) (app.SetAttributesRequest, bool) {
	var req app.SetAttributesRequest
	for _, f := range []struct {
		name string
		dst  **bool
	}{{"locked", &req.Locked}, {"discrete", &req.Discrete}} {
		raw, present := form[f.name]
		if !present {
			continue
		}
		v, err := strconv.ParseBool(raw[0])
		if err != nil {
			return app.SetAttributesRequest{}, false
		}
		*f.dst = &v
	}
	return req, req.Locked != nil || req.Discrete != nil
}

// handleAttr sets the attribute(s) posted by a detail-panel button. Failures
// re-read the entity and render its current state with an inline error.
func (s *Server) handleAttr(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entityID")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	req, ok := parseAttrForm(r.PostForm)
	if !ok {
		http.Error(w, "locked and/or discrete must be true or false", http.StatusBadRequest)
		return
	}
	req.EntityID = entityID
	req.ActorID = s.actorFor(r)

	_, setErr := s.cfg.App.SetAttributes(r.Context(), []app.SetAttributesRequest{req})

	fresh, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		s.serverError(w, "build detail data", err)
		return
	}
	if fresh.Entity.EntityID == "" {
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}

	switch {
	case setErr == nil:
	case errors.Is(setErr, app.ErrNoChange):
		s.cfg.Logger.Info("attribute unchanged: panel was stale", "error", setErr, "entity_id", entityID)
		fresh.Error = attrStaleMessage
	default:
		s.cfg.Logger.Error("set attributes", "error", setErr, "entity_id", entityID)
		fresh.Error = attrFailedMessage
	}

	s.renderDetailSection(w, r, fresh)
	if wantsFragment(r) {
		s.renderHTMLAppend(w, "tree_attrs_oob", fresh.Entity)
	}
}

// detailPageData wraps detailData with the tree roots for full-page (non-HTMX) renders.
type detailPageData struct {
	Detail detailData
	Actor  actorView
	Roots  []app.EntityResult
}

// Breadcrumb is one segment of the entity path shown above the detail heading.
// EntityID is empty for the last (current) crumb — it renders as plain text.
type Breadcrumb struct {
	Name     string
	EntityID string
}

// BreadcrumbsForEntity builds a breadcrumb slice from fullPath by matching
// each path prefix against the provided entity list. Exported for testing.
func BreadcrumbsForEntity(entities []app.EntityResult, fullPath string) []Breadcrumb {
	p, err := entitypath.Parse(fullPath)
	if err != nil {
		return nil
	}

	idByPath := make(map[string]string, len(entities))
	for _, e := range entities {
		idByPath[e.FullPathDisplay] = e.EntityID
	}

	ancestors := p.Ancestors()
	crumbs := make([]Breadcrumb, 0, len(ancestors)+1)

	for _, ancestor := range ancestors {
		crumbs = append(crumbs, Breadcrumb{
			Name:     ancestor.Base(),
			EntityID: idByPath[ancestor.String()],
		})
	}

	crumbs = append(crumbs, Breadcrumb{Name: p.Base()})

	return crumbs
}

// detailHistory returns the entity's creation timestamp and its history,
// newest first.
func (s *Server) detailHistory(ctx context.Context, entityID string) (string, []app.HistoryResult, error) {
	history, err := s.cfg.App.GetHistory(ctx, app.GetHistoryRequest{
		EntityID:    entityID,
		OldestFirst: true,
	})
	if err != nil {
		return "", nil, fmt.Errorf("get history: %w", err)
	}

	dateAdded := ""
	if len(history) > 0 {
		dateAdded = history[0].TimestampUTC
	}
	slices.Reverse(history)

	return dateAdded, history, nil
}

// buildRemovedDetailData builds the read-only view of a removed entity: its
// last location and history, with no actions. A zero Entity.EntityID means
// the ID never existed (or is not removed, which buildDetailData handles).
func (s *Server) buildRemovedDetailData(ctx context.Context, entityID string) (detailData, error) {
	entity, err := s.cfg.App.GetEntityByIDAnyStatus(ctx, entityID)
	if err != nil {
		if errors.Is(err, app.ErrNotFound) {
			return detailData{}, nil
		}
		return detailData{}, fmt.Errorf("get entity any status: %w", err)
	}
	if entity.Status != inventory.EntityStatusRemoved {
		return detailData{}, nil
	}

	dateAdded, history, err := s.detailHistory(ctx, entityID)
	if err != nil {
		return detailData{}, err
	}

	return detailData{
		Entity:    entity,
		DateAdded: dateAdded,
		History:   history,
		Removed:   true,
	}, nil
}

func (s *Server) buildDetailData(ctx context.Context, entityID string) (detailData, error) {
	entity, err := s.cfg.App.GetEntityByID(ctx, entityID)
	if err != nil {
		if errors.Is(err, app.ErrNotFound) {
			return detailData{}, nil // caller checks Entity.EntityID == ""
		}
		return detailData{}, fmt.Errorf("get entity: %w", err)
	}

	// Breadcrumbs need the full entity list to resolve ancestor IDs by path.
	// Only required when the entity is not at the root; for root entities the
	// breadcrumb template is hidden by len(.)<=1 anyway, so we skip the call.
	var crumbEntities []app.EntityResult
	if isRootEntity(entity) {
		crumbEntities = []app.EntityResult{entity}
	} else {
		crumbEntities, err = s.cfg.App.ListEntities(ctx)
		if err != nil {
			return detailData{}, fmt.Errorf("list entities: %w", err)
		}
	}

	dateAdded, history, err := s.detailHistory(ctx, entityID)
	if err != nil {
		return detailData{}, err
	}

	// Missing toggles and Loan apply only to ok/missing (ADR 0024);
	// borrowed/loaned use Return.
	editable := entity.Status == inventory.EntityStatusOk ||
		entity.Status == inventory.EntityStatusMissing

	loanBlockedReason := ""
	if editable && entity.Locked {
		loanBlockedReason = loanLockedReason
	}

	// Borrowed entities leave the inventory only via Return (ADR 0022).
	canRemove := entity.Status != inventory.EntityStatusBorrowed
	discrete := discreteAction(entity)
	removeBlockedReason := ""
	if s.hasLiveChildren(ctx, entity) {
		discrete.BlockedReason = discreteBlockedReason
		if canRemove {
			removeBlockedReason = removeHasChildrenReason
		}
	}

	return detailData{
		Entity:         entity,
		DateAdded:      dateAdded,
		History:        history,
		CanAddChild:    !entity.Discrete,
		CanMarkMissing: !entity.Locked && editable,
		IsMissing:      entity.Status == inventory.EntityStatusMissing,
		CanReturn: entity.Status == inventory.EntityStatusBorrowed ||
			entity.Status == inventory.EntityStatusLoaned,
		ReturnConfirm:       returnConfirm(entity),
		CanLoan:             !entity.Locked && editable,
		LoanBlockedReason:   loanBlockedReason,
		CanMove:             entity.CanMove(),
		MoveBlockedReason:   moveBlockedReason(entity),
		LockAction:          lockAction(entity),
		DiscreteAction:      discrete,
		CanRemove:           canRemove,
		RemoveBlockedReason: removeBlockedReason,
		Breadcrumbs:         BreadcrumbsForEntity(crumbEntities, entity.FullPathDisplay),
	}, nil
}

// returnConfirm builds the confirmation prompt for returning a borrowed entity.
// Loaned returns are low-stakes and get no prompt.
func returnConfirm(e app.EntityResult) string {
	if e.Status != inventory.EntityStatusBorrowed {
		return ""
	}
	if e.StatusContext == "" {
		return fmt.Sprintf("Return %s? It will be removed from the inventory.", e.DisplayName)
	}
	return fmt.Sprintf("Return %s to %s? It will be removed from the inventory.", e.DisplayName, e.StatusContext)
}

func (s *Server) handleEntityDetail(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entityID")

	data, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		s.serverError(w, "build detail data", err)
		return
	}
	if data.Entity.EntityID == "" {
		data, err = s.buildRemovedDetailData(r.Context(), entityID)
		if err != nil {
			s.serverError(w, "build removed detail data", err)
			return
		}
	}
	if data.Entity.EntityID == "" {
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}

	s.renderDetailSection(w, r, data)
}

func (s *Server) renderDetailSection(w http.ResponseWriter, r *http.Request, data detailData) {
	if wantsFragment(r) {
		s.renderHTML(w, "detail_section", data)
		return
	}

	entities, err := s.cfg.App.ListEntities(r.Context())
	if err != nil {
		s.serverError(w, "list entities", err)
		return
	}
	var roots []app.EntityResult
	for _, e := range entities {
		if isRootEntity(e) {
			roots = append(roots, e)
		}
	}
	s.renderHTML(w, "detail", detailPageData{Detail: data, Roots: roots, Actor: s.actorView(r)})
}

func (s *Server) handleEditNameForm(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entityID")

	data, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		s.serverError(w, "build detail data", err)
		return
	}
	if data.Entity.EntityID == "" {
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}

	s.renderHTML(w, "edit_name_form", struct {
		EntityID    string
		CurrentName string
	}{
		EntityID:    entityID,
		CurrentName: data.Entity.DisplayName,
	})
}

func (s *Server) handleEditName(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entityID")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	data, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		s.serverError(w, "build detail data", err)
		return
	}
	if data.Entity.EntityID == "" {
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}

	newName := strings.TrimSpace(r.FormValue("display_name"))
	if newName == "" {
		http.Error(w, "display_name is required", http.StatusBadRequest)
		return
	}

	_, err = s.cfg.App.RenameEntity(r.Context(), app.RenameEntityRequest{
		EntityID: entityID,
		NewName:  newName,
		ActorID:  s.actorFor(r),
	})
	if err != nil {
		s.cfg.Logger.Error("rename entity", "error", err, "entity_id", entityID)
		data.Error = "rename failed"
		s.renderDetailSection(w, r, data)
		return
	}

	// Re-fetch so the rendered section reflects the new name.
	fresh, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		s.serverError(w, "build detail data", err)
		return
	}
	if fresh.Entity.EntityID == "" {
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}
	s.renderDetailSection(w, r, fresh)
	if wantsFragment(r) {
		s.renderHTMLAppend(w, "tree_label_oob", fresh.Entity)
	}
}

// handleToggleMissing handles POST /entities/{entityID}/actions/toggle-missing.
// It toggles the entity's status between Ok and Missing.
func (s *Server) handleToggleMissing(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entityID")

	data, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		s.serverError(w, "build detail data", err)
		return
	}
	if data.Entity.EntityID == "" {
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}
	if !data.CanMarkMissing {
		http.Error(w, "status cannot be changed here", http.StatusForbidden)
		return
	}

	reqs := []app.ChangeStatusRequest{{EntityID: entityID, ActorID: s.actorFor(r)}}
	if data.IsMissing {
		_, err = s.cfg.App.MarkFound(r.Context(), reqs)
	} else {
		_, err = s.cfg.App.MarkLost(r.Context(), reqs)
	}
	if err != nil {
		s.cfg.Logger.Error("change status", "error", err, "entity_id", entityID)
		data.Error = "status change failed"
		s.renderDetailSection(w, r, data)
		return
	}

	fresh, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		s.serverError(w, "build detail data", err)
		return
	}
	s.renderDetailSection(w, r, fresh)
	if wantsFragment(r) {
		s.renderHTMLAppend(w, "tree_badge_oob", fresh.Entity)
	}
}

// handleReturn handles POST /entities/{entityID}/actions/return.
// A borrowed entity is removed, so the response navigates to its parent;
// a loaned entity goes back to ok and is re-rendered in place.
func (s *Server) handleReturn(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entityID")

	data, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		s.serverError(w, "build detail data", err)
		return
	}
	if data.Entity.EntityID == "" {
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}
	if !data.CanReturn {
		http.Error(w, "entity cannot be returned", http.StatusForbidden)
		return
	}

	_, err = s.cfg.App.MarkReturned(r.Context(), []app.ChangeStatusRequest{{
		EntityID: entityID,
		ActorID:  s.actorFor(r),
	}})
	if err != nil {
		s.renderReturnFailure(w, r, data, err)
		return
	}

	if data.Entity.Status == inventory.EntityStatusBorrowed {
		s.renderAfterRemoval(w, r, data.Entity)
		return
	}

	fresh, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		s.serverError(w, "build detail data", err)
		return
	}
	s.renderStatusChanged(w, r, fresh)
}

// renderStatusChanged renders the detail of an entity whose status changed in
// place, plus the out-of-band tree badge update.
func (s *Server) renderStatusChanged(w http.ResponseWriter, r *http.Request, fresh detailData) {
	s.renderDetailSection(w, r, fresh)
	if wantsFragment(r) {
		s.renderHTMLAppend(w, "tree_badge_oob", fresh.Entity)
	}
}

// renderReturnFailure handles a failed MarkReturned. stale was read before the
// write; a concurrent Return may have committed since (#292), leaving a
// borrowed entity removed or a loaned one back to ok. Re-read and render that
// outcome instead of a stale "return failed" with a live Return button.
func (s *Server) renderReturnFailure(w http.ResponseWriter, r *http.Request, stale detailData, err error) {
	entityID := stale.Entity.EntityID

	if errors.Is(err, app.ErrHasChildren) {
		s.cfg.Logger.Error("return entity", "error", err, "entity_id", entityID)
		stale.Error = "return failed: it still contains other entities — return or move them first"
		s.renderDetailSection(w, r, stale)
		return
	}

	fresh, ferr := s.buildDetailData(r.Context(), entityID)
	if ferr != nil {
		s.serverError(w, "build detail data", ferr)
		return
	}

	switch {
	case fresh.Entity.EntityID == "":
		s.cfg.Logger.Info("return already applied concurrently", "entity_id", entityID)
		s.renderAfterRemoval(w, r, stale.Entity)
	case !fresh.CanReturn:
		s.cfg.Logger.Info("return already applied concurrently", "entity_id", entityID)
		s.renderStatusChanged(w, r, fresh)
	default:
		s.cfg.Logger.Error("return entity", "error", err, "entity_id", entityID)
		fresh.Error = "return failed"
		s.renderDetailSection(w, r, fresh)
	}
}

// handleLoanForm handles GET /entities/{entityID}/actions/loan, swapping the
// Loan button for the inline loan form.
func (s *Server) handleLoanForm(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entityID")

	data, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		s.serverError(w, "build detail data", err)
		return
	}
	if data.Entity.EntityID == "" {
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}
	if !data.CanLoan {
		http.Error(w, "entity cannot be loaned", http.StatusForbidden)
		return
	}

	s.renderHTML(w, "loan_form", loanFormData{EntityID: entityID})
}

// handleLoan handles POST /entities/{entityID}/actions/loan.
func (s *Server) handleLoan(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entityID")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	data, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		s.serverError(w, "build detail data", err)
		return
	}
	if data.Entity.EntityID == "" {
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}
	if !data.CanLoan {
		http.Error(w, "entity cannot be loaned", http.StatusForbidden)
		return
	}

	form := loanFormData{
		EntityID:  entityID,
		Recipient: r.FormValue("recipient"),
		Note:      r.FormValue("note"),
	}
	_, err = s.cfg.App.MarkLoaned(r.Context(), []app.ChangeStatusRequest{{
		EntityID:      entityID,
		StatusContext: form.Recipient,
		Note:          form.Note,
		ActorID:       s.actorFor(r),
	}})
	if err != nil {
		s.renderLoanFailure(w, r, data, form, err)
		return
	}

	fresh, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		s.serverError(w, "build detail data", err)
		return
	}
	s.renderStatusChanged(w, r, fresh)
}

// renderLoanFailure handles a failed MarkLoaned. A blank recipient re-opens the
// form with the user's input. Anything else re-reads the entity: unlike Return,
// a concurrent loan is not this request's outcome (it may name a different
// recipient), so the fresh state is shown with an error, never as success.
func (s *Server) renderLoanFailure(
	w http.ResponseWriter, r *http.Request, stale detailData, form loanFormData, err error,
) {
	if errors.Is(err, app.ErrRecipientRequired) {
		form.Error = "Loaned to is required."
		stale.LoanForm = &form
		s.renderDetailSection(w, r, stale)
		return
	}

	entityID := stale.Entity.EntityID
	fresh, ferr := s.buildDetailData(r.Context(), entityID)
	if ferr != nil {
		s.serverError(w, "build detail data", ferr)
		return
	}
	if fresh.Entity.EntityID == "" {
		s.cfg.Logger.Info("loan target removed concurrently", "error", err, "entity_id", entityID)
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}

	if fresh.CanLoan {
		s.cfg.Logger.Error("loan entity", "error", err, "entity_id", entityID)
	} else {
		s.cfg.Logger.Info("loan rejected: entity changed concurrently", "error", err, "entity_id", entityID)
	}
	fresh.Error = "loan failed"
	s.renderStatusChanged(w, r, fresh)
}

// handleRemoveForm handles GET /entities/{entityID}/actions/remove, swapping
// the Remove button for the inline remove form.
func (s *Server) handleRemoveForm(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entityID")

	data, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		s.serverError(w, "build detail data", err)
		return
	}
	if data.Entity.EntityID == "" {
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}
	if !data.CanRemove || data.RemoveBlockedReason != "" {
		http.Error(w, "entity cannot be removed", http.StatusForbidden)
		return
	}

	s.renderHTML(w, "remove_form", removeFormData{EntityID: entityID, DisplayName: data.Entity.DisplayName})
}

// handleRemove handles POST /entities/{entityID}/actions/remove. The live
// children guard is left to the app layer (#287) so a stale panel still gets
// the inline error rather than a bare 403.
func (s *Server) handleRemove(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entityID")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	data, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		s.serverError(w, "build detail data", err)
		return
	}
	if data.Entity.EntityID == "" {
		s.renderRemoveAlreadyApplied(w, r, entityID)
		return
	}
	if !data.CanRemove {
		http.Error(w, "entity cannot be removed", http.StatusForbidden)
		return
	}

	form := removeFormData{
		EntityID:    entityID,
		DisplayName: data.Entity.DisplayName,
		Note:        strings.TrimSpace(r.FormValue("note")),
	}
	err = s.cfg.App.RemoveEntity(r.Context(), app.RemoveEntityRequest{
		EntityID: entityID,
		ActorID:  s.actorFor(r),
		Note:     form.Note,
	})
	if err != nil {
		s.renderRemoveFailure(w, r, data, form, err)
		return
	}

	s.renderAfterRemoval(w, r, data.Entity)
}

// renderRemoveFailure handles a failed RemoveEntity. Live children re-open the
// form with the user's note on a fresh read (the panel may be stale). A
// concurrent remove is this request's outcome, as with Return (#292).
func (s *Server) renderRemoveFailure(
	w http.ResponseWriter, r *http.Request, stale detailData, form removeFormData, err error,
) {
	entityID := stale.Entity.EntityID

	switch {
	case errors.Is(err, app.ErrNotFound):
		s.renderRemoveAlreadyApplied(w, r, entityID)
	case errors.Is(err, app.ErrHasChildren):
		s.cfg.Logger.Info("remove rejected: entity has live children", "error", err, "entity_id", entityID)
		fresh, ferr := s.buildDetailData(r.Context(), entityID)
		if ferr != nil {
			s.serverError(w, "build detail data", ferr)
			return
		}
		if fresh.Entity.EntityID == "" {
			s.renderRemoveAlreadyApplied(w, r, entityID)
			return
		}
		form.Error = "remove failed: it still contains other entities — remove or move them first"
		fresh.RemoveForm = &form
		s.renderDetailSection(w, r, fresh)
	default:
		s.cfg.Logger.Error("remove entity", "error", err, "entity_id", entityID)
		stale.Error = "remove failed"
		s.renderDetailSection(w, r, stale)
	}
}

// renderRemoveAlreadyApplied handles a Remove for an entity that is no longer
// live. If it was removed elsewhere (another tab, the CLI), the user's intent
// already holds, so show the removal outcome. This request's note is dropped;
// history records the removal that won.
func (s *Server) renderRemoveAlreadyApplied(w http.ResponseWriter, r *http.Request, entityID string) {
	removed, err := s.cfg.App.GetEntityByIDAnyStatus(r.Context(), entityID)
	if errors.Is(err, app.ErrNotFound) {
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}
	if err != nil {
		s.serverError(w, "get entity", err)
		return
	}
	s.cfg.Logger.Info("remove already applied concurrently", "entity_id", entityID)
	s.renderAfterRemoval(w, r, removed)
}

// handleMoveForm handles GET /entities/{entityID}/actions/move, swapping the
// Move button for the inline move form.
func (s *Server) handleMoveForm(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entityID")

	data, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		s.serverError(w, "build detail data", err)
		return
	}
	if data.Entity.EntityID == "" {
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}
	if !data.CanMove {
		http.Error(w, "entity cannot be moved", http.StatusForbidden)
		return
	}

	s.renderHTML(w, "move_form", moveFormData{EntityID: entityID, AtRoot: isRootEntity(data.Entity)})
}

// moveRootValue is the destination form value meaning "move to the root".
// Entity IDs are 10-char NanoIDs, so it cannot collide with one.
const moveRootValue = "root"

// handleMove handles POST /entities/{entityID}/actions/move.
func (s *Server) handleMove(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entityID")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	data, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		s.serverError(w, "build detail data", err)
		return
	}
	if data.Entity.EntityID == "" {
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}
	if !data.CanMove {
		http.Error(w, "entity cannot be moved", http.StatusForbidden)
		return
	}

	form := moveFormData{
		EntityID: entityID,
		AtRoot:   isRootEntity(data.Entity),
		Note:     strings.TrimSpace(r.FormValue("note")),
	}
	destination := strings.TrimSpace(r.FormValue("destination"))
	if destination == "" {
		form.Error = "Choose a destination."
		data.MoveForm = &form
		s.renderDetailSection(w, r, data)
		return
	}
	newParentID := destination
	if destination == moveRootValue {
		newParentID = ""
	}

	moved, err := s.cfg.App.ReparentEntity(r.Context(), app.ReparentEntityRequest{
		EntityID:    entityID,
		NewParentID: newParentID,
		ActorID:     s.actorFor(r),
		Note:        form.Note,
	})
	if err != nil {
		s.renderMoveFailure(w, r, data, form, err)
		return
	}

	fresh, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		s.serverError(w, "build detail data", err)
		return
	}
	s.setTreeNodeMovedTrigger(w, r, data.Entity.ParentID, moved)
	s.renderDetailSection(w, r, fresh)
	s.renderHTMLAppend(w, "tree_node_delete_oob", moved)
}

// renderMoveFailure handles a failed ReparentEntity. It re-reads the entity
// first: if it is still movable, a bad or vanished destination re-opens the
// form; anything else shows the fresh state with an error, never as success.
func (s *Server) renderMoveFailure(
	w http.ResponseWriter, r *http.Request, stale detailData, form moveFormData, err error,
) {
	entityID := stale.Entity.EntityID
	fresh, ferr := s.buildDetailData(r.Context(), entityID)
	if ferr != nil {
		s.serverError(w, "build detail data", ferr)
		return
	}
	if fresh.Entity.EntityID == "" {
		s.cfg.Logger.Info("move target removed concurrently", "error", err, "entity_id", entityID)
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}

	if fresh.CanMove && (errors.Is(err, app.ErrInvalidDestination) || errors.Is(err, app.ErrNotFound)) {
		form.Error = "Can't move there — choose another destination."
		fresh.MoveForm = &form
		s.renderDetailSection(w, r, fresh)
		return
	}

	if fresh.CanMove {
		s.cfg.Logger.Error("move entity", "error", err, "entity_id", entityID)
	} else {
		s.cfg.Logger.Info("move rejected: entity changed concurrently", "error", err, "entity_id", entityID)
	}
	fresh.Error = "move failed"
	s.renderStatusChanged(w, r, fresh)
}

// treeNodeDetachedEvent is the tree-node-detached payload: entityID left
// parentID's child list. tree.js resets the parent to a leaf when
// ParentHasChildren is false.
type treeNodeDetachedEvent struct {
	EntityID          string `json:"entityId"`
	ParentID          string `json:"parentId"`
	ParentHasChildren bool   `json:"parentHasChildren"`
}

// detachedEvent builds the tree-node-detached payload for entityID leaving
// parentID. ok is false when the parent's child count could not be read; the
// caller then sends no trigger, because the write already committed and a
// stale chevron is better than a false error.
func (s *Server) detachedEvent(ctx context.Context, entityID, parentID string) (treeNodeDetachedEvent, bool) {
	ev := treeNodeDetachedEvent{EntityID: entityID, ParentID: parentID}
	if parentID != "" {
		has, err := s.cfg.App.HasLiveChildren(ctx, parentID)
		if err != nil {
			s.cfg.Logger.Warn("count parent children", "error", err, "parent_id", parentID)
			return treeNodeDetachedEvent{}, false
		}
		ev.ParentHasChildren = has
	}
	return ev, true
}

// treeNodeMovedTriggers is the Hx-Trigger-After-Swap body for a move: detach
// from the old parent, then attach under the new one. Field order is the
// order htmx fires the events.
type treeNodeMovedTriggers struct {
	Detached *treeNodeDetachedEvent `json:"tree-node-detached,omitempty"`
	Attached treeNodeAttachedEvent  `json:"tree-node-attached"`
}

// setTreeNodeMovedTrigger sets the detach + attach triggers for a moved entity.
// If the old parent's child count is unreadable, only the attach is sent.
func (s *Server) setTreeNodeMovedTrigger(
	w http.ResponseWriter, r *http.Request, oldParentID string, moved app.EntityResult,
) {
	triggers := treeNodeMovedTriggers{
		Attached: treeNodeAttachedEvent{EntityID: moved.EntityID, ParentID: moved.ParentID},
	}
	if ev, ok := s.detachedEvent(r.Context(), moved.EntityID, oldParentID); ok {
		triggers.Detached = &ev
	}
	trigger, err := json.Marshal(triggers)
	if err != nil {
		s.cfg.Logger.Warn("marshal tree-node-moved", "error", err)
		return
	}
	w.Header().Set("Hx-Trigger-After-Swap", string(trigger))
}

// setTreeNodeDetachedTrigger sets Hx-Trigger-After-Swap so tree.js runs after
// the OOB delete.
func (s *Server) setTreeNodeDetachedTrigger(w http.ResponseWriter, r *http.Request, removed app.EntityResult) {
	ev, ok := s.detachedEvent(r.Context(), removed.EntityID, removed.ParentID)
	if !ok {
		return
	}
	trigger, err := json.Marshal(map[string]treeNodeDetachedEvent{"tree-node-detached": ev})
	if err != nil {
		s.cfg.Logger.Warn("marshal tree-node-detached", "error", err)
		return
	}
	w.Header().Set("Hx-Trigger-After-Swap", string(trigger))
}

// renderAfterRemoval shows the removed entity's parent (or the empty state for a
// root entity), pushes the matching URL, and deletes the entity's tree node.
func (s *Server) renderAfterRemoval(w http.ResponseWriter, r *http.Request, removed app.EntityResult) {
	var parent detailData
	if removed.ParentID != "" {
		var err error
		parent, err = s.buildDetailData(r.Context(), removed.ParentID)
		if err != nil {
			s.serverError(w, "build parent detail data", err)
			return
		}
	}

	s.setTreeNodeDetachedTrigger(w, r, removed)
	if parent.Entity.EntityID == "" {
		w.Header().Set("Hx-Push-Url", "/")
		s.renderHTML(w, "detail_empty", nil)
	} else {
		w.Header().Set("Hx-Push-Url", "/entities/"+parent.Entity.EntityID)
		s.renderHTML(w, "detail_section", parent)
	}
	s.renderHTMLAppend(w, "tree_node_delete_oob", removed)
}
