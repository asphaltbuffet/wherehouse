package web

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

const (
	searchLimit   = 50
	maxQueryLen   = 100
	maxFormBytes  = 16 * 1024 // upper bound for POST form bodies
	htmxHeaderVal = "true"
)

// serverError logs the internal detail and returns a generic 500 to the client.
// Use this for any failure that originates inside the server (DB, template, app
// layer) — never expose the wrapped error text in the response.
func (s *Server) serverError(w http.ResponseWriter, op string, err error) {
	s.cfg.Logger.Error(op, "error", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}

// renderHTML renders one template into a buffer and writes it to w with the
// HTML content-type. On template error the response is a clean 500 — the
// partial output never reaches the client.
func (s *Server) renderHTML(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, name, data); err != nil {
		s.serverError(w, "execute "+name, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write(buf.Bytes()); err != nil {
		s.cfg.Logger.Error("write response", "error", err, "template", name)
	}
}

// renderHTMLAppend renders into a buffer and appends to w without setting
// Content-Type or status. Used for HTMX out-of-band swaps that follow a
// primary render. Template errors are logged but the primary response is
// not affected.
func (s *Server) renderHTMLAppend(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, name, data); err != nil {
		s.cfg.Logger.Error("execute "+name, "error", err)
		return
	}
	if _, err := w.Write(buf.Bytes()); err != nil {
		s.cfg.Logger.Error("write OOB", "error", err, "template", name)
	}
}

// handleHealthz responds with 200 OK for liveness checks.
func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// handleIndex renders the full page shell with root-level tree nodes.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
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

	data := struct{ Roots []app.EntityResult }{Roots: roots}
	s.renderHTML(w, "index", data)
}

// handleTreeChildren returns the direct children of an entity as tree_node fragments.
func (s *Server) handleTreeChildren(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entityID")

	if _, err := s.cfg.App.GetEntityByID(r.Context(), entityID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "entity not found", http.StatusNotFound)
			return
		}
		s.serverError(w, "get entity", err)
		return
	}

	children, err := s.cfg.App.GetChildren(r.Context(), entityID)
	if err != nil {
		s.serverError(w, "get children", err)
		return
	}

	var buf bytes.Buffer
	for _, child := range children {
		if tmplErr := s.templates.ExecuteTemplate(&buf, "tree_node", child); tmplErr != nil {
			s.serverError(w, "execute tree_node", tmplErr)
			return
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, werr := w.Write(buf.Bytes()); werr != nil {
		s.cfg.Logger.Error("write tree children", "error", werr)
	}
}

// handleEntityDetail returns the detail pane fragment for a single entity.
type detailData struct {
	Entity         app.EntityResult
	DateAdded      string
	History        []app.HistoryResult
	CanAddChild    bool // false when EntityType == EntityTypeLeaf
	CanMarkMissing bool // false when EntityType == EntityTypePlace
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
		CanAddChild:    entity.EntityType != inventory.EntityTypeLeaf,
		CanMarkMissing: entity.EntityType != inventory.EntityTypePlace && editable,
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

// handleSearch handles GET /search?q=... returning either a full page or
// an HTMX fragment depending on the Hx-Request header.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) > maxQueryLen {
		http.Error(w, "query too long", http.StatusBadRequest)
		return
	}

	tmplName := "search_page"
	if r.Header.Get("Hx-Request") == htmxHeaderVal {
		tmplName = "search_results"
	}

	if q == "" {
		s.renderHTML(w, tmplName, nil)
		return
	}

	results, err := s.cfg.App.FindEntities(r.Context(), app.FindEntitiesRequest{
		Query: q,
		Limit: searchLimit,
	})
	if err != nil {
		s.serverError(w, "search", err)
		return
	}

	s.renderHTML(w, tmplName, struct {
		Query   string
		Results []app.FindResult
	}{Query: q, Results: results})
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

	err = s.cfg.App.ChangeStatus(r.Context(), app.ChangeStatusRequest{
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
