# Web UI Actor From Trusted Identity Header — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Record the reverse-proxy-asserted user (e.g. `Tailscale-User-Login`) as the event actor for web UI mutations, show it in the page header, and close #267 and #145.

**Architecture:** `internal/web` gains an `ActorHeader` setting on `web.Config`; a single `Server.resolveActor` reads that header per request, falling back to the constant `webui`. All three mutation paths and the page header use it; the add form's free-text "User" field is removed. The setting flows from `--actor-header` > `WHEREHOUSE_WEB_ACTOR_HEADER` > `[web] actor_header`, resolved in the thin `cmd/serve` shell. `Server.Run` warns on stderr when the header is trusted on a non-loopback bind.

**Tech Stack:** Go 1.25, `net/http`, `html/template`, cobra, viper, testify, jj (no git), mise.

**Spec:** `docs/adr/0031-web-authentication-delegated-to-trusted-proxy.md` (decision record), GitHub issue #267 (feature), `CONTEXT.md` → **Actor** (glossary). Follow-up out of scope: #271 (actor mapping).

## Global Constraints

- VCS is **jujutsu**. Never run `git`. Commit = `jj describe -m "<msg>"` then `jj new`. Run `jj st` before starting each task to confirm `@` is empty.
- Before each commit: `mise run lint` and `mise run test` must both pass with zero warnings.
- Conventional commit prefixes: `feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `chore:`.
- `cmd/serve/` stays a thin shell: `rg -l '"net/http"|"html/template"|//go:embed' cmd/serve/` must print nothing.
- Fallback actor string is exactly `webui`.
- Header label wording is exactly `acting as <actor>` — never "signed in".
- Header value is recorded **verbatim after `strings.TrimSpace`**; no mapping (that is #271).
- Non-loopback warning **does not** prevent startup.
- The web UI must never let the visitor choose their own actor (no form field, no query param).
- Prefer hand-rolled fakes (`fakeApp`), not mockery. `testify/require` for preconditions, `assert` otherwise.
- No `init()` functions, no boolean parameters, no magic numbers.
- Use `fd -H` / `rg` / `sd`, never `find` / `grep` / `sed`. No `&&` chaining in shell.

## Review Focus

1. **Spoofing while the feature is off** — a request carrying `Tailscale-User-Login` when no `ActorHeader` is configured must still be recorded as `webui`. (Task 1 test `off, header sent`.)
2. **Case mismatch between config and proxy** — `actor_header = "tailscale-user-login"` must match the proxy's `Tailscale-User-Login`. (Task 1 test `lowercase config name`.)
3. **Hostile header value rendered in the page** — `<script>` in the header value must appear HTML-escaped in the `acting as` label. (Task 2 test `TestHeader_EscapesActor`.)
4. **Stale form field still posted** — an old cached form, or a hand-crafted POST, that sends `user=mallory` must be ignored. (Task 1 tests `form user ignored …`.)
5. **Malformed header name in config/env** — `Tailscale User` or `X:Y` must fail at startup with a clear error instead of silently never matching. (Task 1 `TestNew_ActorHeaderValidation`, Task 5 `invalid flag`.)

---

## File Structure

| File | Change | Responsibility |
|---|---|---|
| `internal/web/actor.go` | **Create** | `defaultActor`, `resolveActor`, `actorFor`, `actorView`, `isHeaderToken`, `isLoopbackBind` — everything about "who is acting" |
| `internal/web/actor_test.go` | **Create** | Black-box tests for actor resolution, header label, validation |
| `internal/web/actor_internal_test.go` | **Create** | `isLoopbackBind` table test (unexported) |
| `internal/web/server.go` | Modify | `Config.ActorHeader`, `Config.ErrOutput`, validation in `New`, warning in `Run` |
| `internal/web/server_test.go` | Modify | Startup-warning tests |
| `internal/web/handlers_add.go` | Modify | Use `s.actorFor(r)`; drop form `user` read |
| `internal/web/handlers_entity.go` | Modify | Use `s.actorFor(r)` ×2; `detailPageData.Actor` |
| `internal/web/handlers_browse.go` | Modify | `indexData.Actor`, search page `Actor` |
| `internal/web/handlers_test.go` | Modify | `fakeApp.lastActor`, `newActorHandler` helper |
| `internal/web/assets/templates/add_item_form.html` | Modify | Remove User field |
| `internal/web/assets/templates/partial_header.html` | Modify | `acting as` label |
| `internal/web/assets/templates/{index,detail,search_results}.html` | Modify | Pass `.Actor` to header |
| `internal/web/assets/static/app.css` | Modify | `.app-actor` style |
| `internal/config/config.go` | Modify | `WebConfig` |
| `internal/config/loader.go` | Modify | `envBoundKeys` += `web.actor_header` |
| `internal/config/writer.go` | Modify | default, `GetValue`, `parseConfigValue` |
| `internal/config/{loader,writer}_test.go` | Modify | Coverage for new key |
| `cmd/serve/serve.go` | Modify | `--actor-header` flag, precedence, help text |
| `cmd/serve/serve_test.go` | Modify | Precedence + validation tests |
| `nix/home-manager.nix` | Modify | `settings.web.actorHeader` option |
| `README.md` | Modify | Container identity, Tailscale recipe, config/env samples |

---

### Task 0: Commit the decision docs

ADR 0031 and the `CONTEXT.md` **Actor** update were written during the design session and sit uncommitted in `@`.

**Files:**
- Already modified: `CONTEXT.md`, `docs/adr/0031-web-authentication-delegated-to-trusted-proxy.md`
- Create: `docs/superpowers/plans/2026-10-24-web-actor-header.md` (this file)

- [ ] **Step 1: Confirm working copy contents**

Run: `jj st`
Expected: `M CONTEXT.md`, `A docs/adr/0031-…md`, `A docs/superpowers/plans/2026-10-24-web-actor-header.md` and nothing else.

- [ ] **Step 2: Commit**

```bash
jj describe -m "docs: ADR 0031 delegate web authentication to trusted proxy"
jj new
```

---

### Task 1: Actor resolution in `internal/web` + remove the form field

**Files:**
- Create: `internal/web/actor.go`, `internal/web/actor_test.go`
- Modify: `internal/web/server.go` (Config + `New`), `internal/web/handlers_add.go:68-77`, `internal/web/handlers_entity.go:206,258`, `internal/web/handlers_test.go`, `internal/web/assets/templates/add_item_form.html:17-18`

**Interfaces:**
- Produces:
  - `web.Config.ActorHeader string`
  - `const defaultActor = "webui"` (package `web`)
  - `func (s *Server) resolveActor(r *http.Request) (name string, fromHeader bool)`
  - `func (s *Server) actorFor(r *http.Request) string`
  - `func isHeaderToken(name string) bool`
  - `web.New` returns error `invalid actor header name "<name>"` for bad names
  - Test helpers: `fakeApp.lastActor string`; `newActorHandler(t *testing.T, a web.App, header string) http.Handler`

- [ ] **Step 1: Extend test fixtures**

In `internal/web/handlers_test.go`, add a field to `fakeApp`:

```go
	findErr      error
	lastActor    string // ActorID from the most recent mutating call
}
```

Replace the three mutating fake methods:

```go
func (f *fakeApp) CreateEntity(_ context.Context, req app.CreateEntityRequest) (app.EntityResult, error) {
	f.lastActor = req.ActorID
	return app.EntityResult{}, f.err
}

func (f *fakeApp) RenameEntity(_ context.Context, req app.RenameEntityRequest) (app.EntityResult, error) {
	f.lastActor = req.ActorID
	return f.renameResult, f.renameErr
}

func (f *fakeApp) ChangeStatus(_ context.Context, req app.ChangeStatusRequest) (app.EntityResult, error) {
	f.lastActor = req.ActorID
	return app.EntityResult{}, f.statusErr
}
```

Append a helper (add `"net/http"` to imports). It returns the bare handler so tests call `ServeHTTP` synchronously — reading `fa.lastActor` afterwards is race-free, unlike reading it after a round-trip through `httptest.NewServer`:

```go
// newActorHandler builds a server that trusts header (may be "") for the actor.
func newActorHandler(t *testing.T, a web.App, header string) http.Handler {
	t.Helper()
	srv, err := web.New(web.Config{App: a, Output: io.Discard, ActorHeader: header})
	require.NoError(t, err)
	return srv.Handler()
}
```

- [ ] **Step 2: Write the failing tests**

Create `internal/web/actor_test.go`:

```go
package web_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/web"
)

const tsHeader = "Tailscale-User-Login"

func hammer() []app.EntityResult {
	return []app.EntityResult{
		{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Garage:Hammer",
			Status: inventory.EntityStatusOk},
	}
}

func postForm(target, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Hx-Request", "true")
	return req
}

func TestActor_RecordedOnEveryMutation(t *testing.T) {
	cases := []struct {
		name string
		path string
		body string
	}{
		{"create root", "/entities/add", "display_name=Wrench"},
		{"create child", "/entities/abc/add", "display_name=Wrench"},
		{"rename", "/entities/abc/edit/name", "display_name=Mallet"},
		{"toggle missing", "/entities/abc/actions/toggle-missing", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fa := &fakeApp{entities: hammer(), renameResult: hammer()[0]}
			h := newActorHandler(t, fa, tsHeader)

			req := postForm(tc.path, tc.body)
			req.Header.Set(tsHeader, "alice@example.com")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			assert.Equal(t, "alice@example.com", fa.lastActor)
		})
	}
}

func TestActor_Resolution(t *testing.T) {
	cases := []struct {
		name       string
		configured string // web.Config.ActorHeader
		sent       map[string]string
		body       string
		want       string
	}{
		{"off, no header", "", nil, "", "webui"},
		{"off, header sent", "", map[string]string{tsHeader: "mallory"}, "", "webui"},
		{"on, header absent", tsHeader, nil, "", "webui"},
		{"on, header blank", tsHeader, map[string]string{tsHeader: "   "}, "", "webui"},
		{"on, header present", tsHeader, map[string]string{tsHeader: "alice@example.com"}, "", "alice@example.com"},
		{"on, header padded", tsHeader, map[string]string{tsHeader: "  alice@example.com\t"}, "", "alice@example.com"},
		{"lowercase config name", "tailscale-user-login", map[string]string{tsHeader: "alice@example.com"}, "", "alice@example.com"},
		{"form user ignored when off", "", nil, "user=mallory", "webui"},
		{"form user ignored when on", tsHeader, map[string]string{tsHeader: "alice@example.com"}, "user=mallory", "alice@example.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fa := &fakeApp{}
			h := newActorHandler(t, fa, tc.configured)

			body := "display_name=Wrench"
			if tc.body != "" {
				body += "&" + tc.body
			}
			req := postForm("/entities/add", body)
			for k, v := range tc.sent {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			assert.Equal(t, tc.want, fa.lastActor)
		})
	}
}

func TestNew_ActorHeaderValidation(t *testing.T) {
	cases := []struct {
		header  string
		wantErr bool
	}{
		{"", false},
		{"Tailscale-User-Login", false},
		{"X-Remote_User", false},
		{"Tailscale User", true},
		{"X:Y", true},
		{"X\nY", true},
		{"Ünicode", true},
	}
	for _, tc := range cases {
		t.Run(tc.header, func(t *testing.T) {
			_, err := web.New(web.Config{App: &fakeApp{}, Output: io.Discard, ActorHeader: tc.header})
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid actor header name")
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestAddItemForm_HasNoUserField(t *testing.T) {
	h := newActorHandler(t, &fakeApp{}, "")
	req := httptest.NewRequest(http.MethodGet, "/entities/add", nil)
	req.Header.Set("Hx-Request", "true")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), `name="user"`)
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `gotestsum -- -race -run 'TestActor_|TestNew_ActorHeaderValidation|TestAddItemForm_HasNoUserField' ./internal/web/...`
Expected: compile failure — `unknown field ActorHeader in struct literal of type web.Config`.

- [ ] **Step 4: Add the config field and validation**

In `internal/web/server.go`, extend `Config`:

```go
type Config struct {
	App    App
	Bind   string
	Port   int
	Output io.Writer      // destination for the startup URL line; defaults to os.Stdout
	Logger logging.Logger // structured logger; defaults to logging.GetLogger()

	// ActorHeader names a request header, set by a trusted reverse proxy,
	// whose value is recorded as the event actor. Empty disables it.
	// See ADR 0031.
	ActorHeader string
}
```

In `New`, after the `cfg.Logger` default block and before `parseTemplates`:

```go
	if cfg.ActorHeader != "" && !isHeaderToken(cfg.ActorHeader) {
		return nil, fmt.Errorf("invalid actor header name %q", cfg.ActorHeader)
	}
```

- [ ] **Step 5: Create `internal/web/actor.go`**

```go
package web

import (
	"net/http"
	"strings"
)

// defaultActor is recorded on events when the request carries no trusted
// proxy identity.
const defaultActor = "webui"

// resolveActor returns the actor for r and whether it came from the
// configured identity header. The header is trusted only because ADR 0031
// delegates authentication to a reverse proxy that owns it.
func (s *Server) resolveActor(r *http.Request) (string, bool) {
	if s.cfg.ActorHeader == "" {
		return defaultActor, false
	}
	if v := strings.TrimSpace(r.Header.Get(s.cfg.ActorHeader)); v != "" {
		return v, true
	}
	return defaultActor, false
}

// actorFor returns the actor to record on events produced by r.
func (s *Server) actorFor(r *http.Request) string {
	name, _ := s.resolveActor(r)
	return name
}

// isHeaderToken reports whether name is a valid HTTP field name
// (an RFC 9110 token).
func isHeaderToken(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range []byte(name) {
		if !isTokenChar(c) {
			return false
		}
	}
	return true
}

func isTokenChar(c byte) bool {
	switch {
	case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
		return true
	}
	return strings.IndexByte("!#$%&'*+-.^_`|~", c) >= 0
}
```

- [ ] **Step 6: Use it in the handlers**

`internal/web/handlers_add.go`, in `createAndRenderNode`, delete:

```go
	actor := strings.TrimSpace(r.FormValue("user"))
	if actor == "" {
		actor = "webui"
	}
```

and change the request literal to:

```go
	created, err := s.cfg.App.CreateEntity(r.Context(), app.CreateEntityRequest{
		DisplayName: displayName,
		ParentPath:  parentPath,
		ActorID:     s.actorFor(r),
	})
```

`internal/web/handlers_entity.go`: in both `handleEditName` (`RenameEntityRequest`) and `handleToggleMissing` (`ChangeStatusRequest`) replace `ActorID:  "webui",` with `ActorID:  s.actorFor(r),`.

Verify no literals remain: `rg -n '"webui"' internal/web --glob '!*_test.go'` → only `actor.go`.

- [ ] **Step 7: Remove the form field**

In `internal/web/assets/templates/add_item_form.html` delete these two lines:

```html
  <label for="user">User</label>
  <input id="user" type="text" name="user" placeholder="webui">
```

(The stale `entity_type` select is out of scope — leave it.)

- [ ] **Step 8: Run tests to verify they pass**

Run: `gotestsum -- -race ./internal/web/...`
Expected: all PASS (new and existing).

- [ ] **Step 9: Lint, full test, commit**

```bash
mise run lint
mise run test
jj describe -m "feat(web): record trusted proxy identity header as event actor

Refs #267"
jj new
```

---

### Task 2: "acting as" label in the page header

**Files:**
- Modify: `internal/web/actor.go`, `internal/web/handlers_browse.go:13-17,34-38,118-121`, `internal/web/handlers_entity.go:28-31,154`, `internal/web/assets/templates/partial_header.html:14-22`, `internal/web/assets/templates/index.html:13`, `internal/web/assets/templates/detail.html:82`, `internal/web/assets/templates/search_results.html:51`, `internal/web/assets/static/app.css:220`
- Test: `internal/web/actor_test.go`

**Interfaces:**
- Consumes: `resolveActor`, `tsHeader`, `hammer()`, `newActorHandler` from Task 1
- Produces: `type actorView struct{ Name, Source string }`; `func (s *Server) actorView(r *http.Request) actorView`; template `header` now receives an `actorView` (not the page data)

- [ ] **Step 1: Write the failing tests**

Append to `internal/web/actor_test.go`:

```go
func getPage(t *testing.T, h http.Handler, target string, hdr map[string]string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return rec.Body.String()
}

func TestHeader_ShowsActorOnEveryFullPage(t *testing.T) {
	for _, page := range []string{"/", "/search?q=ham", "/entities/abc"} {
		t.Run(page, func(t *testing.T) {
			h := newActorHandler(t, &fakeApp{entities: hammer()}, tsHeader)
			body := getPage(t, h, page, map[string]string{tsHeader: "alice@example.com"})

			assert.Contains(t, body, "acting as alice@example.com")
			assert.Contains(t, body, `title="from header Tailscale-User-Login"`)
		})
	}
}

func TestHeader_ActorSource(t *testing.T) {
	cases := []struct {
		name       string
		configured string
		sent       map[string]string
		wantText   string
		wantTitle  string
	}{
		{"not configured", "", nil, "acting as webui", `title="no identity header configured"`},
		{"configured, absent", tsHeader, nil, "acting as webui", `title="header Tailscale-User-Login not received"`},
		{"configured lowercase, present", "tailscale-user-login",
			map[string]string{tsHeader: "alice@example.com"},
			"acting as alice@example.com", `title="from header Tailscale-User-Login"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newActorHandler(t, &fakeApp{entities: hammer()}, tc.configured)
			body := getPage(t, h, "/", tc.sent)

			assert.Contains(t, body, tc.wantText)
			assert.Contains(t, body, tc.wantTitle)
			assert.NotContains(t, body, "signed in")
		})
	}
}

func TestHeader_EscapesActor(t *testing.T) {
	h := newActorHandler(t, &fakeApp{entities: hammer()}, tsHeader)
	body := getPage(t, h, "/", map[string]string{tsHeader: `<script>alert(1)</script>`})

	assert.NotContains(t, body, "<script>alert(1)</script>")
	assert.Contains(t, body, "acting as &lt;script&gt;alert(1)&lt;/script&gt;")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `gotestsum -- -race -run 'TestHeader_' ./internal/web/...`
Expected: FAIL — body does not contain `acting as`.

- [ ] **Step 3: Add `actorView`**

Append to `internal/web/actor.go`:

```go
// actorView is the page header's "acting as" label. Source explains where
// Name came from so a header that fails to arrive is visible.
type actorView struct {
	Name   string
	Source string
}

func (s *Server) actorView(r *http.Request) actorView {
	name, fromHeader := s.resolveActor(r)
	header := http.CanonicalHeaderKey(s.cfg.ActorHeader)
	switch {
	case fromHeader:
		return actorView{Name: name, Source: "from header " + header}
	case header == "":
		return actorView{Name: name, Source: "no identity header configured"}
	default:
		return actorView{Name: name, Source: "header " + header + " not received"}
	}
}
```

- [ ] **Step 4: Thread it through every full-page data struct**

`internal/web/handlers_browse.go`:

```go
type indexData struct {
	Roots     []app.EntityResult
	Version   string
	GitCommit string
	Actor     actorView
}
```

```go
	data := indexData{
		Roots:     roots,
		Version:   versioncmd.Version,
		GitCommit: versioncmd.GitCommit,
		Actor:     s.actorView(r),
	}
```

and the search page:

```go
	s.renderHTML(w, "search_page", struct {
		Roots  []app.EntityResult
		Search any
		Actor  actorView
	}{Roots: roots, Search: searchData, Actor: s.actorView(r)})
```

`internal/web/handlers_entity.go`:

```go
type detailPageData struct {
	Detail detailData
	Roots  []app.EntityResult
	Actor  actorView
}
```

```go
	s.renderHTML(w, "detail", detailPageData{Detail: data, Roots: roots, Actor: s.actorView(r)})
```

- [ ] **Step 5: Templates**

In `index.html`, `detail.html` and `search_results.html` (`search_page` define), change `{{template "header" .}}` to:

```html
  {{template "header" .Actor}}
```

Confirm `partial_header.html` uses no other `.` fields: `rg -n '\{\{[^}]*\.' internal/web/assets/templates/partial_header.html` → no output before your edit.

In `partial_header.html`, replace the `app-actions` div with:

```html
  <div class="app-actions">
    <span class="app-actor" title="{{.Source}}">acting as {{.Name}}</span>
    <button class="btn-ghost"
            aria-label="Add root location"
            aria-haspopup="dialog"
            aria-controls="add-modal"
            hx-get="/entities/add"
            hx-target="#add-modal-body"
            hx-swap="innerHTML">+ Add root</button>
  </div>
```

- [ ] **Step 6: CSS**

In `internal/web/assets/static/app.css`, replace `.app-actions { margin-left: auto; }` with:

```css
.app-actions {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.app-actor {
  color: var(--color-text-muted);
  font-size: var(--font-size-sm);
}
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `gotestsum -- -race ./internal/web/...`
Expected: all PASS.

- [ ] **Step 8: Eyeball it**

Run: `mise run build`, then `./dist/wherehouse serve --db "$(mktemp -d)/w.db"` and open http://127.0.0.1:8080. Expected: muted `acting as webui` left of `+ Add root`, hover tooltip `no identity header configured`. Stop with Ctrl-C.

- [ ] **Step 9: Lint, full test, commit**

```bash
mise run lint
mise run test
jj describe -m "feat(web): show acting-as actor in page header

Refs #267"
jj new
```

---

### Task 3: Startup warning on non-loopback bind

**Files:**
- Modify: `internal/web/server.go` (Config, `New`, `Run`), `internal/web/actor.go`
- Create: `internal/web/actor_internal_test.go`
- Test: `internal/web/server_test.go`

**Interfaces:**
- Consumes: `Config.ActorHeader` from Task 1
- Produces: `web.Config.ErrOutput io.Writer` (defaults to `os.Stderr`); `func isLoopbackBind(bind string) bool`; warning line on `ErrOutput` starting with `warning: trusting identity header`

- [ ] **Step 1: Write the failing tests**

Create `internal/web/actor_internal_test.go`:

```go
package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsLoopbackBind(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1":   true,
		"127.0.0.2":   true,
		"::1":         true,
		"localhost":   true,
		"LOCALHOST":   true,
		"0.0.0.0":     false,
		"::":          false,
		"192.168.1.5": false,
		"wherehouse":  false,
	}
	for bind, want := range cases {
		t.Run(bind, func(t *testing.T) {
			assert.Equal(t, want, isLoopbackBind(bind))
		})
	}
}
```

Append to `internal/web/server_test.go` (add `"bytes"` import):

```go
func TestRun_ActorHeaderBindWarning(t *testing.T) {
	cases := []struct {
		name   string
		bind   string
		header string
		warn   bool
	}{
		{"header on all interfaces", "0.0.0.0", "Tailscale-User-Login", true},
		{"header on loopback", "127.0.0.1", "Tailscale-User-Login", false},
		{"no header on all interfaces", "0.0.0.0", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer
			srv, err := web.New(web.Config{
				App:         &fakeApp{},
				Bind:        tc.bind,
				Port:        0,
				Output:      io.Discard,
				ErrOutput:   &stderr,
				ActorHeader: tc.header,
			})
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			require.NoError(t, srv.Run(ctx))

			if tc.warn {
				assert.Contains(t, stderr.String(), "warning: trusting identity header Tailscale-User-Login")
				assert.Contains(t, stderr.String(), tc.bind)
				return
			}
			assert.Empty(t, stderr.String())
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `gotestsum -- -race -run 'TestIsLoopbackBind|TestRun_ActorHeaderBindWarning' ./internal/web/...`
Expected: compile failure — `undefined: isLoopbackBind` / `unknown field ErrOutput`.

- [ ] **Step 3: Implement**

Append to `internal/web/actor.go` (add `"net"` import):

```go
// isLoopbackBind reports whether bind only accepts connections from this host.
func isLoopbackBind(bind string) bool {
	if strings.EqualFold(bind, "localhost") {
		return true
	}
	ip := net.ParseIP(bind)
	return ip != nil && ip.IsLoopback()
}
```

In `internal/web/server.go`, add to `Config` (after `Output`):

```go
	ErrOutput io.Writer // destination for startup warnings; defaults to os.Stderr
```

In `New`, next to the `Output` default:

```go
	if cfg.ErrOutput == nil {
		cfg.ErrOutput = os.Stderr
	}
```

In `Run`, before the `fmt.Fprintln(s.cfg.Output, "Listening on", addr)` line:

```go
	if s.cfg.ActorHeader != "" && !isLoopbackBind(s.cfg.Bind) {
		header := http.CanonicalHeaderKey(s.cfg.ActorHeader)
		fmt.Fprintf(s.cfg.ErrOutput,
			"warning: trusting identity header %s on non-loopback address %s; "+
				"any client that can reach this address directly can forge it\n",
			header, s.cfg.Bind)
		s.cfg.Logger.Warn("actor header trusted on non-loopback bind",
			"header", header, "bind", s.cfg.Bind)
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `gotestsum -- -race ./internal/web/...`
Expected: all PASS.

- [ ] **Step 5: Lint, full test, commit**

```bash
mise run lint
mise run test
jj describe -m "feat(web): warn when actor header is trusted on non-loopback bind

Refs #267"
jj new
```

---

### Task 4: `[web] actor_header` config key + env var

**Files:**
- Modify: `internal/config/config.go:26-31` (+ new struct), `internal/config/loader.go:87-96`, `internal/config/writer.go:53-61,133-156,161-205`, `nix/home-manager.nix`
- Test: `internal/config/loader_test.go`, `internal/config/writer_test.go`

**Interfaces:**
- Produces: `config.WebConfig{ActorHeader string}`; `config.Config.Web WebConfig`; key `web.actor_header`; env `WHEREHOUSE_WEB_ACTOR_HEADER`

- [ ] **Step 1: Write the failing tests**

`internal/config/loader_test.go`, in `TestLoadConfig_EnvWithoutConfigFile` add:

```go
	t.Setenv("WHEREHOUSE_WEB_ACTOR_HEADER", "Tailscale-User-Login")
```

and:

```go
	assert.Equal(t, "Tailscale-User-Login", cfg.Web.ActorHeader)
```

`internal/config/writer_test.go`:
- In `TestWriteDefault_AllDefaultsRoundTrip`, after the `output.quiet` assert:

```go
	assert.Equal(t, defaults.Web.ActorHeader, v.GetString("web.actor_header"))
```

- In the `TestSet_UpdatesValue` cases table, add:

```go
		{
			name:  "web.actor_header",
			key:   "web.actor_header",
			value: "Tailscale-User-Login",
			want:  "Tailscale-User-Login",
		},
```

- In the `TestGetValue_AllKeys` cases table, add:

```go
		{
			name: "web.actor_header",
			key:  "web.actor_header",
			setupCfg: func(cfg *Config) {
				cfg.Web.ActorHeader = "Tailscale-User-Login"
			},
			want: "Tailscale-User-Login",
		},
```

(Read each test's loop body first to confirm the case struct fields match those shown; adjust field names only if they differ.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `gotestsum -- -race ./internal/config/...`
Expected: compile failure — `cfg.Web undefined`.

- [ ] **Step 3: Implement**

`internal/config/config.go`:

```go
type Config struct {
	Database DatabaseConfig `mapstructure:"database" toml:"database"`
	Logging  LoggingConfig  `mapstructure:"logging"  toml:"logging"`
	User     UserConfig     `mapstructure:"user"     toml:"user"`
	Output   OutputConfig   `mapstructure:"output"   toml:"output"`
	Web      WebConfig      `mapstructure:"web"      toml:"web"`
}
```

After `OutputConfig`:

```go
// WebConfig holds web server configuration.
type WebConfig struct {
	// ActorHeader names a request header, set by a trusted reverse proxy,
	// whose value is recorded as the event actor. Empty disables it.
	// See ADR 0031.
	ActorHeader string `mapstructure:"actor_header" toml:"actor_header"`
}
```

`internal/config/loader.go` — append `"web.actor_header",` to `envBoundKeys`.

`internal/config/writer.go`:
- In `WriteDefault` after the `output.quiet` default: `v.SetDefault("web.actor_header", defaults.Web.ActorHeader)`
- In `GetValue` before `default:`:

```go
	case "web.actor_header":
		return cfg.Web.ActorHeader, nil
```

- In `parseConfigValue` before `default:` (name validity is checked by `web.New` at startup — config must not import `internal/web`):

```go
	case "web.actor_header":
		return value, nil
```

`nix/home-manager.nix`:
- After `outputSettings`:

```nix
  # Build the [web] section, omitting null values.
  webSettings = lib.filterAttrs (_: v: v != null) {
    actor_header = cfg.settings.web.actorHeader;
  };
```

- Extend `settingsToml` (move the trailing `;` from the output line):

```nix
    // lib.optionalAttrs (outputSettings != {}) {output = outputSettings;}
    // lib.optionalAttrs (webSettings != {}) {web = webSettings;};
```

- After the `output = { … };` option block, inside `settings`:

```nix
      web = {
        actorHeader = lib.mkOption {
          type = lib.types.nullOr lib.types.str;
          default = null;
          description = ''
            Request header set by a trusted reverse proxy whose value is
            recorded as the actor for web UI changes (e.g. Tailscale-User-Login).
            Only safe when the proxy overwrites the header and is the sole path
            to the listener. When null, web changes are recorded as "webui".
          '';
          example = "Tailscale-User-Login";
        };
      };
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `gotestsum -- -race ./internal/config/...`
Expected: all PASS.

Run: `nix flake check` (if it is slow or unavailable, at minimum `nix-instantiate --parse nix/home-manager.nix`).
Expected: no errors.

- [ ] **Step 5: Lint, full test, commit**

```bash
mise run lint
mise run test
jj describe -m "feat(config): add web.actor_header setting

Refs #267"
jj new
```

---

### Task 5: `serve --actor-header` flag, precedence, help text

**Files:**
- Modify: `cmd/serve/serve.go`
- Test: `cmd/serve/serve_test.go`

**Interfaces:**
- Consumes: `web.Config.ActorHeader`, `web.Config.ErrOutput` (Tasks 1, 3); `config.Config.Web.ActorHeader` (Task 4); `cli.GetConfig(ctx)`; `config.ConfigKey`
- Produces: flag `--actor-header` (string, default `""`); precedence flag > config (env/file) > off

- [ ] **Step 1: Write the failing tests**

Append to `cmd/serve/serve_test.go` (add imports `"bytes"`, `"io"`, `"github.com/asphaltbuffet/wherehouse/internal/config"`):

```go
func TestBuildServeCmd_ActorHeaderDefault(t *testing.T) {
	cmd := serve.NewServeCmd(&fakeApp{})

	h, err := cmd.Flags().GetString("actor-header")
	require.NoError(t, err)
	assert.Empty(t, h)
}

// runServe executes serve on 0.0.0.0 with an already-cancelled context so it
// starts and shuts down immediately. The non-loopback warning on stderr
// reveals which actor header won.
func runServe(t *testing.T, cfgHeader string, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if cfgHeader != "" {
		cfg := config.GetDefaults()
		cfg.Web.ActorHeader = cfgHeader
		ctx = context.WithValue(ctx, config.ConfigKey, cfg)
	}

	var stderr bytes.Buffer
	cmd := serve.NewServeCmd(&fakeApp{})
	cmd.SetOut(io.Discard)
	cmd.SetErr(&stderr)
	cmd.SetArgs(append([]string{"--bind", "0.0.0.0", "--port", "0"}, args...))
	err := cmd.ExecuteContext(ctx)
	return stderr.String(), err
}

func TestServe_ActorHeaderPrecedence(t *testing.T) {
	cases := []struct {
		name      string
		cfgHeader string
		args      []string
		want      string // header named in the warning; "" means no warning
	}{
		{"neither", "", nil, ""},
		{"config only", "X-Config-User", nil, "X-Config-User"},
		{"flag only", "", []string{"--actor-header", "Tailscale-User-Login"}, "Tailscale-User-Login"},
		{"flag beats config", "X-Config-User", []string{"--actor-header", "Tailscale-User-Login"}, "Tailscale-User-Login"},
		{"empty flag disables config", "X-Config-User", []string{"--actor-header", ""}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stderr, err := runServe(t, tc.cfgHeader, tc.args...)
			require.NoError(t, err)
			if tc.want == "" {
				assert.NotContains(t, stderr, "warning: trusting identity header")
				return
			}
			assert.Contains(t, stderr, "warning: trusting identity header "+tc.want)
		})
	}
}

func TestServe_InvalidActorHeader(t *testing.T) {
	_, err := runServe(t, "", "--actor-header", "Tailscale User")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid actor header name")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `gotestsum -- -race ./cmd/serve/...`
Expected: FAIL — `flag accessed but not defined: actor-header` / `unknown flag: --actor-header`.

- [ ] **Step 3: Implement**

In `cmd/serve/serve.go`, replace `buildServeCmd`'s `Long` and flags:

```go
		Long: `Start a local HTTP server that renders the inventory as a navigable
tree in your browser. You can browse, search, add entities, edit names, and
toggle item status (active/missing) from the UI.

Identity: web changes are recorded as actor "webui". With --actor-header
(or web.actor_header / WHEREHOUSE_WEB_ACTOR_HEADER), the named request
header's value is recorded instead.

wherehouse does NOT authenticate requests. Only set an actor header behind
a reverse proxy that overwrites that header and is the sole path to this
listener (e.g. --bind 127.0.0.1 inside a Tailscale sidecar's network
namespace). Anyone who can reach the listener directly can forge it.

Examples:
  wherehouse serve                   # Listen on 127.0.0.1:8080
  wherehouse serve --port 9090
  wherehouse serve --bind 0.0.0.0   # Listen on all interfaces (LAN)
  wherehouse serve --actor-header Tailscale-User-Login`,
		Args: cobra.NoArgs,
	}
	cmd.Flags().IntP("port", "p", defaultPort, "port to listen on")
	cmd.Flags().String("bind", "127.0.0.1", "address to bind to")
	cmd.Flags().String("actor-header", "",
		"trusted proxy header recorded as the actor (overrides web.actor_header)")
	return cmd
```

In `runServe`, after reading `bind`:

```go
	actorHeader, err := resolveActorHeader(cmd)
	if err != nil {
		return err
	}

	srv, err := web.New(web.Config{
		App:         a,
		Bind:        bind,
		Port:        port,
		Output:      cmd.OutOrStdout(),
		ErrOutput:   cmd.ErrOrStderr(),
		ActorHeader: actorHeader,
	})
```

Add below `runServe`:

```go
// resolveActorHeader applies --actor-header > web.actor_header (config file
// or WHEREHOUSE_WEB_ACTOR_HEADER). An explicitly empty flag disables it.
func resolveActorHeader(cmd *cobra.Command) (string, error) {
	if cmd.Flags().Changed("actor-header") {
		h, err := cmd.Flags().GetString("actor-header")
		if err != nil {
			return "", fmt.Errorf("get actor-header flag: %w", err)
		}
		return h, nil
	}
	if cfg, ok := cli.GetConfig(cmd.Context()); ok {
		return cfg.Web.ActorHeader, nil
	}
	return "", nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `gotestsum -- -race ./cmd/serve/...`
Expected: all PASS.

Run: `rg -l '"net/http"|"html/template"|//go:embed' cmd/serve/`
Expected: no output.

- [ ] **Step 5: Manual smoke test**

```bash
mise run build
./dist/wherehouse serve --db /tmp/wh-smoke.db --bind 0.0.0.0 --actor-header Tailscale-User-Login
```

Expected stderr: `warning: trusting identity header Tailscale-User-Login on non-loopback address 0.0.0.0; …`, then `Listening on http://0.0.0.0:8080`. In another terminal:

```bash
curl -s -H 'Tailscale-User-Login: alice@example.com' http://127.0.0.1:8080/ | rg -o 'acting as [^<]*'
```

Expected: `acting as alice@example.com`. Ctrl-C the server.

- [ ] **Step 6: Lint, full test, commit**

```bash
mise run lint
mise run test
jj describe -m "feat(serve): add --actor-header flag

Refs #267"
jj new
```

---

### Task 6: README

**Files:**
- Modify: `README.md` (Container section ~L109-128, `serve` examples ~L287-292, command list ~L324, Config File sample, Environment Variables)

- [ ] **Step 1: Fix the Container identity bullet**

The current bullet is wrong: web edits have always been recorded as `webui`, and `user.default_identity` never applied to the web UI. Replace:

```markdown
- **Identity**: web edits are recorded as user `unknown` unless you set `-e WHEREHOUSE_USER_DEFAULT_IDENTITY=<name>`.
```

with:

```markdown
- **Identity**: web edits are recorded as actor `webui`. Behind an authenticating proxy, set `-e WHEREHOUSE_WEB_ACTOR_HEADER=<header>` to record the proxy's user instead — see [Behind Tailscale Serve](#behind-tailscale-serve).
```

- [ ] **Step 2: Add a Tailscale recipe after the Container bullets**

````markdown
#### Behind Tailscale Serve

wherehouse does no authentication of its own ([ADR 0031](docs/adr/0031-web-authentication-delegated-to-trusted-proxy.md)). Put it behind Tailscale Serve, which authenticates tailnet users and sets `Tailscale-User-Login`, and bind to loopback so Serve is the only way in:

```yaml
services:
  tailscale:
    image: tailscale/tailscale:latest
    hostname: wherehouse
    environment:
      - TS_AUTHKEY=${TS_AUTHKEY}
      - TS_STATE_DIR=/var/lib/tailscale
      - TS_SERVE_CONFIG=/config/serve.json
    volumes:
      - ts-state:/var/lib/tailscale
      - ./serve.json:/config/serve.json:ro
  wherehouse:
    image: ghcr.io/asphaltbuffet/wherehouse:latest
    network_mode: service:tailscale
    command: ["--bind", "127.0.0.1"]   # overrides the image's 0.0.0.0
    environment:
      - WHEREHOUSE_WEB_ACTOR_HEADER=Tailscale-User-Login
    volumes:
      - wherehouse:/data
volumes:
  ts-state:
  wherehouse:
```

`serve.json`:

```json
{
  "TCP": { "443": { "HTTPS": true } },
  "Web": { "${TS_CERT_DOMAIN}:443": { "Handlers": { "/": { "Proxy": "http://127.0.0.1:8080" } } } }
}
```

Without `--bind 127.0.0.1`, the listener is reachable on the node's tailnet IP and any peer can bypass Serve and forge the header; `serve` prints a warning at startup in that case. The page header shows `acting as …` so you can confirm the identity arrives.
````

- [ ] **Step 3: Update the other references**

- `serve` examples block: add `wherehouse serve --actor-header Tailscale-User-Login   # trust a proxy identity header`
- Command list: `serve                Start local web server (--port, --bind, --actor-header)`
- Config File sample: append

```toml

[web]
actor_header = ""
```

- Environment Variables block: add `export WHEREHOUSE_WEB_ACTOR_HEADER="Tailscale-User-Login"`

- [ ] **Step 4: Verify the config sample matches reality**

Run: `./dist/wherehouse config init --config /tmp/wh-init.toml` (add `--force` if it exists), then `rg -n -A1 '\[web\]' /tmp/wh-init.toml`
Expected: `[web]` followed by `actor_header = ''` (or `""`). Match README quoting to whatever the existing sample uses.

- [ ] **Step 5: Commit**

```bash
jj describe -m "docs: document web actor header and Tailscale Serve setup

Closes #267
Closes #145"
jj new
```

---

### Task 7: Finish the branch

- [ ] **Step 1:** `mise run lint`, then `mise run test` — both clean.
- [ ] **Step 2:** `jj log --no-graph -r 'trunk()..@'` — expect seven described commits (Task 0–6) plus an empty `@`.
- [ ] **Step 3:** Use superpowers:finishing-a-development-branch. The PR body must contain `Closes #267` and `Closes #145`, link ADR 0031, and mention follow-up #271.
