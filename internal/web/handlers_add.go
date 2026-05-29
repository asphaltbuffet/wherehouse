package web

import (
	"errors"
	"net/http"
	"strings"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
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

// handleAddItem processes the POST form, creates the entity, and returns a fresh tree_node fragment.
func (s *Server) handleAddItem(w http.ResponseWriter, r *http.Request) {
	parentID := r.PathValue("parentID")

	parent, err := s.cfg.App.GetEntityByID(r.Context(), parentID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "parent entity not found", http.StatusNotFound)
			return
		}
		s.serverError(w, "get parent entity", err)
		return
	}

	s.createAndRenderNode(w, r, parent.FullPathDisplay)
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
		http.Error(w, "invalid entity_type", http.StatusBadRequest)
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
		s.serverError(w, "create entity", err)
		return
	}

	s.renderHTML(w, "tree_node", created)
}
