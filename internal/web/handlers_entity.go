package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// detailData is the view model for the entity detail pane.
type detailData struct {
	Entity         app.EntityResult
	DateAdded      string
	History        []app.HistoryResult
	CanAddChild    bool
	CanMarkMissing bool
	IsMissing      bool // true when Status == EntityStatusMissing
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
		if errors.Is(err, store.ErrNotFound) {
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

	// borrowed and loaned transitions require dedicated flows with relationship data.
	editable := entity.Status == inventory.EntityStatusOk ||
		entity.Status == inventory.EntityStatusMissing

	return detailData{
		Entity:         entity,
		DateAdded:      dateAdded,
		History:        history,
		CanAddChild:    !entity.Discrete,
		CanMarkMissing: !entity.Locked && editable,
		IsMissing:      entity.Status == inventory.EntityStatusMissing,
		Breadcrumbs:    BreadcrumbsForEntity(crumbEntities, entity.FullPathDisplay),
	}, nil
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
	s.renderHTML(w, "detail", detailPageData{Detail: data, Roots: roots})
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
		EntityPath: data.Entity.FullPathDisplay,
		NewName:    newName,
		ActorID:    "webui",
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
		EntityPath: data.Entity.FullPathDisplay,
		Status:     target,
		ActorID:    "webui",
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
