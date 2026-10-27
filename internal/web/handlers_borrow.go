package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
)

// borrowFormData is the view model for the borrow modal. Field values are
// echoed back when the form is re-rendered with an error.
type borrowFormData struct {
	ParentID    string // empty means root
	ParentPath  string // FullPathDisplay of ParentID; empty for root
	Error       string
	DisplayName string
	From        string
	Note        string
}

// handleBorrowForm serves the borrow modal, pre-selecting a parent derived from
// the entity the user is currently viewing.
func (s *Server) handleBorrowForm(w http.ResponseWriter, r *http.Request) {
	parent, err := s.defaultBorrowParent(r.Context(), currentEntityID(r))
	if err != nil {
		s.serverError(w, "resolve default parent", err)
		return
	}
	s.renderHTML(w, "borrow_form", borrowFormData{
		ParentID:   parent.EntityID,
		ParentPath: parent.FullPathDisplay,
	})
}

// currentEntityID returns the entity ID when the HX-Current-URL header points at
// /entities/{id}; any other page (or a missing/malformed header) yields "".
func currentEntityID(r *http.Request) string {
	u, err := url.Parse(r.Header.Get("Hx-Current-Url"))
	if err != nil {
		return ""
	}
	rest, ok := strings.CutPrefix(u.Path, "/entities/")
	if !ok || rest == "" || strings.Contains(rest, "/") {
		return ""
	}
	return rest
}

// defaultBorrowParent walks from entityID up the hierarchy to the nearest eligible
// parent. A zero EntityResult means root.
func (s *Server) defaultBorrowParent(ctx context.Context, entityID string) (app.EntityResult, error) {
	for entityID != "" {
		e, err := s.cfg.App.GetEntityByIDAnyStatus(ctx, entityID)
		if errors.Is(err, app.ErrNotFound) {
			return app.EntityResult{}, nil
		}
		if err != nil {
			return app.EntityResult{}, fmt.Errorf("get entity %q: %w", entityID, err)
		}
		if e.IsEligibleParent() {
			return e, nil
		}
		entityID = e.ParentID
	}
	return app.EntityResult{}, nil
}

var errNoBorrowResult = errors.New("borrow returned no result")

// treeNodeAttachedEvent is the trigger detail tree.js uses to insert a node
// that joined ParentID's list ("" for the root): sent as entity-created after
// a borrow and tree-node-attached after a move.
type treeNodeAttachedEvent struct {
	EntityID string `json:"entityId"`
	ParentID string `json:"parentId"`
}

// handleBorrow creates one borrowed entity and shows its detail. Validation and
// app errors re-render the form inside the modal instead.
func (s *Server) handleBorrow(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	form := borrowFormData{
		ParentID:    strings.TrimSpace(r.FormValue("parent_id")),
		DisplayName: strings.TrimSpace(r.FormValue("display_name")),
		From:        strings.TrimSpace(r.FormValue("from")),
		Note:        strings.TrimSpace(r.FormValue("note")),
	}

	if form.ParentID != "" {
		parent, err := s.cfg.App.GetEntityByID(r.Context(), form.ParentID)
		if errors.Is(err, app.ErrNotFound) {
			form.ParentID = ""
			s.renderBorrowFormError(w, form, "That location no longer exists — choose another.")
			return
		}
		if err != nil {
			s.serverError(w, "get parent entity", err)
			return
		}
		form.ParentPath = parent.FullPathDisplay
	}

	results, err := s.cfg.App.BorrowEntities(r.Context(), []app.BorrowEntityRequest{{
		DisplayName:   form.DisplayName,
		ParentPath:    form.ParentPath,
		StatusContext: form.From,
		ActorID:       s.actorFor(r),
		Note:          form.Note,
	}})
	if err != nil {
		s.cfg.Logger.Error("borrow entity", "error", err)
		s.renderBorrowFormError(w, form, borrowErrorMessage(err))
		return
	}
	if len(results) == 0 {
		s.serverError(w, "borrow entity", errNoBorrowResult)
		return
	}
	created := results[0]

	data, err := s.buildDetailData(r.Context(), created.EntityID)
	if err != nil {
		s.serverError(w, "build detail data", err)
		return
	}
	trigger, err := json.Marshal(map[string]treeNodeAttachedEvent{
		"entity-created": {EntityID: created.EntityID, ParentID: created.ParentID},
	})
	if err != nil {
		s.serverError(w, "marshal trigger", err)
		return
	}

	w.Header().Set("Hx-Push-Url", "/entities/"+created.EntityID)
	w.Header().Set("Hx-Trigger", string(trigger))
	s.renderHTML(w, "detail_section", data)
}

// renderBorrowFormError re-renders the borrow form in the modal with msg shown.
func (s *Server) renderBorrowFormError(w http.ResponseWriter, form borrowFormData, msg string) {
	form.Error = msg
	w.Header().Set("Hx-Retarget", "#add-modal-body")
	w.Header().Set("Hx-Reswap", "innerHTML")
	s.renderHTML(w, "borrow_form", form)
}

// borrowErrorMessage maps app-layer validation errors to form messages.
func borrowErrorMessage(err error) string {
	switch {
	case errors.Is(err, app.ErrLenderRequired):
		return "Borrowed from is required."
	case errors.Is(err, entitypath.ErrEmptySegment):
		return "Name is required."
	case errors.Is(err, entitypath.ErrSegmentContainsSeparator):
		return `Name cannot contain ":".`
	case errors.Is(err, entitypath.ErrInvalidSegment):
		return "Name contains invalid characters."
	default:
		return "borrow failed"
	}
}
