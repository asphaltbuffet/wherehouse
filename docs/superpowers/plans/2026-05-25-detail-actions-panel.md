# Detail Actions Panel Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an "Actions" aside panel to the right of the entity metadata `<dl>` on the detail page, containing "Add child" and "Mark missing/Mark found" buttons, and remove the inline status Edit button from the metadata field.

**Architecture:** The boolean flags (`CanAddChild`, `CanMarkMissing`, `IsMissing`) are computed server-side in `buildDetailData` and passed into the existing `detailData` struct — Go templates cannot compare typed iota constants, so booleans are the right tool. A new dedicated POST handler (`handleToggleMissing`) performs the missing/found toggle without needing any form body: the server derives the target state from the current entity status. The `edit/status` form route and handler remain (they are still used internally), but the trigger button is removed from the template.

**Tech Stack:** Go 1.25, `html/template`, HTMX, `testify/assert` + `testify/require`

---

## File Map

| File | Change |
|---|---|
| `internal/web/handlers.go` | Extend `detailData` struct; populate new fields in `buildDetailData`; add `handleToggleMissing` method |
| `internal/web/routes.go` | Register `POST /entities/{entityID}/actions/toggle-missing` |
| `internal/web/assets/templates/detail.html` | Wrap metadata `<dl>` + new `<aside>` in `.detail-top` flex container; remove inline status Edit button |
| `internal/web/assets/static/app.css` | Add `.detail-top`, `.detail-actions`, `.detail-actions-heading`, `.detail-action-btn` rules |
| `internal/web/handlers_test.go` | Add tests for `handleToggleMissing` (ok→missing, missing→ok, place forbidden); update `TestHandleEntityDetail_Found` assertions |

---

## Task 1: Extend `detailData` and populate it in `buildDetailData`

**Files:**
- Modify: `internal/web/handlers.go:66-73` (detailData struct)
- Modify: `internal/web/handlers.go:112-159` (buildDetailData return statement)

- [ ] **Step 1: Add three boolean fields to `detailData`**

In `internal/web/handlers.go`, find the `detailData` struct (currently ends around line 73) and add three fields after `StatusEditable`:

```go
detailData struct {
    Entity         app.EntityResult
    DateAdded      string
    History        []app.HistoryResult
    StatusEditable bool
    CanAddChild    bool // false when EntityType == EntityTypeLeaf
    CanMarkMissing bool // false when EntityType == EntityTypePlace
    IsMissing      bool // true when Status == EntityStatusMissing
    Breadcrumbs    []Breadcrumb
    Error          string // populated by edit POST handlers; rendered inline above the detail dl
}
```

- [ ] **Step 2: Populate the new fields in `buildDetailData`**

In the `return detailData{…}` block at the end of `buildDetailData`, add the three new fields:

```go
return detailData{
    Entity:         *entity,
    DateAdded:      dateAdded,
    History:        history,
    StatusEditable: editable,
    CanAddChild:    entity.EntityType != inventory.EntityTypeLeaf,
    CanMarkMissing: entity.EntityType != inventory.EntityTypePlace,
    IsMissing:      entity.Status == inventory.EntityStatusMissing,
    Breadcrumbs:    BreadcrumbsForEntity(entities, entity.FullPathDisplay),
}, nil
```

- [ ] **Step 3: Verify it compiles**

```bash
mise run build
```
Expected: no output (clean build).

---

## Task 2: Add `handleToggleMissing` handler and register the route

**Files:**
- Modify: `internal/web/handlers.go` (append new method after `handleSearch`)
- Modify: `internal/web/routes.go`

- [ ] **Step 1: Write failing tests for `handleToggleMissing`**

Append to `internal/web/handlers_test.go`:

```go
func TestHandleToggleMissing_OkToMissing(t *testing.T) {
    entities := []app.EntityResult{
        {EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Garage:Hammer",
            EntityType: inventory.EntityTypeLeaf, Status: inventory.EntityStatusOk},
    }
    ts := newTestServer(t, &fakeApp{entities: entities})
    defer ts.Close()

    req, _ := http.NewRequest(http.MethodPost, ts.URL+"/entities/abc/actions/toggle-missing", nil)
    req.Header.Set("Hx-Request", "true")
    resp, err := ts.Client().Do(req)
    require.NoError(t, err)
    defer resp.Body.Close()

    assert.Equal(t, http.StatusOK, resp.StatusCode)
    body, _ := io.ReadAll(resp.Body)
    assert.Contains(t, string(body), `id="entity-detail"`)
}

func TestHandleToggleMissing_MissingToOk(t *testing.T) {
    entities := []app.EntityResult{
        {EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Garage:Hammer",
            EntityType: inventory.EntityTypeLeaf, Status: inventory.EntityStatusMissing},
    }
    ts := newTestServer(t, &fakeApp{entities: entities})
    defer ts.Close()

    req, _ := http.NewRequest(http.MethodPost, ts.URL+"/entities/abc/actions/toggle-missing", nil)
    resp, err := ts.Client().Do(req)
    require.NoError(t, err)
    defer resp.Body.Close()

    assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestHandleToggleMissing_PlaceForbidden(t *testing.T) {
    entities := []app.EntityResult{
        {EntityID: "abc", DisplayName: "Garage", FullPathDisplay: "Garage",
            EntityType: inventory.EntityTypePlace, Status: inventory.EntityStatusOk},
    }
    ts := newTestServer(t, &fakeApp{entities: entities})
    defer ts.Close()

    req, _ := http.NewRequest(http.MethodPost, ts.URL+"/entities/abc/actions/toggle-missing", nil)
    resp, err := ts.Client().Do(req)
    require.NoError(t, err)
    defer resp.Body.Close()

    assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestHandleToggleMissing_NotFound(t *testing.T) {
    ts := newTestServer(t, &fakeApp{entities: []app.EntityResult{}})
    defer ts.Close()

    req, _ := http.NewRequest(http.MethodPost, ts.URL+"/entities/missing/actions/toggle-missing", nil)
    resp, err := ts.Client().Do(req)
    require.NoError(t, err)
    defer resp.Body.Close()

    assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
gotestsum -- -race ./internal/web/...
```
Expected: 4 new tests fail with `404 Not Found` (route not registered yet).

- [ ] **Step 3: Add the `handleToggleMissing` method to `handlers.go`**

Append after `handleSearch` (the last method in the file):

```go
func (s *Server) handleToggleMissing(w http.ResponseWriter, r *http.Request) {
    entityID := r.PathValue("entityID")

    data, err := s.buildDetailData(r.Context(), entityID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    if data.Entity.EntityID == "" {
        http.Error(w, "entity not found", http.StatusNotFound)
        return
    }
    if !data.CanMarkMissing {
        http.Error(w, "cannot mark place as missing", http.StatusForbidden)
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
        data.Error = fmt.Sprintf("status change failed: %v", err)
        s.renderDetailSection(w, r, data)
        return
    }

    fresh, err := s.buildDetailData(r.Context(), entityID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    s.renderDetailSection(w, r, fresh)
    if r.Header.Get("Hx-Request") == htmxHeaderVal {
        _ = s.templates.ExecuteTemplate(w, "tree_badge_oob", fresh.Entity)
    }
}
```

- [ ] **Step 4: Register the route in `routes.go`**

Add before the `GET /search` line:

```go
mux.HandleFunc("POST /entities/{entityID}/actions/toggle-missing", s.handleToggleMissing)
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
gotestsum -- -race ./internal/web/...
```
Expected: all tests pass including the 4 new ones.

- [ ] **Step 6: Commit**

```bash
jj describe -m "feat(webui): add handleToggleMissing — POST /entities/{id}/actions/toggle-missing"
jj new
```

---

## Task 3: Update `detail.html` template — Actions aside + remove status Edit button

**Files:**
- Modify: `internal/web/assets/templates/detail.html`

- [ ] **Step 1: Replace the `detail_section` template body**

Replace the entire `{{define "detail_section"}}…{{end}}` block (lines 1–48) with the following. Key changes: (a) wrap `<dl>` and new `<aside>` in `<div class="detail-top">`, (b) remove the conditional Edit button from the Status `<dd>`, (c) add the Actions `<aside>`:

```html
{{define "detail_section"}}
<section id="entity-detail">
  {{template "breadcrumbs" .Breadcrumbs}}
  <h2>{{.Entity.DisplayName}}</h2>
  {{if .Error}}<div class="error" role="alert">{{.Error}}</div>{{end}}
  <div class="detail-top">
    <dl class="detail-kv">
      <dt>Name</dt>
      <dd>
        {{.Entity.DisplayName}}
        <button class="btn-link"
                hx-get="/entities/{{.Entity.EntityID}}/edit/name"
                hx-target="closest dd" hx-swap="outerHTML">Edit</button>
      </dd>
      <dt>Type</dt>      <dd>{{.Entity.EntityType}}</dd>
      <dt>Path</dt>      <dd>{{.Entity.FullPathDisplay}}</dd>
      <dt>Status</dt>    <dd>{{.Entity.Status}}{{if .Entity.StatusContext}} — {{.Entity.StatusContext}}{{end}}</dd>
      <dt>Date added</dt><dd class="mono">{{.DateAdded}}</dd>
    </dl>
    <aside class="detail-actions" aria-label="Actions">
      <h3 class="detail-actions-heading">Actions</h3>
      {{if .CanAddChild}}
      <button class="btn-ghost detail-action-btn"
              onclick="htmx.ajax('GET','/entities/{{.Entity.EntityID}}/add',{target:'#add-modal-body',swap:'innerHTML'});document.getElementById('add-modal').showModal()">
        Add child
      </button>
      {{else}}
      <button class="btn-ghost detail-action-btn" disabled aria-disabled="true"
              title="Leaf items cannot have children">
        Add child
      </button>
      {{end}}
      {{if .CanMarkMissing}}
      <button class="btn-ghost detail-action-btn"
              hx-post="/entities/{{.Entity.EntityID}}/actions/toggle-missing"
              hx-target="#entity-detail" hx-swap="outerHTML">
        {{if .IsMissing}}Mark found{{else}}Mark missing{{end}}
      </button>
      {{else}}
      <button class="btn-ghost detail-action-btn" disabled aria-disabled="true"
              title="Places cannot be marked missing">
        Mark missing
      </button>
      {{end}}
    </aside>
  </div>

  {{if .History}}
  <h3>History</h3>
  <table class="history-table">
    <thead><tr><th>When</th><th>Event</th><th>By</th><th>Note</th></tr></thead>
    <tbody>
    {{range .History}}
      <tr>
        <td class="mono">{{.TimestampUTC}}</td>
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

- [ ] **Step 2: Verify tests still pass (template renders correctly)**

```bash
gotestsum -- -race ./internal/web/...
```
Expected: all tests pass.

- [ ] **Step 3: Update `TestHandleEntityDetail_Found` to assert Actions panel presence**

In `handlers_test.go`, find `TestHandleEntityDetail_Found` and add assertions for the actions panel. The entity in that test is `EntityTypeLeaf` so "Add child" should be disabled; it is `EntityStatusOk` and not a place, so "Mark missing" should be enabled:

```go
// Add these assertions after the existing ones:
assert.Contains(t, bs, `class="detail-actions"`)
assert.Contains(t, bs, `Mark missing`)
// Add child is disabled for leaf:
assert.Contains(t, bs, `title="Leaf items cannot have children"`)
// Status field has no Edit button:
assert.NotContains(t, bs, `hx-get="/entities/xyz/edit/status"`)
```

- [ ] **Step 4: Run tests**

```bash
gotestsum -- -race ./internal/web/...
```
Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
jj describe -m "feat(webui): actions panel on detail page — Add child, Mark missing/found"
jj new
```

---

## Task 4: Add CSS for the actions layout

**Files:**
- Modify: `internal/web/assets/static/app.css`

- [ ] **Step 1: Insert new rules after the `.detail-kv` block**

Find the three existing `.detail-kv` lines (around line 381) and insert the new rules immediately after them:

```css
.detail-top { display: flex; gap: var(--space-6); align-items: flex-start; }
.detail-kv { display: grid; grid-template-columns: 140px 1fr; row-gap: var(--space-2); }
.detail-kv dt { font-weight: 600; color: var(--color-text-muted); }
.detail-kv dd { margin: 0; }
.detail-actions {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  min-width: 140px;
}
.detail-actions-heading {
  margin: 0 0 var(--space-1);
  font-size: var(--font-size-sm);
  font-weight: 600;
  color: var(--color-text-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.detail-action-btn { width: 100%; text-align: left; }
.detail-action-btn:disabled { opacity: 0.4; cursor: default; }
```

- [ ] **Step 2: Build and lint**

```bash
mise run build
mise run lint
```
Expected: `0 issues`.

- [ ] **Step 3: Run full test suite**

```bash
mise run test
```
Expected: all tests pass, no race conditions.

- [ ] **Step 4: Commit**

```bash
jj describe -m "feat(webui): CSS for detail actions panel layout"
jj new
```

---

## Self-Review

**Spec coverage:**
- ✅ Actions section to the right of metadata — `.detail-top` flex row with `<aside class="detail-actions">`
- ✅ "Add child" disabled for `EntityTypeLeaf` — `CanAddChild = entity.EntityType != inventory.EntityTypeLeaf`
- ✅ "Mark missing" disabled for `EntityTypePlace` — `CanMarkMissing = entity.EntityType != inventory.EntityTypePlace`
- ✅ "Mark missing" shows "Mark found" when status is already `missing` — `IsMissing` flag + `{{if .IsMissing}}Mark found{{else}}Mark missing{{end}}`
- ✅ Remove inline status Edit button from metadata — Status `<dd>` simplified to read-only text
- ✅ Actions section above History — the aside is inside `.detail-top` which sits before `{{if .History}}`

**Placeholder scan:** No TBDs, TODOs, or vague steps found.

**Type consistency:**
- `CanAddChild`, `CanMarkMissing`, `IsMissing` defined in Task 1, referenced identically in Task 3 template.
- `handleToggleMissing` defined in Task 2, route registered in Task 2, no forward references.
- `inventory.EntityTypeLeaf`, `inventory.EntityTypePlace`, `inventory.EntityStatusMissing`, `inventory.EntityStatusOk` — all valid constants from `internal/inventory/entity_type.go` and `entity_status.go`.
