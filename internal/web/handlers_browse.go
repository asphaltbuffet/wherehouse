package web

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

type indexData struct {
	Shell pageShell
	Home  homeData
}

// homeData is the view model for the home detail at "/": the roots as a children list.
type homeData struct {
	ChildList childList
}

func (s *Server) homeView(ctx context.Context) (homeData, error) {
	roots, err := s.listRoots(ctx)
	if err != nil {
		return homeData{}, err
	}
	return homeData{ChildList: childList{Children: roots}}, nil
}

// handleIndex renders the home detail: as a fragment for htmx, else inside the full page shell.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	home, err := s.homeView(r.Context())
	if err != nil {
		s.serverError(w, "build home", err)
		return
	}
	if wantsFragment(w, r) {
		s.renderHTML(w, "home_section", home)
		return
	}
	shell, err := s.newPageShell(r.Context(), r, "Wherehouse")
	if err != nil {
		s.serverError(w, "build page shell", err)
		return
	}
	s.renderHTML(w, "index", indexData{Shell: shell, Home: home})
}

// handleTreeChildren returns the direct children of an entity as tree_node fragments.
func (s *Server) handleTreeChildren(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entityID")

	if _, err := s.cfg.App.GetEntityByID(r.Context(), entityID); err != nil {
		if errors.Is(err, app.ErrNotFound) {
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
// an HTMX fragment depending on wantsFragment.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if len(q) > maxQueryLen {
		http.Error(w, "query too long", http.StatusBadRequest)
		return
	}
	q = strings.TrimSpace(q)

	htmx := wantsFragment(w, r)

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

	shell, err := s.newPageShell(r.Context(), r, "Wherehouse — Search")
	if err != nil {
		s.serverError(w, "build page shell", err)
		return
	}
	s.renderHTML(w, "search_page", struct {
		Shell  pageShell
		Search any
	}{Shell: shell, Search: searchData})
}

// handleParentOptions returns eligible-parent matches for a parent picker.
// With ?moving=<id> (the move or use form; ?verb=use selects use) it returns
// only valid destinations for that entity.
func (s *Server) handleParentOptions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if len(q) > maxQueryLen {
		http.Error(w, "query too long", http.StatusBadRequest)
		return
	}
	q = strings.TrimSpace(q)

	var results []app.FindResult
	if q != "" {
		req := app.FindEntitiesRequest{Query: q, Limit: parentOptionsLimit}
		var err error
		if moving := r.URL.Query().Get("moving"); moving != "" {
			results, err = s.cfg.App.FindMoveDestinations(r.Context(), moving, relocationParam(r), req)
		} else {
			results, err = s.cfg.App.FindParentCandidates(r.Context(), req)
		}
		if errors.Is(err, app.ErrNotFound) {
			http.Error(w, "entity not found", http.StatusNotFound)
			return
		}
		if err != nil {
			s.serverError(w, "find parent candidates", err)
			return
		}
	}

	s.renderHTML(w, "parent_options", struct {
		Query   string
		Results []app.FindResult
	}{Query: q, Results: results})
}

// handleTreeNode returns a single tree_node fragment for inserting a new or moved entity.
func (s *Server) handleTreeNode(w http.ResponseWriter, r *http.Request) {
	e, err := s.cfg.App.GetEntityByID(r.Context(), r.PathValue("entityID"))
	if errors.Is(err, app.ErrNotFound) {
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}
	if err != nil {
		s.serverError(w, "get entity", err)
		return
	}
	// GetEntityByID leaves HasChildren false; a moved entity may have children.
	e.HasChildren, err = s.cfg.App.HasLiveChildren(r.Context(), e.EntityID)
	if err != nil {
		s.serverError(w, "count children", err)
		return
	}
	s.renderHTML(w, "tree_node", e)
}
