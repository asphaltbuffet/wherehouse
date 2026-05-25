package web

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

const (
	historyLimit  = 50
	searchLimit   = 50
	htmxHeaderVal = "true"
)

// handleHealthz responds with 200 OK for liveness checks.
func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// handleIndex renders the full page shell with root-level tree nodes.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	entities, err := s.cfg.App.ListEntities(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("list entities: %v", err), http.StatusInternalServerError)
		return
	}

	var roots []app.EntityResult
	for _, e := range entities {
		if isRootEntity(e) {
			roots = append(roots, e)
		}
	}

	data := struct{ Roots []app.EntityResult }{Roots: roots}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if tmplErr := s.templates.ExecuteTemplate(w, "index", data); tmplErr != nil {
		// headers already sent; log and return
		s.cfg.Logger.Error("execute index template", "error", tmplErr)
	}
}

// handleTreeChildren returns the direct children of an entity as tree_node fragments.
func (s *Server) handleTreeChildren(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entityID")
	children, err := s.cfg.App.GetChildren(r.Context(), entityID)
	if err != nil {
		http.Error(w, fmt.Sprintf("get children: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	for _, child := range children {
		if tmplErr := s.templates.ExecuteTemplate(w, "tree_node", child); tmplErr != nil {
			// headers already sent; log and return
			s.cfg.Logger.Error("execute tree_node template", "error", tmplErr)
			return
		}
	}
}

// handleEntityDetail returns the detail pane fragment for a single entity.
type detailData struct {
	Entity         app.EntityResult
	DateAdded      string
	History        []app.HistoryResult
	StatusEditable bool
	Breadcrumbs    []Breadcrumb
	Error          string // populated by edit POST handlers; rendered inline above the detail dl
}

// detailPageData wraps detailData with the tree roots for full-page (non-HTMX) renders.
type detailPageData struct {
	Detail detailData
	Roots  []app.EntityResult
}

// Breadcrumb is one segment of the entity path shown above the detail heading.
// EntityID is empty for the last (current) crumb — it renders as plain text.
type Breadcrumb struct {
	Name     string
	EntityID string
}

// BreadcrumbsForEntity builds a breadcrumb slice from fullPath by matching each
// path prefix against the provided entity list. Exported for testing.
func BreadcrumbsForEntity(entities []app.EntityResult, fullPath string) []Breadcrumb {
	parts := strings.Split(fullPath, ":")
	crumbs := make([]Breadcrumb, len(parts))
	for i, part := range parts {
		prefix := strings.Join(parts[:i+1], ":")
		id := ""
		if i < len(parts)-1 {
			for _, e := range entities {
				if e.FullPathDisplay == prefix {
					id = e.EntityID
					break
				}
			}
		}
		crumbs[i] = Breadcrumb{Name: part, EntityID: id}
	}
	return crumbs
}

func (s *Server) buildDetailData(ctx context.Context, entityID string) (detailData, error) {
	entities, err := s.cfg.App.ListEntities(ctx)
	if err != nil {
		return detailData{}, fmt.Errorf("list entities: %w", err)
	}
	var entity *app.EntityResult
	for i := range entities {
		if entities[i].EntityID == entityID {
			entity = &entities[i]
			break
		}
	}
	if entity == nil {
		return detailData{}, nil // caller checks Entity.EntityID == ""
	}

	history, err := s.cfg.App.GetHistory(ctx, app.GetHistoryRequest{
		EntityID:    entityID,
		Limit:       historyLimit,
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

	// borrowed and loaned transitions require dedicated flows with relationship data.
	editable := entity.Status == inventory.EntityStatusOk ||
		entity.Status == inventory.EntityStatusMissing

	return detailData{
		Entity:         *entity,
		DateAdded:      dateAdded,
		History:        history,
		StatusEditable: editable,
		Breadcrumbs:    BreadcrumbsForEntity(entities, entity.FullPathDisplay),
	}, nil
}

func (s *Server) handleEntityDetail(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entityID")

	data, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if data.Entity.EntityID == "" {
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}

	s.renderDetailSection(w, r, data)
}

func (s *Server) renderDetailSection(w http.ResponseWriter, r *http.Request, data detailData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Header.Get("Hx-Request") == htmxHeaderVal {
		if tmplErr := s.templates.ExecuteTemplate(w, "detail_section", data); tmplErr != nil {
			s.cfg.Logger.Error("execute detail template", "error", tmplErr)
		}
		return
	}

	entities, err := s.cfg.App.ListEntities(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("list entities: %v", err), http.StatusInternalServerError)
		return
	}
	var roots []app.EntityResult
	for _, e := range entities {
		if isRootEntity(e) {
			roots = append(roots, e)
		}
	}
	pageData := detailPageData{Detail: data, Roots: roots}
	if tmplErr := s.templates.ExecuteTemplate(w, "detail", pageData); tmplErr != nil {
		s.cfg.Logger.Error("execute detail template", "error", tmplErr)
	}
}

func (s *Server) handleEditNameForm(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entityID")

	data, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if data.Entity.EntityID == "" {
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if tmplErr := s.templates.ExecuteTemplate(w, "edit_name_form", struct {
		EntityID    string
		CurrentName string
	}{
		EntityID:    entityID,
		CurrentName: data.Entity.DisplayName,
	}); tmplErr != nil {
		s.cfg.Logger.Error("execute edit_name_form template", "error", tmplErr)
	}
}

func (s *Server) handleEditStatusForm(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entityID")

	data, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if data.Entity.EntityID == "" {
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}
	if !data.StatusEditable {
		http.Error(w, "status cannot be edited here", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if tmplErr := s.templates.ExecuteTemplate(w, "edit_status_form", struct {
		EntityID             string
		CurrentStatus        string
		CurrentStatusContext string
	}{
		EntityID:             entityID,
		CurrentStatus:        data.Entity.Status.String(),
		CurrentStatusContext: data.Entity.StatusContext,
	}); tmplErr != nil {
		s.cfg.Logger.Error("execute edit_status_form template", "error", tmplErr)
	}
}

func (s *Server) handleEditName(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entityID")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	data, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		EntityPath: data.Entity.FullPathDisplay,
		NewName:    newName,
		ActorID:    "webui",
	})
	if err != nil {
		data.Error = fmt.Sprintf("rename failed: %v", err)
		s.renderDetailSection(w, r, data)
		return
	}

	// Re-fetch so the rendered section reflects the new name.
	fresh, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if fresh.Entity.EntityID == "" {
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}
	s.renderDetailSection(w, r, fresh)
	if r.Header.Get("Hx-Request") == htmxHeaderVal {
		_ = s.templates.ExecuteTemplate(w, "tree_label_oob", fresh.Entity)
	}
}

func (s *Server) handleEditStatus(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entityID")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	data, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if data.Entity.EntityID == "" {
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}
	if !data.StatusEditable {
		http.Error(w, "status cannot be edited here", http.StatusForbidden)
		return
	}

	status, err := inventory.ParseEntityStatus(r.FormValue("status"))
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid status: %v", err), http.StatusBadRequest)
		return
	}

	err = s.cfg.App.ChangeStatus(r.Context(), app.ChangeStatusRequest{
		EntityPath:    data.Entity.FullPathDisplay,
		Status:        status,
		StatusContext: strings.TrimSpace(r.FormValue("status_context")),
		ActorID:       "webui",
	})
	if err != nil {
		data.Error = fmt.Sprintf("status change failed: %v", err)
		s.renderDetailSection(w, r, data)
		return
	}

	fresh, err := s.buildDetailData(r.Context(), entityID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if fresh.Entity.EntityID == "" {
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}
	s.renderDetailSection(w, r, fresh)
	if r.Header.Get("Hx-Request") == htmxHeaderVal {
		_ = s.templates.ExecuteTemplate(w, "tree_badge_oob", fresh.Entity)
	}
}

// isRootEntity returns true when e has no parent (no colon in canonical name = depth 0).
func isRootEntity(e app.EntityResult) bool {
	for _, c := range e.FullPathDisplay {
		if c == ':' {
			return false
		}
	}
	return true
}

// handleAddItemForm returns an inline modal form for adding a child entity under parentID.
func (s *Server) handleAddItemForm(w http.ResponseWriter, r *http.Request) {
	s.renderAddForm(w, r, r.PathValue("parentID"))
}

// handleRootAddItemForm serves the add-item form for creating a root-level entity.
func (s *Server) handleRootAddItemForm(w http.ResponseWriter, r *http.Request) {
	s.renderAddForm(w, r, "")
}

type addFormData struct {
	ParentID string // empty means root
	Target   string // CSS selector for hx-target
}

func (s *Server) renderAddForm(w http.ResponseWriter, _ *http.Request, parentID string) {
	target := "#tree-children-" + parentID
	if parentID == "" {
		target = "#tree-root-list"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(
		w,
		"add_item_form",
		addFormData{ParentID: parentID, Target: target},
	); err != nil {
		s.cfg.Logger.Error("execute add_item_form template", "error", err)
	}
}

// handleAddItem processes the POST form, creates the entity, and returns a fresh tree_node fragment.
func (s *Server) handleAddItem(w http.ResponseWriter, r *http.Request) {
	parentID := r.PathValue("parentID")

	entities, err := s.cfg.App.ListEntities(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("list entities: %v", err), http.StatusInternalServerError)
		return
	}
	var parentPath string
	for _, e := range entities {
		if e.EntityID == parentID {
			parentPath = e.FullPathDisplay
			break
		}
	}
	if parentPath == "" {
		http.Error(w, "parent entity not found", http.StatusNotFound)
		return
	}

	s.createAndRenderNode(w, r, parentPath)
}

// handleRootAddItem processes the POST form for a new root-level entity.
func (s *Server) handleRootAddItem(w http.ResponseWriter, r *http.Request) {
	s.createAndRenderNode(w, r, "")
}

func (s *Server) createAndRenderNode(w http.ResponseWriter, r *http.Request, parentPath string) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	displayName := strings.TrimSpace(r.FormValue("display_name"))
	if displayName == "" {
		http.Error(w, "display_name is required", http.StatusBadRequest)
		return
	}

	entityType, err := inventory.ParseEntityType(r.FormValue("entity_type"))
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid entity_type: %v", err), http.StatusBadRequest)
		return
	}

	actor := strings.TrimSpace(r.FormValue("user"))
	if actor == "" {
		actor = "webui"
	}

	created, err := s.cfg.App.CreateEntity(r.Context(), app.CreateEntityRequest{
		DisplayName: displayName,
		EntityType:  entityType,
		ParentPath:  parentPath,
		ActorID:     actor,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("create entity: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if tmplErr := s.templates.ExecuteTemplate(w, "tree_node", created); tmplErr != nil {
		s.cfg.Logger.Error("execute tree_node template", "error", tmplErr)
	}
}

// handleSearch handles GET /search?q=... returning either a full page or
// an HTMX fragment depending on the Hx-Request header.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	tmplName := "search_page"
	if r.Header.Get("Hx-Request") == htmxHeaderVal {
		tmplName = "search_results"
	}

	if q == "" {
		// Headers already written; log template errors rather than writing a 500.
		if tmplErr := s.templates.ExecuteTemplate(w, tmplName, nil); tmplErr != nil {
			s.cfg.Logger.Error("execute search template", "error", tmplErr)
		}
		return
	}

	results, err := s.cfg.App.FindEntities(r.Context(), app.FindEntitiesRequest{
		Query: q,
		Limit: searchLimit,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("search: %v", err), http.StatusInternalServerError)
		return
	}

	data := struct {
		Query   string
		Results []app.FindResult
	}{Query: q, Results: results}

	// Headers already written; log template errors rather than writing a 500.
	if tmplErr := s.templates.ExecuteTemplate(w, tmplName, data); tmplErr != nil {
		s.cfg.Logger.Error("execute search template", "error", tmplErr)
	}
}
