package web

import (
	"errors"
	"net/http"
	"strings"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

type addFormData struct {
	ParentID string // empty means root
	Target   string // CSS selector for hx-target
}

// handleAddItemForm returns an inline modal form for adding a child entity under parentID.
func (s *Server) handleAddItemForm(w http.ResponseWriter, r *http.Request) {
	s.renderAddForm(w, r, r.PathValue("parentID"))
}

// handleRootAddItemForm serves the add-item form for creating a root-level entity.
func (s *Server) handleRootAddItemForm(w http.ResponseWriter, r *http.Request) {
	s.renderAddForm(w, r, "")
}

func (s *Server) renderAddForm(w http.ResponseWriter, _ *http.Request, parentID string) {
	target := "#tree-children-" + parentID
	if parentID == "" {
		target = "#tree-root-list"
	}
	s.renderHTML(w, "add_item_form", addFormData{ParentID: parentID, Target: target})
}

// handleAddItem processes the POST form, creates the entity under the picked
// parent, and returns a fresh tree_node fragment.
func (s *Server) handleAddItem(w http.ResponseWriter, r *http.Request) {
	s.createAndRenderNode(w, r, r.PathValue("parentID"))
}

// handleRootAddItem processes the POST form for a new root-level entity.
func (s *Server) handleRootAddItem(w http.ResponseWriter, r *http.Request) {
	s.createAndRenderNode(w, r, "")
}

func (s *Server) createAndRenderNode(w http.ResponseWriter, r *http.Request, parentID string) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	displayName := strings.TrimSpace(r.FormValue("display_name"))
	if displayName == "" {
		http.Error(w, "display_name is required", http.StatusBadRequest)
		return
	}

	created, err := s.cfg.App.CreateEntity(r.Context(), app.CreateEntityRequest{
		DisplayName: displayName,
		Locked:      r.FormValue("locked") != "",
		Discrete:    r.FormValue("discrete") != "",
		ParentID:    parentID,
		ActorID:     s.actorFor(r),
	})
	if err != nil {
		switch {
		case errors.Is(err, app.ErrNotFound):
			http.Error(w, "parent entity not found", http.StatusNotFound)
		case errors.Is(err, app.ErrInvalidDestination):
			http.Error(w, "that location can't hold items", http.StatusUnprocessableEntity)
		default:
			s.serverError(w, "create entity", err)
		}
		return
	}

	s.renderHTML(w, "tree_node", created)
}
