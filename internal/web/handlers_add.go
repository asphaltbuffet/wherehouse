package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

type addFormData struct {
	ParentID string // empty means root
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
	s.renderHTML(w, "add_item_form", addFormData{ParentID: parentID})
}

// handleAddItem processes the POST form, creates the entity under the picked
// parent, and returns the parent's detail plus an entity-created trigger that
// tree.js uses to insert the tree node.
func (s *Server) handleAddItem(w http.ResponseWriter, r *http.Request) {
	s.createAndRenderParent(w, r, r.PathValue("parentID"))
}

// handleRootAddItem processes the POST form for a new root-level entity.
func (s *Server) handleRootAddItem(w http.ResponseWriter, r *http.Request) {
	s.createAndRenderParent(w, r, "")
}

func (s *Server) createAndRenderParent(w http.ResponseWriter, r *http.Request, parentID string) {
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

	trigger, err := json.Marshal(map[string]treeNodeAttachedEvent{
		"entity-created": {EntityID: created.EntityID, ParentID: parentID},
	})
	if err != nil {
		s.serverError(w, "marshal trigger", err)
		return
	}
	w.Header().Set("Hx-Trigger", string(trigger))
	s.renderParentDetail(w, r, parentID)
}

// renderParentDetail shows parentID's detail with its fresh children list, or
// the home detail for the root (or a parent that vanished after the write).
func (s *Server) renderParentDetail(w http.ResponseWriter, r *http.Request, parentID string) {
	if parentID != "" {
		data, err := s.buildDetailData(r.Context(), parentID)
		if err != nil {
			s.serverError(w, "build parent detail data", err)
			return
		}
		if data.Entity.EntityID != "" {
			w.Header().Set("Hx-Push-Url", "/entities/"+parentID)
			s.renderHTML(w, "detail_section", data)
			return
		}
	}
	home, err := s.homeView(r.Context())
	if err != nil {
		s.serverError(w, "build home", err)
		return
	}
	w.Header().Set("Hx-Push-Url", "/")
	s.renderHTML(w, "home_section", home)
}
