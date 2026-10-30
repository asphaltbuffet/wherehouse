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
	CanMarkLost       bool          // ok and not locked (ADR 0024)
	CanMarkFound      bool          // missing; found has no locked guard (ADR 0024)
	CanReturn         bool          // borrowed, loaned, or displaced ok
	ReturnConfirm     string        // non-empty only for borrowed: returning removes the entity
	CanLoan           bool          // ok/missing and not locked (ADR 0024)
	LoanBlockedReason string        // non-empty when Loan is shown disabled
	LoanForm          *loanFormData // non-nil re-opens the loan form (validation error)
	CanMove           bool          // not locked; ok or borrowed (ADR 0034)
	MoveBlockedReason string        // non-empty when Move is shown disabled
	UseBlockedReason  string        // non-empty when Use is shown disabled
	MoveForm          *moveFormData // non-nil re-opens the move form (validation error)
	CanUse            bool          // same as CanMove (use = move preconditions)
	UseForm           *moveFormData // non-nil re-opens the use form (validation error)
	Displaced         bool          // parent differs from home (CONTEXT.md "Displaced")
	LockAction        attrAction    // Lock / Unlock button
	DiscreteAction    attrAction    // Mark discrete / Clear discrete button
	// CanRemove is false for borrowed: a borrowed entity exits via Return (ADR 0022).
	CanRemove           bool
	RemoveBlockedReason string            // non-empty when Remove is shown disabled
	RemoveForm          *removeFormData   // non-nil re-opens the remove form (guard tripped at submit)
	EditNameForm        *editNameFormData // non-nil swaps the name tape for the rename form
	Breadcrumbs         []Breadcrumb
	AncestorIDs         string    // space-separated, root first; tree.js expands along them (#363)
	Error               string    // populated by edit POST handlers; rendered inline above the detail dl
	Removed             bool      // read-only view of a removed entity: last location + history, no actions
	ChildList           childList // live children; empty for discrete entities (never fetched)
}

// childList is the view model for the children list. ParentID is "" on the
// home detail, where Children are the roots.
type childList struct {
	ParentID string
	Children []app.EntityResult
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
	Verb     string // "move" or "use": form action and labels
	AtRoot   bool   // hides the "move to root" choice: see hidesRootChoice
	Note     string
	Error    string
}

// Tooltips on a disabled Move or Use button; each names the one action that
// unblocks it. participle is "moved" or "used".
func relocateBlockedReason(e app.EntityResult, participle string) string {
	switch {
	case e.Locked:
		return "Locked entities cannot be " + participle
	case e.Status == inventory.EntityStatusMissing:
		return "Missing entities cannot be " + participle + " — mark found first"
	case e.Status == inventory.EntityStatusLoaned:
		return "Loaned entities cannot be " + participle + " — return it first"
	}
	return ""
}

func moveBlockedReason(e app.EntityResult) string { return relocateBlockedReason(e, "moved") }

func useBlockedReason(e app.EntityResult) string { return relocateBlockedReason(e, "used") }

// removeFormData is the view model for the inline remove form.
type removeFormData struct {
	EntityID    string
	DisplayName string
	Note        string
	Error       string
}

// removeHasChildrenReason is the tooltip on a disabled Remove button (#287).
const removeHasChildrenReason = "Contains other entities — remove or move them first"

// editNameFormData is the view model for the inline rename form.
type editNameFormData struct {
	EntityID    string
	CurrentName string
}

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
	lockDisplacedReason   = "Displaced entities cannot be locked — return it first"
	useAtHomeError        = "That's where it belongs — use Return instead."
	attrFailedMessage     = "attribute change failed"
	discreteIsHomeMessage = "Can't mark discrete: it is home to entities that are away — return them first"
)

func lockAction(e app.EntityResult) attrAction {
	if e.Locked {
		return attrAction{EntityID: e.EntityID, Label: "Unlock", Field: "locked", Value: "false"}
	}
	a := attrAction{EntityID: e.EntityID, Label: "Lock", Field: "locked", Value: "true"}
	if e.Status != inventory.EntityStatusOk && e.Status != inventory.EntityStatusBorrowed {
		a.BlockedReason = lockBlockedReason
	}
	if e.IsDisplaced() {
		a.BlockedReason = lockDisplacedReason
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
		s.renderGone(w, r, entityID)
		return
	}

	switch {
	case setErr == nil:
	case errors.Is(setErr, app.ErrNoChange):
		s.cfg.Logger.Info("attribute unchanged: panel was stale", "error", setErr, "entity_id", entityID)
		fresh.Error = staleMessage
	case errors.Is(setErr, app.ErrDisplaced):
		s.cfg.Logger.Info("attribute rejected: entity is displaced", "error", setErr, "entity_id", entityID)
		fresh.Error = lockDisplacedReason
	case errors.Is(setErr, app.ErrIsHome):
		s.cfg.Logger.Info("attribute rejected: entity is home to displaced entities",
			"error", setErr, "entity_id", entityID)
		fresh.Error = discreteIsHomeMessage
	default:
		s.cfg.Logger.Error("set attributes", "error", setErr, "entity_id", entityID)
		fresh.Error = attrFailedMessage
	}

	s.renderDetailSection(w, r, fresh)
	if wantsFragment(w, r) {
		s.renderHTMLAppend(w, "tree_attrs_oob", fresh.Entity)
	}
}

// detailPageData wraps detailData with the tree roots for full-page (non-HTMX) renders.
type detailPageData struct {
	Shell  pageShell
	Detail detailData
}

// ancestorIDs is the space-separated IDs of the crumbs' entities, root first.
func ancestorIDs(crumbs []Breadcrumb) string {
	ids := make([]string, 0, len(crumbs))
	for _, c := range crumbs {
		if c.EntityID != "" {
			ids = append(ids, c.EntityID)
		}
	}

	return strings.Join(ids, " ")
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
	// A root entity's only crumb (after Inventory) is itself, so skip the call.
	var crumbEntities []app.EntityResult
	if isRootEntity(entity) {
		crumbEntities = []app.EntityResult{entity}
	} else {
		crumbEntities, err = s.cfg.App.ListEntities(ctx)
		if err != nil {
			return detailData{}, fmt.Errorf("list entities: %w", err)
		}
	}
	crumbs := BreadcrumbsForEntity(crumbEntities, entity.FullPathDisplay)

	dateAdded, history, err := s.detailHistory(ctx, entityID)
	if err != nil {
		return detailData{}, err
	}

	var children []app.EntityResult
	if !entity.Discrete {
		children, err = s.cfg.App.GetChildren(ctx, entityID)
		if err != nil {
			return detailData{}, fmt.Errorf("get children: %w", err)
		}
	}

	// Loan applies only to ok/missing (ADR 0024);
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
		Entity:       entity,
		DateAdded:    dateAdded,
		History:      history,
		CanAddChild:  !entity.Discrete,
		CanMarkLost:  entity.Status == inventory.EntityStatusOk && !entity.Locked,
		CanMarkFound: entity.Status == inventory.EntityStatusMissing,
		CanReturn: entity.Status == inventory.EntityStatusBorrowed ||
			entity.Status == inventory.EntityStatusLoaned ||
			(entity.Status == inventory.EntityStatusOk && entity.IsDisplaced()),
		ReturnConfirm:       returnConfirm(entity),
		CanLoan:             !entity.Locked && editable,
		LoanBlockedReason:   loanBlockedReason,
		CanMove:             entity.CanMove(),
		CanUse:              entity.CanMove(),
		Displaced:           entity.IsDisplaced(),
		MoveBlockedReason:   moveBlockedReason(entity),
		UseBlockedReason:    useBlockedReason(entity),
		LockAction:          lockAction(entity),
		DiscreteAction:      discrete,
		CanRemove:           canRemove,
		RemoveBlockedReason: removeBlockedReason,
		Breadcrumbs:         crumbs,
		AncestorIDs:         ancestorIDs(crumbs),
		ChildList:           childList{ParentID: entity.EntityID, Children: children},
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
	if wantsFragment(w, r) {
		s.renderHTML(w, "detail_section", data)
		return
	}

	shell, err := s.newPageShell(r.Context(), r, "Wherehouse")
	if err != nil {
		s.serverError(w, "build page shell", err)
		return
	}
	s.renderHTML(w, "detail", detailPageData{Shell: shell, Detail: data})
}

// handleEditNameForm handles GET /entities/{entityID}/edit/name, re-rendering
// the detail section with the rename form in place of the name tape.
func (s *Server) handleEditNameForm(w http.ResponseWriter, r *http.Request) {
	s.renderFormOpen(w, r, func(d *detailData) (string, bool) {
		d.EditNameForm = &editNameFormData{EntityID: d.Entity.EntityID, CurrentName: d.Entity.DisplayName}
		return "", true
	})
}

// handleEditName handles POST /entities/{entityID}/edit/name. Liveness is left
// to the app layer (ADR 0035); the result is always rendered from a fresh read.
func (s *Server) handleEditName(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entityID")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	newName := strings.TrimSpace(r.FormValue("display_name"))
	if newName == "" {
		http.Error(w, "display_name is required", http.StatusBadRequest)
		return
	}

	_, renameErr := s.cfg.App.RenameEntity(r.Context(), app.RenameEntityRequest{
		EntityID: entityID,
		NewName:  newName,
		ActorID:  s.actorFor(r),
	})

	fresh, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		s.serverError(w, "build detail data", err)
		return
	}
	if fresh.Entity.EntityID == "" {
		s.renderGone(w, r, entityID)
		return
	}
	if renameErr != nil {
		s.cfg.Logger.Error("rename entity", "error", renameErr, "entity_id", entityID)
		fresh.Error = "rename failed"
		s.renderDetailSection(w, r, fresh)
		return
	}

	s.renderDetailSection(w, r, fresh)
	if wantsFragment(w, r) {
		s.renderHTMLAppend(w, "tree_label_oob", fresh.Entity)
		s.renderHTMLAppend(w, "tree_attrs_oob", fresh.Entity) // the add button's label names it
	}
}

// missingChange is app.MarkLost or app.MarkFound.
type missingChange func(context.Context, []app.ChangeStatusRequest) ([]app.EntityResult, error)

// handleLost handles POST /entities/{entityID}/actions/lost.
func (s *Server) handleLost(w http.ResponseWriter, r *http.Request) {
	s.changeMissing(w, r, s.cfg.App.MarkLost, func(d detailData) bool { return d.CanMarkLost })
}

// handleFound handles POST /entities/{entityID}/actions/found.
func (s *Server) handleFound(w http.ResponseWriter, r *http.Request) {
	s.changeMissing(w, r, s.cfg.App.MarkFound, func(d detailData) bool { return d.CanMarkFound })
}

// changeMissing applies an explicit lost/found change. The button names its
// target, so a stale click fails the app layer's source-status guard (ADR
// 0024, 0035) instead of flipping the status the other way (#306). offered
// reports whether the fresh panel still offers this change: if not, the
// failure was a stale panel, not an error.
func (s *Server) changeMissing(
	w http.ResponseWriter, r *http.Request, change missingChange, offered func(detailData) bool,
) {
	entityID := r.PathValue("entityID")
	_, changeErr := change(r.Context(), []app.ChangeStatusRequest{{EntityID: entityID, ActorID: s.actorFor(r)}})

	fresh, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		s.serverError(w, "build detail data", err)
		return
	}
	if fresh.Entity.EntityID == "" {
		s.renderGone(w, r, entityID)
		return
	}

	switch {
	case changeErr == nil:
	case offered(fresh):
		s.cfg.Logger.Error("change status", "error", changeErr, "entity_id", entityID)
		fresh.Error = "status change failed"
	default:
		// No pre-write read, so a stale panel and a replayed request look alike.
		s.cfg.Logger.Info("status change rejected: not offered", "error", changeErr, "entity_id", entityID)
		fresh.Error = unavailableMessage
	}
	s.renderStatusChanged(w, r, fresh)
}

// handleReturn handles POST /entities/{entityID}/actions/return.
// A borrowed entity is removed, so the response navigates to its parent;
// a loaned entity goes back to ok and is re-rendered in place. A displaced
// entity goes back to its home, so its tree node moves like a Move's. Eligibility is
// left to the app layer (ADR 0035), so a stale panel reaches
// renderReturnFailure instead of a bare 403 (#306).
func (s *Server) handleReturn(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entityID")

	data, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		s.serverError(w, "build detail data", err)
		return
	}
	if data.Entity.EntityID == "" {
		s.renderAlreadyRemoved(w, r, entityID)
		return
	}

	wasDisplaced := data.Entity.IsDisplaced()

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
	if wasDisplaced {
		s.setTreeNodeMovedTrigger(w, r, data.Entity.ParentID, fresh.Entity)
		s.renderDetailSection(w, r, fresh)
		s.renderHTMLAppend(w, "tree_node_delete_oob", fresh.Entity)
		return
	}
	s.renderStatusChanged(w, r, fresh)
}

// renderStatusChanged renders the detail of an entity whose status changed in
// place, plus the out-of-band tree badge update.
func (s *Server) renderStatusChanged(w http.ResponseWriter, r *http.Request, fresh detailData) {
	s.renderDetailSection(w, r, fresh)
	if wantsFragment(w, r) {
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

	if errors.Is(err, app.ErrIsHome) {
		s.cfg.Logger.Error("return entity", "error", err, "entity_id", entityID)
		stale.Error = "return failed: it is home to entities that are away — return them first"
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

// formOpener sets a form on d and reports whether the entity allows the
// action. When it doesn't, blocked is the action's blocked reason, if any.
type formOpener func(d *detailData) (blocked string, opened bool)

// renderFormOpen answers a form-opening GET with the fresh detail section, so
// a stale panel is refreshed rather than left to fail silently (#306). If the
// action isn't offered, the current state is shown with unavailableError and
// the tree badge, since the status may have changed elsewhere (#335).
func (s *Server) renderFormOpen(w http.ResponseWriter, r *http.Request, open formOpener) {
	entityID := r.PathValue("entityID")

	data, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		s.serverError(w, "build detail data", err)
		return
	}
	if data.Entity.EntityID == "" {
		s.renderGone(w, r, entityID)
		return
	}
	blocked, opened := open(&data)
	if opened {
		s.renderDetailSection(w, r, data)
		return
	}
	s.cfg.Logger.Info("form not opened: action not offered", "entity_id", entityID)
	data.Error = unavailableError(blocked)
	s.renderStatusChanged(w, r, data)
}

// handleLoanForm handles GET /entities/{entityID}/actions/loan, re-rendering
// the detail section with the inline loan form open.
func (s *Server) handleLoanForm(w http.ResponseWriter, r *http.Request) {
	s.renderFormOpen(w, r, func(d *detailData) (string, bool) {
		if !d.CanLoan {
			return d.LoanBlockedReason, false
		}
		d.LoanForm = &loanFormData{EntityID: d.Entity.EntityID}
		return "", true
	})
}

// handleLoan handles POST /entities/{entityID}/actions/loan. Eligibility is
// left to the app layer (ADR 0035), so a stale panel reaches renderLoanFailure.
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
		s.renderGone(w, r, entityID)
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
		// The recipient check runs first, so re-check here: never re-open the
		// form on a panel whose entity can't be loaned (#334).
		if !stale.CanLoan {
			s.cfg.Logger.Info("loan rejected: not offered", "error", err, "entity_id", stale.Entity.EntityID)
			stale.Error = unavailableError(stale.LoanBlockedReason)
			s.renderStatusChanged(w, r, stale)
			return
		}
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
		s.renderGone(w, r, entityID)
		return
	}

	if fresh.CanLoan {
		s.cfg.Logger.Error("loan entity", "error", err, "entity_id", entityID)
		fresh.Error = "loan failed"
	} else {
		s.cfg.Logger.Info("loan rejected: not offered", "error", err, "entity_id", entityID)
		fresh.Error = rejectedError(stale.CanLoan, fresh.LoanBlockedReason)
	}
	s.renderStatusChanged(w, r, fresh)
}

// handleRemoveForm handles GET /entities/{entityID}/actions/remove,
// re-rendering the detail section with the inline remove form open.
func (s *Server) handleRemoveForm(w http.ResponseWriter, r *http.Request) {
	s.renderFormOpen(w, r, func(d *detailData) (string, bool) {
		if !d.removeOffered() {
			return d.RemoveBlockedReason, false
		}
		d.RemoveForm = &removeFormData{EntityID: d.Entity.EntityID, DisplayName: d.Entity.DisplayName}
		return "", true
	})
}

// removeOffered reports whether the panel shows an enabled Remove button.
func (d *detailData) removeOffered() bool { return d.CanRemove && d.RemoveBlockedReason == "" }

// handleRemove handles POST /entities/{entityID}/actions/remove. Eligibility,
// including the live children guard (#287), is left to the app layer (ADR
// 0035), so a stale panel gets an inline error rather than a bare 403.
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
		s.renderAlreadyRemoved(w, r, entityID)
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

// renderRemoveFailure handles a failed RemoveEntity on a fresh read (the panel
// may be stale). Live children re-open the form with the user's note; a
// concurrent remove is this request's outcome, as with Return (#292).
func (s *Server) renderRemoveFailure(
	w http.ResponseWriter, r *http.Request, stale detailData, form removeFormData, err error,
) {
	entityID := stale.Entity.EntityID

	switch {
	case errors.Is(err, app.ErrNotFound):
		s.renderAlreadyRemoved(w, r, entityID)
	case errors.Is(err, app.ErrHasChildren):
		s.renderRemoveFormError(w, r, form, err, removeHasChildrenMessage)
	case errors.Is(err, app.ErrIsHome):
		s.renderRemoveFormError(w, r, form, err, removeIsHomeMessage)
	default:
		fresh, ferr := s.buildDetailData(r.Context(), entityID)
		if ferr != nil {
			s.serverError(w, "build detail data", ferr)
			return
		}
		if fresh.Entity.EntityID == "" {
			s.renderAlreadyRemoved(w, r, entityID)
			return
		}
		if fresh.CanRemove {
			s.cfg.Logger.Error("remove entity", "error", err, "entity_id", entityID)
			fresh.Error = "remove failed"
		} else {
			s.cfg.Logger.Info("remove rejected: not offered", "error", err, "entity_id", entityID)
			fresh.Error = rejectedError(stale.removeOffered(), fresh.RemoveBlockedReason)
		}
		s.renderStatusChanged(w, r, fresh)
	}
}

// Remove-form errors for guards the app layer trips at submit time.
const (
	removeHasChildrenMessage = "remove failed: it still contains other entities — remove or move them first"
	removeIsHomeMessage      = "remove failed: it is home to entities that are away — return them first"
)

// renderRemoveFormError re-reads the entity and re-opens the remove form with
// message and the user's note, for a guard rejection.
func (s *Server) renderRemoveFormError(
	w http.ResponseWriter, r *http.Request, form removeFormData, err error, message string,
) {
	entityID := form.EntityID
	s.cfg.Logger.Info("remove rejected by guard", "error", err, "entity_id", entityID)
	fresh, ferr := s.buildDetailData(r.Context(), entityID)
	if ferr != nil {
		s.serverError(w, "build detail data", ferr)
		return
	}
	if fresh.Entity.EntityID == "" {
		s.renderAlreadyRemoved(w, r, entityID)
		return
	}
	form.Error = message
	fresh.RemoveForm = &form
	s.renderDetailSection(w, r, fresh)
}

// renderAlreadyRemoved handles a Remove or Return for an entity that is no
// longer live. If it left the inventory elsewhere (another tab, the CLI), the
// user's intent already holds, so show the removal outcome. This request's
// note is dropped; history records the event that won.
func (s *Server) renderAlreadyRemoved(w http.ResponseWriter, r *http.Request, entityID string) {
	removed, err := s.cfg.App.GetEntityByIDAnyStatus(r.Context(), entityID)
	if errors.Is(err, app.ErrNotFound) {
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}
	if err != nil {
		s.serverError(w, "get entity", err)
		return
	}
	s.cfg.Logger.Info("entity already removed concurrently", "entity_id", entityID)
	s.renderAfterRemoval(w, r, removed)
}

// handleMoveForm handles GET /entities/{entityID}/actions/move, re-rendering
// the detail section with the inline move form open.
func (s *Server) handleMoveForm(w http.ResponseWriter, r *http.Request) {
	s.renderFormOpen(w, r, func(d *detailData) (string, bool) {
		if !d.CanMove {
			return d.MoveBlockedReason, false
		}
		d.MoveForm = &moveFormData{
			EntityID: d.Entity.EntityID, AtRoot: hidesRootChoice(d.Entity, verbMove), Verb: verbMove,
		}
		return "", true
	})
}

// moveRootValue is the destination form value meaning "move to the root".
// Entity IDs are 10-char NanoIDs, so it cannot collide with one.
const moveRootValue = "root"

// handleUseForm handles GET /entities/{entityID}/actions/use: the move form in
// use mode (CONTEXT.md "Use": same preconditions as move).
func (s *Server) handleUseForm(w http.ResponseWriter, r *http.Request) {
	s.renderFormOpen(w, r, func(d *detailData) (string, bool) {
		if !d.CanUse {
			return d.UseBlockedReason, false
		}
		d.UseForm = &moveFormData{
			EntityID: d.Entity.EntityID, AtRoot: hidesRootChoice(d.Entity, verbUse), Verb: verbUse,
		}
		return "", true
	})
}

// relocateBlocked is the blocked reason of the Move or Use button, per verb.
func (d *detailData) relocateBlocked(verb string) string {
	if verb == verbUse {
		return d.UseBlockedReason
	}
	return d.MoveBlockedReason
}

// Verbs of the relocate form: a move changes home, a use keeps it.
const (
	verbMove = "move"
	verbUse  = "use"
)

// relocateFn performs the write behind a relocate form (move or use).
type relocateFn func(ctx context.Context, entityID, newParentID, actorID, note string) (app.EntityResult, error)

// openRelocateForm re-opens the form that matches f.Verb.
func (d *detailData) openRelocateForm(f *moveFormData) {
	if f.Verb == verbUse {
		d.UseForm = f
		return
	}
	d.MoveForm = f
}

// handleMove handles POST /entities/{entityID}/actions/move. Eligibility is
// left to the app layer (ADR 0035), so a stale panel reaches renderMoveFailure.
func (s *Server) handleMove(w http.ResponseWriter, r *http.Request) {
	s.handleRelocate(w, r, verbMove, func(
		ctx context.Context, entityID, newParentID, actor, note string,
	) (app.EntityResult, error) {
		return s.cfg.App.ReparentEntity(ctx, app.ReparentEntityRequest{
			EntityID: entityID, NewParentID: newParentID, ActorID: actor, Note: note,
		})
	})
}

// handleUse handles POST /entities/{entityID}/actions/use. Like handleMove,
// eligibility is left to the app layer (ADR 0035).
func (s *Server) handleUse(w http.ResponseWriter, r *http.Request) {
	s.handleRelocate(w, r, verbUse, func(
		ctx context.Context, entityID, newParentID, actor, note string,
	) (app.EntityResult, error) {
		return s.cfg.App.UseEntity(ctx, app.UseEntityRequest{
			EntityID: entityID, NewParentID: newParentID, ActorID: actor, Note: note,
		})
	})
}

// handleRelocate is the shared body of the move and use POST handlers.
func (s *Server) handleRelocate(w http.ResponseWriter, r *http.Request, verb string, do relocateFn) {
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
		s.renderGone(w, r, entityID)
		return
	}

	form := moveFormData{
		EntityID: entityID,
		AtRoot:   hidesRootChoice(data.Entity, verb),
		Verb:     verb,
		Note:     strings.TrimSpace(r.FormValue("note")),
	}
	destination := strings.TrimSpace(r.FormValue("destination"))
	if destination == "" {
		// No write to fail, so re-check here: never re-open the form on a
		// stale panel whose entity can no longer move.
		if !data.CanMove {
			s.cfg.Logger.Info("relocate rejected: not offered", "verb", verb, "entity_id", entityID)
			data.Error = unavailableError(data.relocateBlocked(verb))
			s.renderStatusChanged(w, r, data)
			return
		}
		form.Error = "Choose a destination."
		data.openRelocateForm(&form)
		s.renderDetailSection(w, r, data)
		return
	}
	newParentID := destination
	if destination == moveRootValue {
		newParentID = ""
	}

	moved, err := do(r.Context(), entityID, newParentID, s.actorFor(r), form.Note)
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

// renderMoveFailure handles a failed ReparentEntity or UseEntity. It re-reads the entity
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
		s.renderGone(w, r, entityID)
		return
	}

	if fresh.CanMove && errors.Is(err, app.ErrDestinationIsHome) {
		form.Error = useAtHomeError
		fresh.openRelocateForm(&form)
		s.renderDetailSection(w, r, fresh)
		return
	}

	if fresh.CanMove && errors.Is(err, app.ErrHomeCycle) {
		form.Error = "Can't " + form.Verb + " there — something that belongs inside it is away. Return it first."
		fresh.openRelocateForm(&form)
		s.renderDetailSection(w, r, fresh)
		return
	}

	if fresh.CanMove && (errors.Is(err, app.ErrInvalidDestination) || errors.Is(err, app.ErrNotFound)) {
		form.Error = "Can't " + form.Verb + " there — choose another destination."
		fresh.openRelocateForm(&form)
		s.renderDetailSection(w, r, fresh)
		return
	}

	if fresh.CanMove {
		s.cfg.Logger.Error(form.Verb+" entity", "error", err, "entity_id", entityID)
		fresh.Error = form.Verb + " failed"
	} else {
		s.cfg.Logger.Info(form.Verb+" rejected: not offered", "error", err, "entity_id", entityID)
		fresh.Error = rejectedError(stale.CanMove, fresh.relocateBlocked(form.Verb))
	}
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

// renderAfterRemoval shows the removed entity's parent (or the home detail for a
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
		home, err := s.homeView(r.Context())
		if err != nil {
			s.serverError(w, "build home", err)
			return
		}
		w.Header().Set("Hx-Push-Url", "/")
		s.renderHTML(w, "home_section", home)
	} else {
		w.Header().Set("Hx-Push-Url", "/entities/"+parent.Entity.EntityID)
		s.renderHTML(w, "detail_section", parent)
	}
	s.renderHTMLAppend(w, "tree_node_delete_oob", removed)
}

// renderGone answers an action on an entity that is no longer live. A removed
// entity gets its read-only panel with staleMessage, and its stale tree node is
// dropped. An ID that never existed is a plain 404: no real panel links to it.
func (s *Server) renderGone(w http.ResponseWriter, r *http.Request, entityID string) {
	removed, err := s.buildRemovedDetailData(r.Context(), entityID)
	if err != nil {
		s.serverError(w, "build removed detail data", err)
		return
	}
	if removed.Entity.EntityID == "" {
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}

	s.cfg.Logger.Info("action on removed entity: panel was stale", "entity_id", entityID)
	removed.Error = staleMessage
	s.setTreeNodeDetachedTrigger(w, r, removed.Entity)
	s.renderDetailSection(w, r, removed)
	if wantsFragment(w, r) {
		s.renderHTMLAppend(w, "tree_node_delete_oob", removed.Entity)
	}
}
