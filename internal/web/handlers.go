package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

const historyLimit = 50

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
func (s *Server) handleEntityDetail(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entityID")

	entities, err := s.cfg.App.ListEntities(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("list entities: %v", err), http.StatusInternalServerError)
		return
	}
	// ListEntities excludes removed entities; removed entity IDs will 404.
	var entity *app.EntityResult
	for i := range entities {
		if entities[i].EntityID == entityID {
			entity = &entities[i]
			break
		}
	}
	if entity == nil {
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}

	history, err := s.cfg.App.GetHistory(r.Context(), app.GetHistoryRequest{
		EntityID:    entityID,
		Limit:       historyLimit,
		OldestFirst: true,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("get history: %v", err), http.StatusInternalServerError)
		return
	}

	dateAdded := ""
	if len(history) > 0 {
		dateAdded = history[0].TimestampUTC
	}
	// Flip to newest-first for display.
	for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
		history[i], history[j] = history[j], history[i]
	}

	data := struct {
		Entity    app.EntityResult
		DateAdded string
		History   []app.HistoryResult
	}{
		Entity:    *entity,
		DateAdded: dateAdded,
		History:   history,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if tmplErr := s.templates.ExecuteTemplate(w, "detail", data); tmplErr != nil {
		// headers already sent; log and return
		s.cfg.Logger.Error("execute detail template", "error", tmplErr)
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
