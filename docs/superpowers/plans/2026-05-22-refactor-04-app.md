# Refactor 04: `internal/app` Package Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create `internal/app` as the single entry point for all callers (CLI and TUI), owning input shaping, use-case orchestration, and result types that neither caller needs to construct themselves.

**Architecture:** `app` imports `eventbus` and `store`, exposes named operations (`CreateEntity`, `MoveEntity`, `GetHistory`, etc.) that accept typed request structs and return typed result structs. `cmd/` and `tui/` import only `app` — they never touch `store` or `eventbus` directly. This plan implements the operations needed to replace the existing `cmd/` commands; new commands added in future will extend `app` first.

**Tech Stack:** Go 1.25, `internal/inventory` (plan 01), `internal/store` (plan 02), `internal/eventbus` (plan 03), `internal/entitypath` (existing — unchanged). All three prior plans must be complete.

**Prerequisites:** Plans 01, 02, and 03 complete.

---

## Target File Map

```
internal/app/
  doc.go          # package doc
  app.go          # App struct, New
  entities.go     # CreateEntity, RenameEntity, ReparentEntity, RemoveEntity, GetEntity, ListEntities
  status.go       # ChangeEntityStatus (missing, found, borrowed, loaned, returned)
  history.go      # GetHistory, GetHistoryByID
  search.go       # FindEntities (name search with ranking)
  requests.go     # all request structs
  results.go      # all result structs
```

---

### Task 1: `App` scaffold and request/result types

**Files:**
- Create: `internal/app/doc.go`
- Create: `internal/app/app.go`
- Create: `internal/app/requests.go`
- Create: `internal/app/results.go`

- [ ] **Step 1: Create `doc.go`**

```go
// Package app provides the use-case entry points for wherehouse.
// It is the single API consumed by cmd/ and tui/ — neither layer
// imports store or eventbus directly.
package app
```

- [ ] **Step 2: Create `app.go`**

```go
package app

import (
	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// App is the top-level application handle.
// Create one per process via New and share it across all callers.
type App struct {
	store *store.Store
	bus   *eventbus.Bus
}

// New creates an App from an open store.
func New(s *store.Store) *App {
	return &App{
		store: s,
		bus:   eventbus.New(s),
	}
}
```

- [ ] **Step 3: Create `requests.go`**

```go
package app

import "github.com/asphaltbuffet/wherehouse/internal/inventory"

// CreateEntityRequest specifies the parameters for creating a new entity.
type CreateEntityRequest struct {
	DisplayName string
	EntityType  inventory.EntityType
	// ParentPath is a colon-separated path string, e.g. "Garage:Toolbox".
	// Empty means root-level.
	ParentPath string
	ActorID    string
	Note       string
}

// RenameEntityRequest specifies a rename operation.
type RenameEntityRequest struct {
	// EntityPath is the current colon-separated path to the entity.
	EntityPath string
	NewName    string
	ActorID    string
	Note       string
}

// ReparentEntityRequest moves an entity to a new parent.
type ReparentEntityRequest struct {
	EntityPath    string
	NewParentPath string // empty means make root-level
	ActorID       string
	Note          string
}

// RemoveEntityRequest permanently removes an entity.
type RemoveEntityRequest struct {
	EntityPath string
	ActorID    string
	Note       string
}

// ChangeStatusRequest changes an entity's lifecycle status.
type ChangeStatusRequest struct {
	EntityPath    string
	Status        inventory.EntityStatus
	StatusContext string // optional human-readable context
	ActorID       string
	Note          string
}

// GetHistoryRequest retrieves event history for an entity.
type GetHistoryRequest struct {
	// EntityPath or EntityID — one must be set.
	EntityPath string
	EntityID   string
	// Limit 0 means no limit.
	Limit       int
	OldestFirst bool
}

// FindEntitiesRequest searches for entities by name.
type FindEntitiesRequest struct {
	Query    string
	Limit    int
	ActorID  string
}
```

- [ ] **Step 4: Create `results.go`**

```go
package app

import "github.com/asphaltbuffet/wherehouse/internal/inventory"

// EntityResult is the app-layer view of an entity returned to callers.
type EntityResult struct {
	EntityID          string
	DisplayName       string
	CanonicalName     string
	EntityType        inventory.EntityType
	FullPathDisplay   string
	Status            inventory.EntityStatus
	StatusContext     string
}

// HistoryResult is the app-layer view of an event returned to callers.
type HistoryResult struct {
	EventID      int64
	EventType    inventory.EventType
	TimestampUTC string
	ActorUserID  string
	// Payload is the raw JSON — callers that need details can unmarshal it.
	Payload []byte
	Note    string
}

// FindResult is one match returned by FindEntities.
type FindResult struct {
	Entity   EntityResult
	Distance int // Levenshtein distance from query; 0 = exact match
}

func entityToResult(e *inventory.Entity) EntityResult {
	ctx := ""
	if e.StatusContext != nil {
		ctx = *e.StatusContext
	}
	return EntityResult{
		EntityID:      e.EntityID,
		DisplayName:   e.DisplayName,
		CanonicalName: e.CanonicalName,
		EntityType:    e.EntityType,
		FullPathDisplay: e.FullPathDisplay,
		Status:        e.Status,
		StatusContext:  ctx,
	}
}
```

- [ ] **Step 5: Verify it compiles**

```bash
go build ./internal/app/...
```

Expected: success (no tests yet, just a compilation check).

- [ ] **Step 6: Commit**

```bash
jj new -m "feat(app): scaffold App, request and result types"
```

---

### Task 2: Entity creation and lookup

**Files:**
- Create: `internal/app/entities.go`
- Test: `internal/app/entities_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/app/entities_test.go
package app_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestApp(t *testing.T) *app.App {
	t.Helper()
	s, err := store.Open(store.Config{
		Path:        filepath.Join(t.TempDir(), "test.db"),
		AutoMigrate: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return app.New(s)
}

func TestCreateEntity_RootPlace(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	result, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage",
		EntityType:  inventory.EntityTypePlace,
		ActorID:     "alice",
	})
	require.NoError(t, err)
	assert.Equal(t, "Garage", result.DisplayName)
	assert.Equal(t, "Garage", result.FullPathDisplay)
	assert.Equal(t, inventory.EntityTypePlace, result.EntityType)
}

func TestCreateEntity_NestedPath(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage",
		EntityType:  inventory.EntityTypePlace,
		ActorID:     "alice",
	})
	require.NoError(t, err)

	result, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Toolbox",
		EntityType:  inventory.EntityTypeContainer,
		ParentPath:  "Garage",
		ActorID:     "alice",
	})
	require.NoError(t, err)
	assert.Equal(t, "Garage:Toolbox", result.FullPathDisplay)
}

func TestCreateEntity_PlaceInNonPlace_Error(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage",
		EntityType:  inventory.EntityTypePlace,
		ActorID:     "alice",
	})
	require.NoError(t, err)

	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Wrench",
		EntityType:  inventory.EntityTypeLeaf,
		ParentPath:  "Garage",
		ActorID:     "alice",
	})
	require.NoError(t, err)

	// Creating a place inside a leaf should fail.
	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Zone",
		EntityType:  inventory.EntityTypePlace,
		ParentPath:  "Garage:Wrench",
		ActorID:     "alice",
	})
	assert.Error(t, err)
}

func TestGetEntity_ByPath(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage",
		EntityType:  inventory.EntityTypePlace,
		ActorID:     "alice",
	})
	require.NoError(t, err)

	result, err := a.GetEntityByPath(ctx, "Garage")
	require.NoError(t, err)
	assert.Equal(t, "Garage", result.DisplayName)
}

func TestListEntities(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	for _, name := range []string{"Garage", "Basement", "Kitchen"} {
		_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: name,
			EntityType:  inventory.EntityTypePlace,
			ActorID:     "alice",
		})
		require.NoError(t, err)
	}

	results, err := a.ListEntities(ctx)
	require.NoError(t, err)
	assert.Len(t, results, 3)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
gotestsum -- ./internal/app/...
```

Expected: compilation error — CreateEntity undefined.

- [ ] **Step 3: Create `entities.go`**

```go
package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/nanoid"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// CreateEntity creates a new entity, resolving ParentPath to a parent entity ID if provided.
func (a *App) CreateEntity(ctx context.Context, req CreateEntityRequest) (EntityResult, error) {
	var parentID *string

	if req.ParentPath != "" {
		parent, err := a.resolveEntityPath(ctx, req.ParentPath)
		if err != nil {
			return EntityResult{}, fmt.Errorf("resolve parent path %q: %w", req.ParentPath, err)
		}
		parentID = &parent.EntityID
	}

	entityID := nanoid.New()
	payload := eventbus.EntityCreatedPayload{
		EntityID:    entityID,
		DisplayName: req.DisplayName,
		EntityType:  req.EntityType.String(),
		ParentID:    parentID,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return EntityResult{}, fmt.Errorf("marshal payload: %w", err)
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	if _, err := a.bus.Dispatch(ctx, inventory.EntityCreatedEvent, req.ActorID, raw, note); err != nil {
		return EntityResult{}, fmt.Errorf("create entity: %w", err)
	}

	entity, err := a.store.GetEntity(ctx, entityID)
	if err != nil {
		return EntityResult{}, fmt.Errorf("get created entity: %w", err)
	}

	return entityToResult(entity), nil
}

// RenameEntity renames an entity, resolved by its current path.
func (a *App) RenameEntity(ctx context.Context, req RenameEntityRequest) (EntityResult, error) {
	entity, err := a.resolveEntityPath(ctx, req.EntityPath)
	if err != nil {
		return EntityResult{}, fmt.Errorf("resolve path %q: %w", req.EntityPath, err)
	}

	payload := eventbus.EntityRenamedPayload{
		EntityID:    entity.EntityID,
		DisplayName: req.NewName,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return EntityResult{}, fmt.Errorf("marshal payload: %w", err)
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	if _, err := a.bus.Dispatch(ctx, inventory.EntityRenamedEvent, req.ActorID, raw, note); err != nil {
		return EntityResult{}, fmt.Errorf("rename entity: %w", err)
	}

	updated, err := a.store.GetEntity(ctx, entity.EntityID)
	if err != nil {
		return EntityResult{}, fmt.Errorf("get renamed entity: %w", err)
	}

	return entityToResult(updated), nil
}

// ReparentEntity moves an entity to a new parent, resolved by paths.
func (a *App) ReparentEntity(ctx context.Context, req ReparentEntityRequest) (EntityResult, error) {
	entity, err := a.resolveEntityPath(ctx, req.EntityPath)
	if err != nil {
		return EntityResult{}, fmt.Errorf("resolve entity path %q: %w", req.EntityPath, err)
	}

	var newParentID *string
	if req.NewParentPath != "" {
		parent, err := a.resolveEntityPath(ctx, req.NewParentPath)
		if err != nil {
			return EntityResult{}, fmt.Errorf("resolve new parent path %q: %w", req.NewParentPath, err)
		}
		newParentID = &parent.EntityID
	}

	payload := eventbus.EntityReparentedPayload{
		EntityID:    entity.EntityID,
		NewParentID: newParentID,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return EntityResult{}, fmt.Errorf("marshal payload: %w", err)
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	if _, err := a.bus.Dispatch(ctx, inventory.EntityReparentedEvent, req.ActorID, raw, note); err != nil {
		return EntityResult{}, fmt.Errorf("reparent entity: %w", err)
	}

	updated, err := a.store.GetEntity(ctx, entity.EntityID)
	if err != nil {
		return EntityResult{}, fmt.Errorf("get reparented entity: %w", err)
	}

	return entityToResult(updated), nil
}

// RemoveEntity permanently marks an entity as removed.
func (a *App) RemoveEntity(ctx context.Context, req RemoveEntityRequest) error {
	entity, err := a.resolveEntityPath(ctx, req.EntityPath)
	if err != nil {
		return fmt.Errorf("resolve path %q: %w", req.EntityPath, err)
	}

	payload := eventbus.EntityRemovedPayload{EntityID: entity.EntityID}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	if _, err := a.bus.Dispatch(ctx, inventory.EntityRemovedEvent, req.ActorID, raw, note); err != nil {
		return fmt.Errorf("remove entity: %w", err)
	}

	return nil
}

// GetEntityByPath retrieves an entity by its colon-separated display path.
func (a *App) GetEntityByPath(ctx context.Context, path string) (EntityResult, error) {
	entity, err := a.resolveEntityPath(ctx, path)
	if err != nil {
		return EntityResult{}, err
	}
	return entityToResult(entity), nil
}

// ListEntities returns all non-removed entities.
func (a *App) ListEntities(ctx context.Context) ([]EntityResult, error) {
	entities, err := a.store.ListEntities(ctx)
	if err != nil {
		return nil, fmt.Errorf("list entities: %w", err)
	}

	results := make([]EntityResult, 0, len(entities))
	for _, e := range entities {
		if e.Status != inventory.EntityStatusRemoved {
			results = append(results, entityToResult(e))
		}
	}
	return results, nil
}

// resolveEntityPath looks up an entity by its colon-separated display path.
// Returns store.ErrNotFound if no match exists.
func (a *App) resolveEntityPath(ctx context.Context, path string) (*inventory.Entity, error) {
	segments, err := entitypath.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("parse path %q: %w", path, err)
	}

	canonical := inventory.CanonicalizeString(segments[len(segments)-1])
	candidates, err := a.store.GetEntitiesByCanonicalName(ctx, canonical)
	if err != nil {
		return nil, fmt.Errorf("lookup %q: %w", canonical, err)
	}

	canonicalPath := entitypath.Canonicalize(segments, inventory.CanonicalizeString)
	for _, e := range candidates {
		if e.FullPathCanonical == canonicalPath {
			return e, nil
		}
	}

	return nil, fmt.Errorf("%w: %q", store.ErrNotFound, path)
}
```

Note: `resolveEntityPath` calls `entitypath.Parse` and `entitypath.Canonicalize`. Check what `internal/entitypath` actually exposes and adjust the call sites to match the real API — do not invent function names.

- [ ] **Step 4: Run tests**

```bash
gotestsum -- ./internal/app/...
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
jj new -m "feat(app): add entity CRUD operations"
```

---

### Task 3: Status, history, and search

**Files:**
- Create: `internal/app/status.go`
- Create: `internal/app/history.go`
- Create: `internal/app/search.go`
- Test: `internal/app/status_test.go`, `internal/app/history_test.go`, `internal/app/search_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/app/status_test.go
package app_test

import (
	"context"
	"testing"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChangeStatus_Missing(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage", EntityType: inventory.EntityTypePlace, ActorID: "alice",
	})
	require.NoError(t, err)

	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Wrench", EntityType: inventory.EntityTypeLeaf, ParentPath: "Garage", ActorID: "alice",
	})
	require.NoError(t, err)

	err = a.ChangeStatus(ctx, app.ChangeStatusRequest{
		EntityPath:    "Garage:Wrench",
		Status:        inventory.EntityStatusMissing,
		StatusContext: "lost at job site",
		ActorID:       "alice",
	})
	require.NoError(t, err)

	result, err := a.GetEntityByPath(ctx, "Garage:Wrench")
	require.NoError(t, err)
	assert.Equal(t, inventory.EntityStatusMissing, result.Status)
	assert.Equal(t, "lost at job site", result.StatusContext)
}
```

```go
// internal/app/history_test.go
package app_test

import (
	"context"
	"testing"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetHistory_ByPath(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage", EntityType: inventory.EntityTypePlace, ActorID: "alice",
	})
	require.NoError(t, err)

	history, err := a.GetHistory(ctx, app.GetHistoryRequest{EntityPath: "Garage"})
	require.NoError(t, err)
	assert.Len(t, history, 1)
	assert.Equal(t, inventory.EntityCreatedEvent, history[0].EventType)
}
```

```go
// internal/app/search_test.go
package app_test

import (
	"context"
	"testing"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindEntities_ExactMatch(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	for _, name := range []string{"Garage", "Basement", "Kitchen"} {
		_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: name, EntityType: inventory.EntityTypePlace, ActorID: "alice",
		})
		require.NoError(t, err)
	}

	results, err := a.FindEntities(ctx, app.FindEntitiesRequest{Query: "Garage"})
	require.NoError(t, err)
	require.NotEmpty(t, results)
	assert.Equal(t, "Garage", results[0].Entity.DisplayName)
	assert.Equal(t, 0, results[0].Distance)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
gotestsum -- ./internal/app/...
```

Expected: compilation errors — ChangeStatus, GetHistory, FindEntities undefined.

- [ ] **Step 3: Create `status.go`**

```go
package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// ChangeStatus updates the lifecycle status of an entity.
func (a *App) ChangeStatus(ctx context.Context, req ChangeStatusRequest) error {
	entity, err := a.resolveEntityPath(ctx, req.EntityPath)
	if err != nil {
		return fmt.Errorf("resolve path %q: %w", req.EntityPath, err)
	}

	var statusContext *string
	if req.StatusContext != "" {
		statusContext = &req.StatusContext
	}

	payload := eventbus.EntityStatusChangedPayload{
		EntityID:      entity.EntityID,
		Status:        req.Status.String(),
		StatusContext: statusContext,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}

	if _, err := a.bus.Dispatch(ctx, inventory.EntityStatusChangedEvent, req.ActorID, raw, note); err != nil {
		return fmt.Errorf("change status: %w", err)
	}

	return nil
}
```

- [ ] **Step 4: Create `history.go`**

```go
package app

import (
	"context"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// GetHistory returns the event history for an entity, identified by path or ID.
func (a *App) GetHistory(ctx context.Context, req GetHistoryRequest) ([]HistoryResult, error) {
	var entityID string

	switch {
	case req.EntityID != "":
		entityID = req.EntityID
	case req.EntityPath != "":
		entity, err := a.resolveEntityPath(ctx, req.EntityPath)
		if err != nil {
			return nil, fmt.Errorf("resolve path %q: %w", req.EntityPath, err)
		}
		entityID = entity.EntityID
	default:
		return nil, fmt.Errorf("GetHistory: either EntityPath or EntityID must be set")
	}

	events, err := a.store.GetEventsByEntity(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("get history for %s: %w", entityID, err)
	}

	results := make([]HistoryResult, 0, len(events))
	for _, ev := range events {
		note := ""
		if ev.Note != nil {
			note = *ev.Note
		}
		results = append(results, HistoryResult{
			EventID:      ev.EventID,
			EventType:    ev.EventType,
			TimestampUTC: ev.TimestampUTC,
			ActorUserID:  ev.ActorUserID,
			Payload:      []byte(ev.Payload),
			Note:         note,
		})
	}

	if req.OldestFirst {
		return results, nil
	}

	// Default: newest first.
	for i, j := 0, len(results)-1; i < j; i, j = i+1, j-1 {
		results[i], results[j] = results[j], results[i]
	}

	if req.Limit > 0 && len(results) > req.Limit {
		results = results[:req.Limit]
	}

	return results, nil
}
```

- [ ] **Step 5: Create `search.go`**

```go
package app

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// FindEntities searches for entities whose display name contains the query string.
// Results are sorted by Levenshtein distance (exact matches first), then alphabetically.
// Removed entities are excluded.
func (a *App) FindEntities(ctx context.Context, req FindEntitiesRequest) ([]FindResult, error) {
	all, err := a.store.ListEntities(ctx)
	if err != nil {
		return nil, fmt.Errorf("find entities: %w", err)
	}

	query := strings.ToLower(req.Query)
	var results []FindResult

	for _, e := range all {
		if e.Status == inventory.EntityStatusRemoved {
			continue
		}
		name := strings.ToLower(e.DisplayName)
		if !strings.Contains(name, query) {
			continue
		}
		results = append(results, FindResult{
			Entity:   entityToResult(e),
			Distance: levenshtein(query, name),
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Distance != results[j].Distance {
			return results[i].Distance < results[j].Distance
		}
		return results[i].Entity.DisplayName < results[j].Entity.DisplayName
	})

	if req.Limit > 0 && len(results) > req.Limit {
		results = results[:req.Limit]
	}

	return results, nil
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	la, lb := len(ra), len(rb)

	dp := make([][]int, la+1)
	for i := range dp {
		dp[i] = make([]int, lb+1)
		dp[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		dp[0][j] = j
	}
	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			if ra[i-1] == rb[j-1] {
				dp[i][j] = dp[i-1][j-1]
			} else {
				dp[i][j] = 1 + min3(dp[i-1][j], dp[i][j-1], dp[i-1][j-1])
			}
		}
	}
	return dp[la][lb]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
```

- [ ] **Step 6: Run all app tests**

```bash
gotestsum -- ./internal/app/...
```

Expected: PASS

- [ ] **Step 7: Run lint**

```bash
mise run lint
```

Expected: no errors.

- [ ] **Step 8: Run full test suite**

```bash
mise run test
```

Expected: PASS — all existing `internal/database` tests still pass.

- [ ] **Step 9: Commit**

```bash
jj new -m "feat(app): add status, history, and search operations"
```

---

### Task 4: Verify the dependency graph

- [ ] **Step 1: Check that `app` does not import `internal/database`**

```bash
go list -f '{{join .Imports "\n"}}' github.com/asphaltbuffet/wherehouse/internal/app | grep database
```

Expected: no output (no `internal/database` import).

- [ ] **Step 2: Check that `store` does not import `eventbus` or `app`**

```bash
go list -f '{{join .Imports "\n"}}' github.com/asphaltbuffet/wherehouse/internal/store | grep -E "eventbus|app"
```

Expected: no output.

- [ ] **Step 3: Check that `inventory` has no internal imports**

```bash
go list -f '{{join .Imports "\n"}}' github.com/asphaltbuffet/wherehouse/internal/inventory | grep asphaltbuffet
```

Expected: no output.

- [ ] **Step 4: Commit**

```bash
jj new -m "chore(app): verify dependency graph is clean"
```
