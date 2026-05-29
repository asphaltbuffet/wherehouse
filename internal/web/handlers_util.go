package web

import (
	"bytes"
	"net/http"
	"strings"

	"github.com/asphaltbuffet/wherehouse/internal/app"
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

// isRootEntity returns true when e has no parent (no colon in path = depth 0).
func isRootEntity(e app.EntityResult) bool {
	return !strings.Contains(e.FullPathDisplay, ":")
}
