package web

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"slices"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
	versioncmd "github.com/asphaltbuffet/wherehouse/internal/version"
)

// pageShell is what every full page renders around its main panel.
type pageShell struct {
	Title     string
	Roots     []app.EntityResult
	Actor     actorView
	Version   string
	GitCommit string
}

func (s *Server) newPageShell(ctx context.Context, r *http.Request, title string) (pageShell, error) {
	roots, err := s.listRoots(ctx)
	if err != nil {
		return pageShell{}, err
	}
	return pageShell{
		Title:     title,
		Roots:     roots,
		Actor:     s.actorView(r),
		Version:   versioncmd.Version,
		GitCommit: versioncmd.GitCommit,
	}, nil
}

// listRoots returns the root entities in ListEntities order (the tree's order).
func (s *Server) listRoots(ctx context.Context) ([]app.EntityResult, error) {
	entities, err := s.cfg.App.ListEntities(ctx)
	if err != nil {
		return nil, fmt.Errorf("list entities: %w", err)
	}
	var roots []app.EntityResult
	for _, e := range entities {
		if isRootEntity(e) {
			roots = append(roots, e)
		}
	}
	return roots, nil
}

const (
	searchLimit          = 50
	parentOptionsLimit   = 10
	maxQueryLen          = 100
	maxFormBytes         = 16 * 1024 // upper bound for POST form bodies
	htmxHeaderVal        = "true"
	htmxRequestHeader    = "Hx-Request"
	historyRestoreHeader = "Hx-History-Restore-Request"
)

// staleMessage is the inline error shown when an action's panel was stale:
// the entity changed elsewhere, so its fresh state is rendered instead. Use it
// only when the code knows the entity changed (#336); otherwise see
// unavailableError.
const staleMessage = "Already changed elsewhere; showing current state."

// unavailableMessage is the neutral inline error for an action the fresh read
// doesn't offer, when there is no blocked reason to explain why (#336).
const unavailableMessage = "That action isn't available; showing current state."

// unavailableError is the inline error for an action the fresh read doesn't
// offer: blocked (the action's disabled-button tooltip) when set, else the
// neutral unavailableMessage. It reads the same for a stale panel and a
// never-eligible URL, since the code can't tell them apart.
func unavailableError(blocked string) string {
	if blocked != "" {
		return blocked
	}
	return unavailableMessage
}

// rejectedError is the inline error for a write the fresh read no longer
// offers. If the pre-write read offered it, the entity changed during the
// request, so staleMessage is known to be true.
func rejectedError(offeredBefore bool, blocked string) string {
	if offeredBefore {
		return staleMessage
	}
	return unavailableError(blocked)
}

// serverError logs the internal detail and returns a generic 500 to the client.
// Use this for any failure that originates inside the server (DB, template, app
// layer) — never expose the wrapped error text in the response.
func (s *Server) serverError(w http.ResponseWriter, op string, err error) {
	s.cfg.Logger.Error(op, "error", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}

// wantsFragment reports whether r should be answered with an HTMX fragment
// rather than a full page. htmx sends its history-cache-miss reload with
// Hx-Request set, but swaps the response into <body>, so that request needs
// the full page. Security checks (csrfGuard) must read Hx-Request directly.
//
// Because the answer shapes the response, it also adds both headers to Vary on
// w, so caches never serve a fragment for a full navigation or the reverse.
// Call it before the response is written.
func wantsFragment(w http.ResponseWriter, r *http.Request) bool {
	h := w.Header()
	for _, name := range []string{htmxRequestHeader, historyRestoreHeader} {
		if !slices.Contains(h.Values("Vary"), name) {
			h.Add("Vary", name)
		}
	}
	return r.Header.Get(htmxRequestHeader) == htmxHeaderVal &&
		r.Header.Get(historyRestoreHeader) != htmxHeaderVal
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

// isRootEntity returns true when e has no parent (no colon in path = depth 0).
func isRootEntity(e app.EntityResult) bool {
	p, err := entitypath.Parse(e.FullPathDisplay)
	if err != nil {
		return false
	}

	return p.Depth() == 1
}

// hidesRootChoice reports whether the move form omits "Move to root". Moving a
// displaced entity to where it already is re-homes it, so for it the choice is
// hidden only when the home is the root (going home is a return); otherwise it
// is hidden when the entity already sits at the root (ADR 0039).
func hidesRootChoice(e app.EntityResult, verb string) bool {
	if verb == verbMove && e.IsDisplaced() {
		return e.HomeID == ""
	}
	return isRootEntity(e)
}

// relocationParam reads the ?verb= query parameter of a destination search;
// anything but "use" is a move.
func relocationParam(r *http.Request) app.Relocation {
	if r.URL.Query().Get("verb") == verbUse {
		return app.RelocationUse
	}
	return app.RelocationMove
}
