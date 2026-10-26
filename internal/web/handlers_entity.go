package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// detailData is the view model for the entity detail pane.
type detailData struct {
	Entity         app.EntityResult
	DateAdded      string
	History        []app.HistoryResult
	CanAddChild    bool
	CanMarkMissing bool
	IsMissing      bool   // true when Status == EntityStatusMissing
	CanReturn      bool   // borrowed or loaned
	ReturnConfirm  string // non-empty only for borrowed: returning removes the entity
	Breadcrumbs    []Breadcrumb
	Error          string // populated by edit POST handlers; rendered inline above the detail dl
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

	history, err := s.cfg.App.GetHistory(ctx, app.GetHistoryRequest{
		EntityID:    entityID,
		OldestFirst: true,
	})
	if err != nil {
		return detailData{}, fmt.Errorf("get history: %w", err)
	}

	dateAdded := ""
	if len(history) > 0 {
		dateAdded = history[0].TimestampUTC
	}
	for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
		history[i], history[j] = history[j], history[i]
	}

	// Missing toggles apply only to ok/missing; borrowed/loaned use Return.
	editable := entity.Status == inventory.EntityStatusOk ||
		entity.Status == inventory.EntityStatusMissing

	return detailData{
		Entity:         entity,
		DateAdded:      dateAdded,
		History:        history,
		CanAddChild:    !entity.Discrete,
		CanMarkMissing: !entity.Locked && editable,
		IsMissing:      entity.Status == inventory.EntityStatusMissing,
		CanReturn: entity.Status == inventory.EntityStatusBorrowed ||
			entity.Status == inventory.EntityStatusLoaned,
		ReturnConfirm: returnConfirm(entity),
		Breadcrumbs:   BreadcrumbsForEntity(crumbEntities, entity.FullPathDisplay),
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
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}

	s.renderDetailSection(w, r, data)
}

func (s *Server) renderDetailSection(w http.ResponseWriter, r *http.Request, data detailData) {
	if r.Header.Get("Hx-Request") == htmxHeaderVal {
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
	if r.Header.Get("Hx-Request") == htmxHeaderVal {
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

	target := inventory.EntityStatusMissing
	if data.IsMissing {
		target = inventory.EntityStatusOk
	}

	_, err = s.cfg.App.ChangeStatus(r.Context(), app.ChangeStatusRequest{
		EntityID: entityID,
		Status:   target,
		ActorID:  s.actorFor(r),
	})
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
	if r.Header.Get("Hx-Request") == htmxHeaderVal {
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
	s.renderReturned(w, r, fresh)
}

// renderReturned renders the detail of an entity that is no longer
// borrowed/loaned, plus the out-of-band tree badge update.
func (s *Server) renderReturned(w http.ResponseWriter, r *http.Request, fresh detailData) {
	s.renderDetailSection(w, r, fresh)
	if r.Header.Get("Hx-Request") == htmxHeaderVal {
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
		s.renderReturned(w, r, fresh)
	default:
		s.cfg.Logger.Error("return entity", "error", err, "entity_id", entityID)
		fresh.Error = "return failed"
		s.renderDetailSection(w, r, fresh)
	}
}

// treeNodeDetachedEvent is the tree-node-detached payload: entityID left
// parentID's child list. tree.js resets the parent to a leaf when
// ParentHasChildren is false.
type treeNodeDetachedEvent struct {
	EntityID          string `json:"entityId"`
	ParentID          string `json:"parentId"`
	ParentHasChildren bool   `json:"parentHasChildren"`
}

// setTreeNodeDetachedTrigger sets Hx-Trigger-After-Swap so tree.js runs after
// the OOB delete. On failure it logs and sends no trigger: the removal has
// already committed, and a stale chevron is better than a false error.
func (s *Server) setTreeNodeDetachedTrigger(w http.ResponseWriter, r *http.Request, removed app.EntityResult) {
	ev := treeNodeDetachedEvent{EntityID: removed.EntityID, ParentID: removed.ParentID}
	if removed.ParentID != "" {
		has, err := s.cfg.App.HasLiveChildren(r.Context(), removed.ParentID)
		if err != nil {
			s.cfg.Logger.Warn("count parent children", "error", err, "parent_id", removed.ParentID)
			return
		}
		ev.ParentHasChildren = has
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
