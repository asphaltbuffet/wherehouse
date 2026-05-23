# `wherehouse serve` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `wherehouse serve` command that starts a local HTTP server rendering the inventory as a lazy-expanding tree with a detail pane showing status, date added, and event history.

**Architecture:** A thin `cmd/serve/` shell delegates everything to a new `internal/web` package. `internal/web` owns the `*http.Server`, HTMX-driven HTML templates, all handlers, and embedded static assets. The app layer is accessed through a typed `web.App` interface backed by the existing `*app.App`. One new `app.App.GetChildren` method wraps the already-existing `store.GetChildren`.

**Tech Stack:** Go stdlib `net/http` + `html/template`, HTMX 2.x (vendored, no build step), `//go:embed` for assets, `httptest` for unit testing.

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `internal/app/entities.go` | Modify | Add `GetChildren(ctx, parentID string) ([]EntityResult, error)` |
| `internal/web/doc.go` | Create | Package doc |
| `internal/web/app.go` | Create | `web.App` interface + `//go:generate mockery` |
| `internal/web/server.go` | Create | `Config`, `Server`, `New`, `Run` |
| `internal/web/routes.go` | Create | `(*Server).registerRoutes` |
| `internal/web/handlers.go` | Create | HTTP handler methods |
| `internal/web/templates.go` | Create | `parseTemplates(fsys fs.FS)` |
| `internal/web/assets/templates/index.html` | Create | Full page shell |
| `internal/web/assets/templates/tree_node.html` | Create | Tree node HTMX fragment |
| `internal/web/assets/templates/detail.html` | Create | Detail pane HTMX fragment |
| `internal/web/assets/static/htmx.min.js` | Create | Vendored HTMX 2.x |
| `internal/web/assets/static/app.css` | Create | Minimal stylesheet |
| `internal/web/assets/static/VERSION` | Create | Records HTMX version |
| `internal/web/handlers_test.go` | Create | httptest handler tests |
| `internal/web/server_test.go` | Create | `New` / `Run` tests |
| `cmd/serve/doc.go` | Create | Package doc |
| `cmd/serve/serve.go` | Create | Command shell only — constructors, flag wiring, `runServe` |
| `cmd/serve/serve_test.go` | Create | Cobra wiring tests |
| `cmd/root.go` | Modify | Register `serve.NewDefaultServeCmd()` |

---

## Task 1: Add `app.App.GetChildren`

The tree's HTMX expand endpoint needs children of a given parent entity. `EntityResult` intentionally omits `ParentID`, so we add a minimal app-layer wrapper around the already-existing `store.GetChildren`.

**Files:**
- Modify: `internal/app/entities.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/app/entities_test.go` (create the file if it does not exist — check first with `fd entities_test internal/app`):

```go
func TestApp_GetChildren(t *testing.T) {
    // Uses the in-memory SQLite test helpers already established in the package.
    // If this file doesn't exist yet, look at internal/app/*_test.go for the
    // pattern used to create a test store (likely testutil or an in-process DB).
    // Adapt accordingly — the key assertions are below.

    ctx := context.Background()
    store := mustOpenTestStore(t)   // reuse whatever helper the package uses
    a := New(store)

    // Seed a parent and two children via the store directly (bypass events for speed).
    parentID := "parent-01"
    child1ID := "child-01"
    child2ID := "child-02"
    mustInsertEntity(t, store, parentID, "Parent", nil)
    mustInsertEntity(t, store, child1ID, "Child1", &parentID)
    mustInsertEntity(t, store, child2ID, "Child2", &parentID)

    results, err := a.GetChildren(ctx, parentID)
    require.NoError(t, err)
    require.Len(t, results, 2)

    ids := []string{results[0].EntityID, results[1].EntityID}
    assert.ElementsMatch(t, []string{child1ID, child2ID}, ids)
}
```

- [ ] **Step 2: Check existing test helpers in `internal/app`**

```bash
fd '_test' internal/app
```

Look at the top of any `*_test.go` file to find how the package opens a test store and inserts entities. Adopt the same helpers — do NOT invent new ones.

- [ ] **Step 3: Run the test to confirm it fails (function not defined)**

```bash
gotestsum -- -run TestApp_GetChildren ./internal/app/...
```

Expected: compile error — `a.GetChildren undefined`.

- [ ] **Step 4: Implement `GetChildren`**

In `internal/app/entities.go`, add after the `ListEntities` method:

```go
// GetChildren returns direct children of parentID, excluding removed entities.
func (a *App) GetChildren(ctx context.Context, parentID string) ([]EntityResult, error) {
    entities, err := a.store.GetChildren(ctx, parentID)
    if err != nil {
        return nil, fmt.Errorf("get children of %s: %w", parentID, err)
    }

    results := make([]EntityResult, 0, len(entities))
    for _, e := range entities {
        if e.Status != inventory.EntityStatusRemoved {
            results = append(results, entityToResult(e))
        }
    }
    return results, nil
}
```

- [ ] **Step 5: Run test to verify it passes**

```bash
gotestsum -- -run TestApp_GetChildren ./internal/app/...
```

Expected: PASS.

- [ ] **Step 6: Lint**

```bash
mise run lint
```

Expected: no new errors.

- [ ] **Step 7: Commit**

```bash
jj new -m "feat(app): add GetChildren method"
jj file track internal/app/entities.go
```

---

## Task 2: Define `internal/web` package skeleton

Sets up the package boundary, the `web.App` interface, and `web.Config`/`web.Server` structs — no actual HTTP yet. Everything compiles cleanly after this task.

**Files:**
- Create: `internal/web/doc.go`
- Create: `internal/web/app.go`
- Create: `internal/web/server.go`

- [ ] **Step 1: Create `internal/web/doc.go`**

```go
// Package web provides the HTTP server and web UI for wherehouse.
package web
```

- [ ] **Step 2: Create `internal/web/app.go`**

```go
package web

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

//go:generate mockery

// App is the dependency contract the web package requires from the app layer.
type App interface {
	ListEntities(ctx context.Context) ([]app.EntityResult, error)
	GetChildren(ctx context.Context, parentID string) ([]app.EntityResult, error)
	GetEntityByPath(ctx context.Context, path string) (app.EntityResult, error)
	GetHistory(ctx context.Context, req app.GetHistoryRequest) ([]app.HistoryResult, error)
}
```

- [ ] **Step 3: Create `internal/web/server.go` (skeleton — no HTTP yet)**

```go
package web

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
)

const shutdownTimeout = 5 * 1e9 // 5 * time.Second, avoid importing "time" ambiguity

// Config holds all configuration for the web server.
type Config struct {
	App    App
	Bind   string
	Port   int
	Output io.Writer // destination for the startup URL line; defaults to os.Stdout
}

// Server is the wherehouse web server.
type Server struct {
	cfg       Config
	httpSrv   *http.Server
	templates *template.Template
}

// New constructs a Server from cfg. Returns an error if template parsing fails.
func New(cfg Config) (*Server, error) {
	if cfg.Output == nil {
		cfg.Output = os.Stdout
	}
	if cfg.Bind == "" {
		cfg.Bind = "127.0.0.1"
	}
	if cfg.Port == 0 {
		cfg.Port = 8080
	}

	tmpl, err := parseTemplates(assetsFS)
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	mux := http.NewServeMux()
	srv := &Server{
		cfg:       cfg,
		templates: tmpl,
		httpSrv: &http.Server{
			Addr:    fmt.Sprintf("%s:%d", cfg.Bind, cfg.Port),
			Handler: mux,
		},
	}
	srv.registerRoutes(mux)
	return srv, nil
}
```

NOTE: `assetsFS` and `parseTemplates` are defined in Tasks 4 and 5. The package won't compile until those tasks are done — that's intentional; we're building breadth-first.

- [ ] **Step 4: Verify package at least parses (expected compile error about missing symbols)**

```bash
go build ./internal/web/... 2>&1 | head -20
```

Expected: errors about `assetsFS`, `parseTemplates`, `registerRoutes` — not unexpected type errors. If you see unexpected type errors, fix them now.

---

## Task 3: Embedded assets + template parsing

Creates the asset files and wires `//go:embed`. No meaningful HTML yet — placeholder templates so the package compiles end-to-end.

**Files:**
- Create: `internal/web/templates.go`
- Create: `internal/web/assets/templates/index.html` (placeholder)
- Create: `internal/web/assets/templates/tree_node.html` (placeholder)
- Create: `internal/web/assets/templates/detail.html` (placeholder)
- Create: `internal/web/assets/static/htmx.min.js`
- Create: `internal/web/assets/static/app.css`
- Create: `internal/web/assets/static/VERSION`

- [ ] **Step 1: Download and vendor HTMX 2.x**

```bash
curl -L https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js \
  -o /home/grue/dev/wherehouse/internal/web/assets/static/htmx.min.js
```

Verify the download succeeded (file should be ~50 KB):
```bash
wc -c internal/web/assets/static/htmx.min.js
```

- [ ] **Step 2: Write `internal/web/assets/static/VERSION`**

```
htmx 2.0.4
https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js
```

- [ ] **Step 3: Write `internal/web/assets/static/app.css`**

```css
*, *::before, *::after { box-sizing: border-box; }

body {
  font-family: system-ui, sans-serif;
  margin: 0;
  display: flex;
  height: 100vh;
  overflow: hidden;
  background: #fafafa;
  color: #1a1a1a;
}

#tree-panel {
  width: 320px;
  min-width: 200px;
  border-right: 1px solid #ddd;
  overflow-y: auto;
  padding: 1rem 0.5rem;
}

#detail-panel {
  flex: 1;
  overflow-y: auto;
  padding: 1.5rem;
}

.tree-node {
  list-style: none;
  padding: 0;
  margin: 0;
}

.tree-node li {
  padding: 2px 0;
}

.tree-item {
  display: flex;
  align-items: center;
  gap: 0.4em;
  padding: 3px 6px;
  border-radius: 4px;
  cursor: pointer;
  user-select: none;
}

.tree-item:hover { background: #e8e8e8; }
.tree-item.selected { background: #d0e8ff; }

.tree-children { padding-left: 1.2em; }

.badge {
  font-size: 0.7em;
  padding: 1px 5px;
  border-radius: 10px;
  background: #e0e0e0;
}
.badge.ok      { background: #c6f6d5; color: #276749; }
.badge.borrowed { background: #fefcbf; color: #744210; }
.badge.loaned  { background: #bee3f8; color: #2a4365; }
.badge.missing { background: #fed7d7; color: #822727; }

.detail-kv { display: grid; grid-template-columns: 140px 1fr; row-gap: 0.4rem; }
.detail-kv dt { font-weight: 600; color: #555; }
.detail-kv dd { margin: 0; }

.history-table { width: 100%; border-collapse: collapse; font-size: 0.85em; margin-top: 1rem; }
.history-table th, .history-table td { text-align: left; padding: 4px 8px; border-bottom: 1px solid #eee; }
.history-table th { background: #f0f0f0; }
```

- [ ] **Step 4: Write placeholder `internal/web/assets/templates/index.html`**

```html
{{define "index"}}<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Wherehouse</title>
  <link rel="stylesheet" href="/static/app.css">
  <script src="/static/htmx.min.js"></script>
</head>
<body>
  <nav id="tree-panel">
    <ul class="tree-node">
      {{range .Roots}}
        {{template "tree_node" .}}
      {{end}}
    </ul>
  </nav>
  <main id="detail-panel">
    <p style="color:#888">Select an item to see details.</p>
  </main>
</body>
</html>{{end}}
```

- [ ] **Step 5: Write `internal/web/assets/templates/tree_node.html`**

```html
{{define "tree_node"}}
<li>
  <div class="tree-item"
       hx-get="/tree/{{.EntityID}}/children"
       hx-target="next .tree-children"
       hx-swap="innerHTML"
       hx-trigger="click[!this.dataset.expanded] once"
       hx-on:htmx:after-request="this.dataset.expanded='1'"
       hx-push-url="false"
       onclick="document.querySelectorAll('.tree-item').forEach(el=>el.classList.remove('selected')); this.classList.add('selected');
                htmx.ajax('GET','/entities/{{.EntityID}}',{target:'#detail-panel',swap:'innerHTML'})">
    <span>{{entityTypeIcon .EntityType}}</span>
    <span>{{.DisplayName}}</span>
    {{if ne .Status 0}}<span class="badge {{statusClass .Status}}">{{.Status}}</span>{{end}}
  </div>
  <ul class="tree-children tree-node"></ul>
</li>
{{end}}
```

- [ ] **Step 6: Write `internal/web/assets/templates/detail.html`**

```html
{{define "detail"}}
<section>
  <h2>{{.Entity.DisplayName}}</h2>
  <dl class="detail-kv">
    <dt>Type</dt>      <dd>{{.Entity.EntityType}}</dd>
    <dt>Path</dt>      <dd>{{.Entity.FullPathDisplay}}</dd>
    <dt>Status</dt>    <dd>{{.Entity.Status}}{{if .Entity.StatusContext}} — {{.Entity.StatusContext}}{{end}}</dd>
    <dt>Date added</dt><dd>{{.DateAdded}}</dd>
  </dl>

  {{if .History}}
  <h3>History</h3>
  <table class="history-table">
    <thead><tr><th>When</th><th>Event</th><th>By</th><th>Note</th></tr></thead>
    <tbody>
    {{range .History}}
      <tr>
        <td>{{.TimestampUTC}}</td>
        <td>{{.EventType}}</td>
        <td>{{.ActorUserID}}</td>
        <td>{{.Note}}</td>
      </tr>
    {{end}}
    </tbody>
  </table>
  {{end}}
</section>
{{end}}
```

- [ ] **Step 7: Create `internal/web/templates.go`**

```go
package web

import (
	"fmt"
	"html/template"
	"io/fs"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// parseTemplates parses all templates from the templates/ subdirectory of fsys.
func parseTemplates(fsys fs.FS) (*template.Template, error) {
	sub, err := fs.Sub(fsys, "assets/templates")
	if err != nil {
		return nil, fmt.Errorf("sub templates: %w", err)
	}

	funcMap := template.FuncMap{
		"entityTypeIcon": entityTypeIcon,
		"statusClass":    statusClass,
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(sub, "*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	return tmpl, nil
}

func entityTypeIcon(t inventory.EntityType) string {
	switch t {
	case inventory.EntityTypePlace:
		return "🏠"
	case inventory.EntityTypeContainer:
		return "📦"
	case inventory.EntityTypeLeaf:
		return "🔧"
	default:
		return "•"
	}
}

func statusClass(s inventory.EntityStatus) string {
	switch s {
	case inventory.EntityStatusOk:
		return "ok"
	case inventory.EntityStatusBorrowed:
		return "borrowed"
	case inventory.EntityStatusLoaned:
		return "loaned"
	case inventory.EntityStatusMissing:
		return "missing"
	default:
		return ""
	}
}
```

NOTE: The `//go:embed` directive for `assetsFS` lives in `routes.go` (Task 4). `parseTemplates` receives the FS as a parameter so it can be tested with an in-memory FS.

- [ ] **Step 8: Verify compile progress**

```bash
go build ./internal/web/... 2>&1 | head -20
```

Expected: errors about `assetsFS`, `registerRoutes` — not unexpected type errors.

---

## Task 4: Routes + static file serving

Adds the `//go:embed` var, `registerRoutes`, and the `/healthz` + static handler. The package now compiles fully.

**Files:**
- Create: `internal/web/routes.go`

- [ ] **Step 1: Create `internal/web/routes.go`**

```go
package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed assets
var assetsFS embed.FS

// registerRoutes wires all URL patterns onto mux.
func (s *Server) registerRoutes(mux *http.ServeMux) {
	staticSub, _ := fs.Sub(assetsFS, "assets/static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("GET /tree/{entityID}/children", s.handleTreeChildren)
	mux.HandleFunc("GET /entities/{entityID}", s.handleEntityDetail)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}
```

- [ ] **Step 2: Verify the package compiles (only `handlers.go` missing now)**

```bash
go build ./internal/web/... 2>&1 | head -20
```

Expected: errors about `handleIndex`, `handleTreeChildren`, `handleEntityDetail` — not type errors.

---

## Task 5: HTTP handlers

Implements the three page handlers. Each follows the pattern: extract path param → call `s.cfg.App` → render template fragment.

**Files:**
- Create: `internal/web/handlers.go`

- [ ] **Step 1: Create `internal/web/handlers.go`**

```go
package web

import (
	"fmt"
	"net/http"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

// handleIndex renders the full page shell with root-level tree nodes.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	entities, err := s.cfg.App.ListEntities(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("list entities: %v", err), http.StatusInternalServerError)
		return
	}

	// Root entities have an empty ParentID in the underlying store, but EntityResult
	// doesn't expose ParentID. We identify roots by Depth == 0 via FullPathDisplay
	// having no colon separator (single segment = depth 0).
	var roots []app.EntityResult
	for _, e := range entities {
		if isRootEntity(e) {
			roots = append(roots, e)
		}
	}

	data := struct{ Roots []app.EntityResult }{Roots: roots}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "index", data); err != nil {
		// Template partially written — log but don't call http.Error (headers sent).
		_ = fmt.Sprintf("execute index template: %v", err)
	}
}

// handleTreeChildren returns the direct children of an entity as tree_node fragments.
func (s *Server) handleTreeChildren(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entityID")
	children, err := s.cfg.App.GetChildren(r.Context(), entityID)
	if err != nil {
		http.Error(w, fmt.Sprintf("get children: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	for _, child := range children {
		if err := s.templates.ExecuteTemplate(w, "tree_node", child); err != nil {
			_ = fmt.Sprintf("execute tree_node template: %v", err)
			return
		}
	}
}

// handleEntityDetail returns the detail pane fragment for a single entity.
func (s *Server) handleEntityDetail(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entityID")

	// Fetch entity by ID — GetEntityByPath expects a canonical path, so we use
	// GetHistory to resolve entity metadata; or we call GetChildren on "" to find
	// roots ... actually GetEntityByPath takes a string path, not an ID.
	// For the detail pane we have the ID; build a GetHistory call to get metadata
	// and history in one pass, then call ListEntities filtered by EntityID.
	entities, err := s.cfg.App.ListEntities(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("list entities: %v", err), http.StatusInternalServerError)
		return
	}
	var entity *app.EntityResult
	for i := range entities {
		if entities[i].EntityID == entityID {
			entity = &entities[i]
			break
		}
	}
	if entity == nil {
		http.Error(w, "entity not found", http.StatusNotFound)
		return
	}

	history, err := s.cfg.App.GetHistory(r.Context(), app.GetHistoryRequest{
		EntityID:    entityID,
		Limit:       50,
		OldestFirst: true, // oldest first so index 0 = creation event
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("get history: %v", err), http.StatusInternalServerError)
		return
	}

	dateAdded := ""
	if len(history) > 0 {
		dateAdded = history[0].TimestampUTC
	}
	// Flip to newest-first for display.
	for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
		history[i], history[j] = history[j], history[i]
	}

	data := struct {
		Entity    app.EntityResult
		DateAdded string
		History   []app.HistoryResult
	}{
		Entity:    *entity,
		DateAdded: dateAdded,
		History:   history,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "detail", data); err != nil {
		_ = fmt.Sprintf("execute detail template: %v", err)
	}
}

// isRootEntity returns true when e has no parent (depth 0: no colon in canonical name).
func isRootEntity(e app.EntityResult) bool {
	for _, c := range e.CanonicalName {
		if c == ':' {
			return false
		}
	}
	return true
}
```

- [ ] **Step 2: Verify the package compiles cleanly**

```bash
go build ./internal/web/...
```

Expected: exit 0, no output.

---

## Task 6: `(*Server).Run` — lifecycle

Adds graceful shutdown to `server.go`.

**Files:**
- Modify: `internal/web/server.go`

- [ ] **Step 1: Add `Run` and the `time` import to `server.go`**

Append the following to `internal/web/server.go` (after the `New` function):

```go
// Run starts the HTTP server, prints the URL, and blocks until ctx is cancelled
// or an OS interrupt (SIGINT/SIGTERM) is received, then shuts down gracefully.
func (s *Server) Run(ctx context.Context) error {
	addr := fmt.Sprintf("http://%s:%d", s.cfg.Bind, s.cfg.Port)
	fmt.Fprintln(s.cfg.Output, "Listening on", addr)

	errCh := make(chan error, 1)
	go func() {
		if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.httpSrv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}
```

Also update the imports block at the top of `server.go` to include `"context"` and `"time"` (remove the `shutdownTimeout` const you wrote as a placeholder — use `5*time.Second` directly as shown above):

The final imports block for `server.go`:
```go
import (
	"context"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"time"
)
```

Remove the `shutdownTimeout` constant line.

- [ ] **Step 2: Verify compile**

```bash
go build ./internal/web/...
```

Expected: exit 0.

---

## Task 7: Tests for `internal/web`

Adds handler tests and a `New`/`Run` smoke test.

**Files:**
- Create: `internal/web/handlers_test.go`
- Create: `internal/web/server_test.go`

- [ ] **Step 1: Write `internal/web/handlers_test.go`**

```go
package web_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/web"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeApp satisfies web.App for tests.
type fakeApp struct {
	entities []app.EntityResult
	history  []app.HistoryResult
	err      error
}

func (f *fakeApp) ListEntities(_ context.Context) ([]app.EntityResult, error) {
	return f.entities, f.err
}

func (f *fakeApp) GetChildren(_ context.Context, parentID string) ([]app.EntityResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []app.EntityResult
	for _, e := range f.entities {
		// In test data, prefix matching on CanonicalName simulates parent lookup.
		// Real code passes parentID to the store; here we just return all entities
		// whose display name starts with "Child" and parentID matches.
		if e.EntityID == parentID+"_child" {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeApp) GetEntityByPath(_ context.Context, _ string) (app.EntityResult, error) {
	if len(f.entities) == 0 {
		return app.EntityResult{}, f.err
	}
	return f.entities[0], f.err
}

func (f *fakeApp) GetHistory(_ context.Context, _ app.GetHistoryRequest) ([]app.HistoryResult, error) {
	return f.history, f.err
}

func newTestServer(t *testing.T, a web.App) *httptest.Server {
	t.Helper()
	srv, err := web.New(web.Config{
		App:    a,
		Bind:   "127.0.0.1",
		Port:   0, // unused by httptest
		Output: io.Discard,
	})
	require.NoError(t, err)
	return httptest.NewServer(srv.Handler())
}

func TestHandleIndex(t *testing.T) {
	fake := &fakeApp{entities: []app.EntityResult{
		{EntityID: "abc", DisplayName: "Garage", CanonicalName: "garage",
			EntityType: inventory.EntityTypePlace, Status: inventory.EntityStatusOk},
	}}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "Garage")
}

func TestHandleHealthz(t *testing.T) {
	ts := newTestServer(t, &fakeApp{})
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHandleEntityDetail_NotFound(t *testing.T) {
	ts := newTestServer(t, &fakeApp{entities: []app.EntityResult{}})
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/entities/nonexistent")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHandleEntityDetail_Found(t *testing.T) {
	history := []app.HistoryResult{
		{EventID: 1, TimestampUTC: "2025-01-01T00:00:00Z", ActorUserID: "alice"},
	}
	entities := []app.EntityResult{
		{EntityID: "xyz", DisplayName: "Hammer", CanonicalName: "garage:toolbox:hammer",
			EntityType: inventory.EntityTypeLeaf, Status: inventory.EntityStatusOk},
	}
	ts := newTestServer(t, &fakeApp{entities: entities, history: history})
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/entities/xyz")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	bs := string(body)
	assert.Contains(t, bs, "Hammer")
	assert.Contains(t, bs, "2025-01-01T00:00:00Z")
	assert.True(t, strings.Contains(bs, "alice"))
}
```

The tests reference `srv.Handler()` — a method we need to expose. See Step 2.

- [ ] **Step 2: Add `Handler()` accessor to `internal/web/server.go`**

Append to `server.go`:

```go
// Handler returns the underlying http.Handler (for use in httptest).
func (s *Server) Handler() http.Handler {
	return s.httpSrv.Handler
}
```

- [ ] **Step 3: Write `internal/web/server_test.go`**

```go
package web_test

import (
	"context"
	"io"
	"testing"

	"github.com/asphaltbuffet/wherehouse/internal/web"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_DefaultsApplied(t *testing.T) {
	srv, err := web.New(web.Config{App: &fakeApp{}, Output: io.Discard})
	require.NoError(t, err)
	assert.NotNil(t, srv)
}

func TestRun_ShutdownOnContextCancel(t *testing.T) {
	srv, err := web.New(web.Config{
		App:    &fakeApp{},
		Bind:   "127.0.0.1",
		Port:   18080,
		Output: io.Discard,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Run(ctx) }()

	cancel()
	err = <-errCh
	assert.NoError(t, err)
}
```

- [ ] **Step 4: Run all web tests**

```bash
gotestsum -- -race ./internal/web/...
```

Expected: all PASS.

- [ ] **Step 5: Lint**

```bash
mise run lint
```

Expected: no new errors.

- [ ] **Step 6: Commit**

```bash
jj new -m "feat(web): add internal/web package with HTTP server and handlers"
jj file track internal/web/
```

---

## Task 8: `cmd/serve` shell

Thin Cobra command that wires flags → `web.Config` → `web.New` → `srv.Run`.

**Files:**
- Create: `cmd/serve/doc.go`
- Create: `cmd/serve/serve.go`
- Create: `cmd/serve/serve_test.go`

- [ ] **Step 1: Create `cmd/serve/doc.go`**

```go
// Package serve provides the "wherehouse serve" command.
package serve
```

- [ ] **Step 2: Create `cmd/serve/serve.go`**

```go
package serve

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/web"
)

// NewDefaultServeCmd returns the serve command wired to the real database.
func NewDefaultServeCmd() *cobra.Command {
	cmd := buildServeCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		s, a, err := cli.OpenDatabase(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer s.Close()
		return runServe(cmd, a)
	}
	return cmd
}

// NewServeCmd returns the serve command with a injected app (for tests).
func NewServeCmd(a web.App) *cobra.Command {
	cmd := buildServeCmd()
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		return runServe(cmd, a)
	}
	return cmd
}

func buildServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start a local web server to browse the inventory",
		Long: `Start a local HTTP server that renders the inventory as a navigable
tree in your browser. The server is read-only — use the CLI commands to modify
inventory.

Examples:
  wherehouse serve                   # Listen on 127.0.0.1:8080
  wherehouse serve --port 9090
  wherehouse serve --bind 0.0.0.0   # Listen on all interfaces (LAN)`,
		Args: cobra.NoArgs,
	}
	cmd.Flags().IntP("port", "p", 8080, "port to listen on")
	cmd.Flags().String("bind", "127.0.0.1", "address to bind to")
	return cmd
}

func runServe(cmd *cobra.Command, a web.App) error {
	port, err := cmd.Flags().GetInt("port")
	if err != nil {
		return fmt.Errorf("get port flag: %w", err)
	}
	bind, err := cmd.Flags().GetString("bind")
	if err != nil {
		return fmt.Errorf("get bind flag: %w", err)
	}

	srv, err := web.New(web.Config{
		App:    a,
		Bind:   bind,
		Port:   port,
		Output: cmd.OutOrStdout(),
	})
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}
	return srv.Run(cmd.Context())
}
```

- [ ] **Step 3: Create `cmd/serve/serve_test.go`**

```go
package serve_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/cmd/serve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeApp struct{}

func (f *fakeApp) ListEntities(_ context.Context) ([]app.EntityResult, error) { return nil, nil }
func (f *fakeApp) GetChildren(_ context.Context, _ string) ([]app.EntityResult, error) {
	return nil, nil
}
func (f *fakeApp) GetEntityByPath(_ context.Context, _ string) (app.EntityResult, error) {
	return app.EntityResult{}, nil
}
func (f *fakeApp) GetHistory(_ context.Context, _ app.GetHistoryRequest) ([]app.HistoryResult, error) {
	return nil, nil
}

func TestBuildServeCmd_FlagDefaults(t *testing.T) {
	cmd := serve.NewServeCmd(&fakeApp{})

	port, err := cmd.Flags().GetInt("port")
	require.NoError(t, err)
	assert.Equal(t, 8080, port)

	bind, err := cmd.Flags().GetString("bind")
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", bind)
}

func TestBuildServeCmd_FlagOverride(t *testing.T) {
	cmd := serve.NewServeCmd(&fakeApp{})
	var out bytes.Buffer
	cmd.SetOut(&out)

	// Cancel immediately so Run doesn't block.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd.SetContext(ctx)

	err := cmd.ParseFlags([]string{"--port", "9090", "--bind", "0.0.0.0"})
	require.NoError(t, err)

	port, _ := cmd.Flags().GetInt("port")
	bind, _ := cmd.Flags().GetString("bind")
	assert.Equal(t, 9090, port)
	assert.Equal(t, "0.0.0.0", bind)
}
```

- [ ] **Step 4: Run cmd/serve tests**

```bash
gotestsum -- -race ./cmd/serve/...
```

Expected: all PASS.

---

## Task 9: Register command + smoke test

Wires `serve` into the root command and verifies the full binary works.

**Files:**
- Modify: `cmd/root.go`

- [ ] **Step 1: Add import and registration to `cmd/root.go`**

In `cmd/root.go`, add the import:
```go
"github.com/asphaltbuffet/wherehouse/cmd/serve"
```

And in `GetRootCmd()`, after line 69, add:
```go
rootCmd.AddCommand(serve.NewDefaultServeCmd())
```

- [ ] **Step 2: Build the binary**

```bash
mise run build
```

Expected: `dist/wherehouse` (or equivalent) builds cleanly.

- [ ] **Step 3: Verify help text**

```bash
./dist/wherehouse serve --help
```

Expected output includes `Start a local web server`, `--port`, `--bind`.

- [ ] **Step 4: Run the full test suite**

```bash
mise run test
```

Expected: all tests PASS, no race conditions.

- [ ] **Step 5: Lint**

```bash
mise run lint
```

Expected: no new errors.

- [ ] **Step 6: Architectural compliance check**

```bash
rg -l '"net/http"|"html/template"|go:embed' cmd/serve/
```

Expected: no output (cmd/serve has none of these imports).

- [ ] **Step 7: Commit**

```bash
jj new -m "feat: add wherehouse serve command"
jj file track cmd/serve/ cmd/root.go
```

---

## Task 10: End-to-end manual smoke test

Verifies the server works with a real database and responds correctly in a browser.

**Files:** None.

- [ ] **Step 1: Initialize test inventory**

```bash
./dist/wherehouse migrate
./dist/wherehouse add place Garage
./dist/wherehouse add container Toolbox --in Garage
./dist/wherehouse add leaf Hammer --in Garage:Toolbox
./dist/wherehouse add leaf Wrench --in Garage:Toolbox
```

- [ ] **Step 2: Start the server**

```bash
./dist/wherehouse serve --port 8080
```

Expected first line of output: `Listening on http://127.0.0.1:8080`

- [ ] **Step 3: Healthz check (new terminal)**

```bash
curl -sf http://127.0.0.1:8080/healthz && echo OK
```

Expected: `OK`

- [ ] **Step 4: Index contains Garage**

```bash
curl -sf http://127.0.0.1:8080/ | grep -q Garage && echo "root tree OK"
```

Expected: `root tree OK`

- [ ] **Step 5: Browser walkthrough (manual)**

Open `http://127.0.0.1:8080` in a browser and verify:
- Garage appears in the tree panel
- Clicking Garage expands it to show Toolbox
- Clicking Toolbox expands it to show Hammer and Wrench
- Clicking Hammer shows a detail pane with: Type = leaf, Path = Garage:Toolbox:Hammer, Status = ok, Date added populated, event history row visible
- Ctrl-C in the server terminal → server exits cleanly (no error message)

---

## Self-review

**Spec coverage check:**
- [x] `--port`/`-p` flag → Task 8
- [x] `--bind` flag (configurable, default 127.0.0.1) → Task 8
- [x] Web page with filetree-like display → Tasks 3–5 (index + tree_node templates + HTMX lazy expand)
- [x] Selecting entity shows details on right → Tasks 3, 5 (detail template + handleEntityDetail)
- [x] Date added → Task 5 (oldest event from history)
- [x] Status displayed → Task 3/5 (badge in tree node, dl in detail pane)
- [x] Read-only scope → no write routes added anywhere
- [x] Graceful shutdown on Ctrl-C → Task 6 (`Run` with signal context)
- [x] `cmd/serve/` is shell only, no HTTP/template/embed → Tasks 8 + compliance check in Task 9
- [x] Business logic in `internal/web` → Tasks 2–7
- [x] `app.GetChildren` needed for tree → Task 1

**Placeholder scan:** None found — all steps include actual code.

**Type consistency check:**
- `web.App` interface in `app.go`: `GetHistory(ctx, app.GetHistoryRequest)` — matches `app.GetHistoryRequest` struct defined in `internal/app/requests.go:47`
- `web.Config.App` is `web.App` — `*app.App` satisfies this interface (all four methods confirmed from source)
- `fakeApp` in `serve_test.go` and `handlers_test.go` both implement the same four `web.App` methods
- `srv.Handler()` accessor added in Task 7 Step 2 before it's used in test helper `newTestServer`
- Template names `"index"`, `"tree_node"`, `"detail"` are consistent between `{{define ...}}` blocks and `ExecuteTemplate` calls
