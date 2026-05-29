package web

import (
	"bytes"
	"errors"
	"net/http"
	"strings"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/store"
	"github.com/asphaltbuffet/wherehouse/internal/version"
)

type indexData struct {
	Roots     []app.EntityResult
	Version   string
	GitCommit string
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

	data := indexData{
		Roots:     roots,
		Version:   version.Version,
		GitCommit: version.GitCommit,
	}
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

// handleSearch handles GET /search?q=... returning either a full page or
// an HTMX fragment depending on the Hx-Request header.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if len(q) > maxQueryLen {
		http.Error(w, "query too long", http.StatusBadRequest)
		return
	}
	q = strings.TrimSpace(q)

	htmx := r.Header.Get("Hx-Request") == htmxHeaderVal

	var searchData any
	if q != "" {
		results, err := s.cfg.App.FindEntities(r.Context(), app.FindEntitiesRequest{
			Query: q,
			Limit: searchLimit,
		})
		if err != nil {
			s.serverError(w, "search", err)
			return
		}
		searchData = struct {
			Query   string
			Results []app.FindResult
		}{Query: q, Results: results}
	}

	if htmx {
		s.renderHTML(w, "search_results", searchData)
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
	s.renderHTML(w, "search_page", struct {
		Roots  []app.EntityResult
		Search any
	}{Roots: roots, Search: searchData})
}
