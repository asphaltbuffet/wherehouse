package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/asphaltbuffet/wherehouse/internal/app"
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
