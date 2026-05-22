# Refactor 03: `internal/eventbus` Package Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create `internal/eventbus` as the single owner of all event logic — routing, business rule validation, projection updates, and derived event generation — using `internal/store` for persistence and `internal/inventory` for types.

**Architecture:** `eventbus` exposes one primary entry point: `Bus.Dispatch(ctx, event)`. It appends the raw event via `store`, then applies the event to projections inside the same transaction. All business rules (e.g. a place may only nest inside another place, path propagation on reparent) live here as pure handler functions that receive a `store.Tx`. No business logic lives in `store` or `inventory`.

**Tech Stack:** Go 1.25, `internal/inventory` (plan 01), `internal/store` (plan 02). Both must be complete before starting this plan.

**Prerequisites:** Plans 01 and 02 complete.

---

## Target File Map

```
internal/eventbus/
  doc.go              # package doc
  bus.go              # Bus struct, New, Dispatch
  handlers.go         # handleEntityCreated, handleEntityRenamed, handleEntityReparented,
                      # handleEntityPathChanged, handleEntityStatusChanged, handleEntityRemoved
  validation.go       # validatePlaceParent, pure validation helpers
  payloads.go         # typed payload structs for each event type
```

---

### Task 1: `Bus` scaffold and `Dispatch` router

**Files:**
- Create: `internal/eventbus/doc.go`
- Create: `internal/eventbus/payloads.go`
- Create: `internal/eventbus/bus.go`
- Test: `internal/eventbus/bus_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/eventbus/bus_test.go
package eventbus_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(store.Config{
		Path:        filepath.Join(t.TempDir(), "test.db"),
		AutoMigrate: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestDispatch_UnknownEventType(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)

	payload := json.RawMessage(`{}`)
	_, err := b.Dispatch(context.Background(), inventory.EventType(999), "alice", payload, nil)
	assert.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
gotestsum -- ./internal/eventbus/...
```

Expected: compilation error — package does not exist.

- [ ] **Step 3: Create `doc.go`**

```go
// Package eventbus owns all event processing for wherehouse.
// It is the single place where events are persisted and projections are updated.
// Business rules (validation, path propagation, derived events) live here.
package eventbus
```

- [ ] **Step 4: Create `payloads.go`**

```go
package eventbus

// EntityCreatedPayload is the JSON payload for entity.created events.
type EntityCreatedPayload struct {
	EntityID    string  `json:"entity_id"`
	DisplayName string  `json:"display_name"`
	EntityType  string  `json:"entity_type"`
	ParentID    *string `json:"parent_id,omitempty"`
}

// EntityRenamedPayload is the JSON payload for entity.renamed events.
type EntityRenamedPayload struct {
	EntityID    string `json:"entity_id"`
	DisplayName string `json:"display_name"`
}

// EntityReparentedPayload is the JSON payload for entity.reparented events.
type EntityReparentedPayload struct {
	EntityID    string  `json:"entity_id"`
	NewParentID *string `json:"new_parent_id,omitempty"`
}

// EntityPathChangedPayload is the JSON payload for entity.path_changed (derived) events.
type EntityPathChangedPayload struct {
	EntityID          string `json:"entity_id"`
	FullPathDisplay   string `json:"full_path_display"`
	FullPathCanonical string `json:"full_path_canonical"`
	Depth             int    `json:"depth"`
}

// EntityStatusChangedPayload is the JSON payload for entity.status_changed events.
type EntityStatusChangedPayload struct {
	EntityID      string  `json:"entity_id"`
	Status        string  `json:"status"`
	StatusContext *string `json:"status_context,omitempty"`
}

// EntityRemovedPayload is the JSON payload for entity.removed events.
type EntityRemovedPayload struct {
	EntityID string `json:"entity_id"`
}
```

- [ ] **Step 5: Create `bus.go`**

```go
package eventbus

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// Bus dispatches events: persisting them and updating projections atomically.
type Bus struct {
	store *store.Store
}

// New creates a Bus backed by the given store.
func New(s *store.Store) *Bus {
	return &Bus{store: s}
}

// Dispatch persists an event and applies it to projections in a single transaction.
// Returns the assigned event ID.
func (b *Bus) Dispatch(
	ctx context.Context,
	eventType inventory.EventType,
	actorUserID string,
	payload json.RawMessage,
	note *string,
) (int64, error) {
	// Extract entity_id from payload for indexing (best-effort).
	var entityID *string
	var m map[string]any
	if json.Unmarshal(payload, &m) == nil {
		if id, ok := m["entity_id"].(string); ok && id != "" {
			entityID = &id
		}
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)

	var eventID int64
	err := b.store.ExecInTransaction(ctx, func(tx store.Tx) error {
		// 1. Persist the event row.
		const q = `
			INSERT INTO events (event_type, timestamp_utc, actor_user_id, payload, note, entity_id)
			VALUES (?, ?, ?, ?, ?, ?)`
		result, err := tx.ExecContext(ctx, q, eventType, timestamp, actorUserID, string(payload), note, entityID)
		if err != nil {
			return fmt.Errorf("insert event: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("get event ID: %w", err)
		}
		eventID = id

		// 2. Build the Event value for handler dispatch.
		ev := &inventory.Event{
			EventID:      eventID,
			EventType:    eventType,
			TimestampUTC: timestamp,
			ActorUserID:  actorUserID,
			Payload:      payload,
			Note:         note,
			EntityID:     entityID,
		}

		// 3. Apply the event to projections.
		return b.applyEventTx(ctx, tx, ev)
	})
	if err != nil {
		return 0, err
	}
	return eventID, nil
}

// applyEventTx routes an event to its projection handler.
func (b *Bus) applyEventTx(ctx context.Context, tx *sql.Tx, ev *inventory.Event) error {
	switch ev.EventType {
	case inventory.EntityCreatedEvent:
		return b.handleEntityCreated(ctx, tx, ev)
	case inventory.EntityRenamedEvent:
		return b.handleEntityRenamed(ctx, tx, ev)
	case inventory.EntityReparentedEvent:
		return b.handleEntityReparented(ctx, tx, ev)
	case inventory.EntityPathChangedEvent:
		return b.handleEntityPathChanged(ctx, tx, ev)
	case inventory.EntityStatusChangedEvent:
		return b.handleEntityStatusChanged(ctx, tx, ev)
	case inventory.EntityRemovedEvent:
		return b.handleEntityRemoved(ctx, tx, ev)
	default:
		return fmt.Errorf("unknown event type: %s", ev.EventType)
	}
}
```

- [ ] **Step 6: Run tests**

```bash
gotestsum -- ./internal/eventbus/...
```

Expected: PASS (only the unknown-type test runs; handlers not yet defined but `Dispatch` compiles).

- [ ] **Step 7: Commit**

```bash
jj new -m "feat(eventbus): add Bus, Dispatch, payload types"
```

---

### Task 2: Validation helpers

**Files:**
- Create: `internal/eventbus/validation.go`
- Test: `internal/eventbus/validation_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/eventbus/validation_test.go
package eventbus_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createPlace inserts a place entity via eventbus to set up test state.
func createPlace(t *testing.T, b *eventbus.Bus, id, name string, parentID *string) {
	t.Helper()
	p := eventbus.EntityCreatedPayload{
		EntityID:    id,
		DisplayName: name,
		EntityType:  "place",
		ParentID:    parentID,
	}
	raw, err := json.Marshal(p)
	require.NoError(t, err)
	_, err = b.Dispatch(context.Background(), inventory.EntityCreatedEvent, "test", raw, nil)
	require.NoError(t, err)
}

func createLeaf(t *testing.T, b *eventbus.Bus, id, name string, parentID *string) {
	t.Helper()
	p := eventbus.EntityCreatedPayload{
		EntityID:    id,
		DisplayName: name,
		EntityType:  "leaf",
		ParentID:    parentID,
	}
	raw, err := json.Marshal(p)
	require.NoError(t, err)
	_, err = b.Dispatch(context.Background(), inventory.EntityCreatedEvent, "test", raw, nil)
	require.NoError(t, err)
}

func TestValidatePlaceParent_PlaceInPlace_OK(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)

	createPlace(t, b, "p1", "Garage", nil)

	p2ID := "p1"
	// Creating a place inside another place should succeed.
	p := eventbus.EntityCreatedPayload{
		EntityID: "p2", DisplayName: "Zone", EntityType: "place", ParentID: &p2ID,
	}
	raw, err := json.Marshal(p)
	require.NoError(t, err)
	_, err = b.Dispatch(context.Background(), inventory.EntityCreatedEvent, "test", raw, nil)
	assert.NoError(t, err)
}

func TestValidatePlaceParent_PlaceInLeaf_Error(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)

	createPlace(t, b, "p1", "Garage", nil)
	leafParentID := "p1"
	createLeaf(t, b, "l1", "Wrench", &leafParentID)

	// Creating a place inside a leaf should fail.
	leafID := "l1"
	p := eventbus.EntityCreatedPayload{
		EntityID: "p3", DisplayName: "Zone", EntityType: "place", ParentID: &leafID,
	}
	raw, err := json.Marshal(p)
	require.NoError(t, err)
	_, err = b.Dispatch(context.Background(), inventory.EntityCreatedEvent, "test", raw, nil)
	assert.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
gotestsum -- -run TestValidate ./internal/eventbus/...
```

Expected: compilation error — handlers.go not yet written so Dispatch panics/errors on EntityCreatedEvent.

- [ ] **Step 3: Create `validation.go`**

```go
package eventbus

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// validatePlaceParentTx enforces the rule: a place entity may only be nested inside another place.
// Must be called inside an existing transaction.
func validatePlaceParentTx(ctx context.Context, tx *sql.Tx, parentID string) error {
	var entityTypeStr string
	err := tx.QueryRowContext(ctx,
		`SELECT entity_type FROM entities_current WHERE entity_id = ?`,
		parentID,
	).Scan(&entityTypeStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("parent entity %q not found", parentID)
		}
		return fmt.Errorf("query parent %s: %w", parentID, err)
	}

	parentType, err := inventory.ParseEntityType(entityTypeStr)
	if err != nil {
		return fmt.Errorf("parse parent entity type: %w", err)
	}

	if parentType != inventory.EntityTypePlace {
		return fmt.Errorf("a place entity can only be nested inside another place, not %q", entityTypeStr)
	}

	return nil
}
```

- [ ] **Step 4: Run tests (expect handler failures, not validation failures)**

```bash
gotestsum -- ./internal/eventbus/...
```

Expected: tests in validation_test.go fail because `handleEntityCreated` is not implemented yet. That is expected and will be fixed in Task 3.

---

### Task 3: Entity event handlers

**Files:**
- Create: `internal/eventbus/handlers.go`
- Test: coverage via existing bus_test.go and validation_test.go

- [ ] **Step 1: Create `handlers.go`**

```go
package eventbus

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

func (b *Bus) handleEntityCreated(ctx context.Context, tx *sql.Tx, ev *inventory.Event) error {
	var p EntityCreatedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return fmt.Errorf("handleEntityCreated: unmarshal: %w", err)
	}

	entityType, err := inventory.ParseEntityType(p.EntityType)
	if err != nil {
		return fmt.Errorf("handleEntityCreated: %w", err)
	}

	if entityType == inventory.EntityTypePlace && p.ParentID != nil {
		if err := validatePlaceParentTx(ctx, tx, *p.ParentID); err != nil {
			return fmt.Errorf("handleEntityCreated: %w", err)
		}
	}

	canonicalName := inventory.CanonicalizeString(p.DisplayName)
	fullPathDisplay, fullPathCanonical, depth, err := b.store.ComputeEntityPathTx(ctx, tx, p.DisplayName, canonicalName, p.ParentID)
	if err != nil {
		return fmt.Errorf("handleEntityCreated: %w", err)
	}

	entity := &inventory.Entity{
		EntityID:          p.EntityID,
		DisplayName:       p.DisplayName,
		CanonicalName:     canonicalName,
		EntityType:        entityType,
		ParentID:          p.ParentID,
		FullPathDisplay:   fullPathDisplay,
		FullPathCanonical: fullPathCanonical,
		Depth:             depth,
		Status:            inventory.EntityStatusOk,
		LastEventID:       ev.EventID,
		UpdatedAt:         time.Now().UTC(),
	}

	return b.store.InsertEntityTx(ctx, tx, entity)
}

func (b *Bus) handleEntityRenamed(ctx context.Context, tx *sql.Tx, ev *inventory.Event) error {
	var p EntityRenamedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return fmt.Errorf("handleEntityRenamed: unmarshal: %w", err)
	}

	entity, err := b.store.GetEntityTx(ctx, tx, p.EntityID)
	if err != nil {
		return fmt.Errorf("handleEntityRenamed: get entity: %w", err)
	}

	oldCanonical := entity.CanonicalName
	entity.DisplayName = p.DisplayName
	entity.CanonicalName = inventory.CanonicalizeString(p.DisplayName)

	// Recompute path with new name.
	entity.FullPathDisplay, entity.FullPathCanonical, entity.Depth, err = b.store.ComputeEntityPathTx(
		ctx, tx, entity.DisplayName, entity.CanonicalName, entity.ParentID,
	)
	if err != nil {
		return fmt.Errorf("handleEntityRenamed: recompute path: %w", err)
	}

	entity.LastEventID = ev.EventID
	entity.UpdatedAt = time.Now().UTC()

	if err := b.store.UpdateEntityTx(ctx, tx, entity); err != nil {
		return fmt.Errorf("handleEntityRenamed: %w", err)
	}

	// Propagate path changes to all descendants if the canonical name changed.
	if entity.CanonicalName != oldCanonical {
		if err := b.propagatePathChangesTx(ctx, tx, ev, entity); err != nil {
			return fmt.Errorf("handleEntityRenamed: propagate: %w", err)
		}
	}

	return nil
}

func (b *Bus) handleEntityReparented(ctx context.Context, tx *sql.Tx, ev *inventory.Event) error {
	var p EntityReparentedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return fmt.Errorf("handleEntityReparented: unmarshal: %w", err)
	}

	entity, err := b.store.GetEntityTx(ctx, tx, p.EntityID)
	if err != nil {
		return fmt.Errorf("handleEntityReparented: get entity: %w", err)
	}

	entity.ParentID = p.NewParentID
	entity.FullPathDisplay, entity.FullPathCanonical, entity.Depth, err = b.store.ComputeEntityPathTx(
		ctx, tx, entity.DisplayName, entity.CanonicalName, entity.ParentID,
	)
	if err != nil {
		return fmt.Errorf("handleEntityReparented: recompute path: %w", err)
	}

	entity.LastEventID = ev.EventID
	entity.UpdatedAt = time.Now().UTC()

	if err := b.store.UpdateEntityTx(ctx, tx, entity); err != nil {
		return fmt.Errorf("handleEntityReparented: %w", err)
	}

	return b.propagatePathChangesTx(ctx, tx, ev, entity)
}

func (b *Bus) handleEntityPathChanged(ctx context.Context, tx *sql.Tx, ev *inventory.Event) error {
	var p EntityPathChangedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return fmt.Errorf("handleEntityPathChanged: unmarshal: %w", err)
	}

	entity, err := b.store.GetEntityTx(ctx, tx, p.EntityID)
	if err != nil {
		return fmt.Errorf("handleEntityPathChanged: get entity: %w", err)
	}

	entity.FullPathDisplay = p.FullPathDisplay
	entity.FullPathCanonical = p.FullPathCanonical
	entity.Depth = p.Depth
	entity.LastEventID = ev.EventID
	entity.UpdatedAt = time.Now().UTC()

	return b.store.UpdateEntityTx(ctx, tx, entity)
}

func (b *Bus) handleEntityStatusChanged(ctx context.Context, tx *sql.Tx, ev *inventory.Event) error {
	var p EntityStatusChangedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return fmt.Errorf("handleEntityStatusChanged: unmarshal: %w", err)
	}

	status, err := inventory.ParseEntityStatus(p.Status)
	if err != nil {
		return fmt.Errorf("handleEntityStatusChanged: %w", err)
	}

	entity, err := b.store.GetEntityTx(ctx, tx, p.EntityID)
	if err != nil {
		return fmt.Errorf("handleEntityStatusChanged: get entity: %w", err)
	}

	entity.Status = status
	entity.StatusContext = p.StatusContext
	entity.LastEventID = ev.EventID
	entity.UpdatedAt = time.Now().UTC()

	return b.store.UpdateEntityTx(ctx, tx, entity)
}

func (b *Bus) handleEntityRemoved(ctx context.Context, tx *sql.Tx, ev *inventory.Event) error {
	var p EntityRemovedPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		return fmt.Errorf("handleEntityRemoved: unmarshal: %w", err)
	}

	entity, err := b.store.GetEntityTx(ctx, tx, p.EntityID)
	if err != nil {
		return fmt.Errorf("handleEntityRemoved: get entity: %w", err)
	}

	entity.Status = inventory.EntityStatusRemoved
	entity.LastEventID = ev.EventID
	entity.UpdatedAt = time.Now().UTC()

	return b.store.UpdateEntityTx(ctx, tx, entity)
}

// propagatePathChangesTx emits entity.path_changed derived events for all descendants
// of the given entity, updating each descendant's projection within the same transaction.
func (b *Bus) propagatePathChangesTx(ctx context.Context, tx *sql.Tx, triggeringEv *inventory.Event, parent *inventory.Entity) error {
	descendants, err := b.store.GetDescendantsTx(ctx, tx, parent.EntityID)
	if err != nil {
		return fmt.Errorf("get descendants: %w", err)
	}

	for _, d := range descendants {
		grandparentPath := parent.FullPathDisplay
		grandparentCanonical := parent.FullPathCanonical

		d.FullPathDisplay = grandparentPath + ":" + d.DisplayName
		d.FullPathCanonical = grandparentCanonical + ":" + d.CanonicalName
		d.Depth = parent.Depth + 1
		d.LastEventID = triggeringEv.EventID
		d.UpdatedAt = time.Now().UTC()

		payload := EntityPathChangedPayload{
			EntityID:          d.EntityID,
			FullPathDisplay:   d.FullPathDisplay,
			FullPathCanonical: d.FullPathCanonical,
			Depth:             d.Depth,
		}
		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal path_changed payload for %s: %w", d.EntityID, err)
		}

		const q = `
			INSERT INTO events (event_type, timestamp_utc, actor_user_id, payload, note, entity_id)
			VALUES (?, ?, ?, ?, NULL, ?)`
		_, err = tx.ExecContext(ctx, q,
			inventory.EntityPathChangedEvent,
			time.Now().UTC().Format(time.RFC3339),
			triggeringEv.ActorUserID,
			string(payloadJSON),
			d.EntityID,
		)
		if err != nil {
			return fmt.Errorf("insert path_changed event for %s: %w", d.EntityID, err)
		}

		if err := b.store.UpdateEntityTx(ctx, tx, d); err != nil {
			return fmt.Errorf("update descendant %s: %w", d.EntityID, err)
		}
	}

	return nil
}
```

Note: `handlers.go` calls `b.store.GetEntityTx` and `b.store.GetDescendantsTx` — these are transaction-scoped variants that need to be added to `internal/store/entities.go` in plan 02's entities file. Add them now:

In `internal/store/entities.go`, add:

```go
// GetEntityTx retrieves a single entity by ID inside an existing transaction.
func (s *Store) GetEntityTx(ctx context.Context, tx Tx, entityID string) (*inventory.Entity, error) {
	const query = `
		SELECT entity_id, display_name, canonical_name, entity_type,
		       parent_id, full_path_display, full_path_canonical,
		       depth, status, status_context, last_event_id, updated_at
		FROM entities_current WHERE entity_id = ?`

	row := tx.QueryRowContext(ctx, query, entityID)
	e, err := scanEntity(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get entity tx %s: %w", entityID, err)
	}
	return e, nil
}

// GetDescendantsTx retrieves all descendants of a given entity inside an existing transaction,
// using path prefix matching. Ordered by depth ASC, display_name ASC, entity_id ASC.
func (s *Store) GetDescendantsTx(ctx context.Context, tx Tx, entityID string) ([]*inventory.Entity, error) {
	const pathQuery = `SELECT full_path_canonical FROM entities_current WHERE entity_id = ?`
	var prefix string
	if err := tx.QueryRowContext(ctx, pathQuery, entityID).Scan(&prefix); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get path for %s: %w", entityID, err)
	}

	const query = `
		SELECT entity_id, display_name, canonical_name, entity_type,
		       parent_id, full_path_display, full_path_canonical,
		       depth, status, status_context, last_event_id, updated_at
		FROM entities_current WHERE full_path_canonical LIKE ?
		ORDER BY depth ASC, display_name ASC, entity_id ASC`

	rows, err := tx.QueryContext(ctx, query, prefix+":%")
	if err != nil {
		return nil, fmt.Errorf("get descendants of %s: %w", entityID, err)
	}
	defer rows.Close()
	return scanEntities(rows)
}
```

- [ ] **Step 2: Run all eventbus tests**

```bash
gotestsum -- ./internal/eventbus/...
```

Expected: PASS — including the validation tests from Task 2.

- [ ] **Step 3: Run full test suite**

```bash
mise run test
```

Expected: PASS

- [ ] **Step 4: Run lint**

```bash
mise run lint
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
jj new -m "feat(eventbus): add entity event handlers and path propagation"
```

---

### Task 4: Integration test — full event lifecycle

**Files:**
- Create: `internal/eventbus/integration_test.go`

- [ ] **Step 1: Write the integration tests**

```go
// internal/eventbus/integration_test.go
package eventbus_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntityLifecycle_CreateAndRename(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)
	ctx := context.Background()

	// Create a place entity.
	createPlace(t, b, "p1", "Garage", nil)

	entity, err := s.GetEntity(ctx, "p1")
	require.NoError(t, err)
	assert.Equal(t, "Garage", entity.DisplayName)
	assert.Equal(t, "garage", entity.CanonicalName)
	assert.Equal(t, "Garage", entity.FullPathDisplay)

	// Rename it.
	renamePayload, err := json.Marshal(eventbus.EntityRenamedPayload{EntityID: "p1", DisplayName: "Workshop"})
	require.NoError(t, err)
	_, err = b.Dispatch(ctx, inventory.EntityRenamedEvent, "alice", renamePayload, nil)
	require.NoError(t, err)

	renamed, err := s.GetEntity(ctx, "p1")
	require.NoError(t, err)
	assert.Equal(t, "Workshop", renamed.DisplayName)
	assert.Equal(t, "workshop", renamed.CanonicalName)
}

func TestEntityLifecycle_PathPropagation(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)
	ctx := context.Background()

	// Garage > Toolbox > Socket Set
	createPlace(t, b, "p1", "Garage", nil)
	p1ID := "p1"
	createPlace(t, b, "p2", "Toolbox", &p1ID)
	p2ID := "p2"
	createPlace(t, b, "p3", "Socket Set", &p2ID)

	// Rename Garage → Workshop; paths of Toolbox and Socket Set must update.
	renamePayload, err := json.Marshal(eventbus.EntityRenamedPayload{EntityID: "p1", DisplayName: "Workshop"})
	require.NoError(t, err)
	_, err = b.Dispatch(ctx, inventory.EntityRenamedEvent, "alice", renamePayload, nil)
	require.NoError(t, err)

	toolbox, err := s.GetEntity(ctx, "p2")
	require.NoError(t, err)
	assert.Equal(t, "Workshop:Toolbox", toolbox.FullPathDisplay)

	socketSet, err := s.GetEntity(ctx, "p3")
	require.NoError(t, err)
	assert.Equal(t, "Workshop:Toolbox:Socket Set", socketSet.FullPathDisplay)
}

func TestEntityLifecycle_StatusChange(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)
	ctx := context.Background()

	createPlace(t, b, "p1", "Garage", nil)
	p1ID := "p1"
	createLeaf(t, b, "l1", "Wrench", &p1ID)

	note := "left at job site"
	statusPayload, err := json.Marshal(eventbus.EntityStatusChangedPayload{
		EntityID:      "l1",
		Status:        "missing",
		StatusContext: &note,
	})
	require.NoError(t, err)
	_, err = b.Dispatch(ctx, inventory.EntityStatusChangedEvent, "alice", statusPayload, nil)
	require.NoError(t, err)

	entity, err := s.GetEntity(ctx, "l1")
	require.NoError(t, err)
	assert.Equal(t, inventory.EntityStatusMissing, entity.Status)
	require.NotNil(t, entity.StatusContext)
	assert.Equal(t, "left at job site", *entity.StatusContext)
}
```

- [ ] **Step 2: Run the integration tests**

```bash
gotestsum -- -run TestEntityLifecycle ./internal/eventbus/...
```

Expected: PASS

- [ ] **Step 3: Run full test suite and lint**

```bash
mise run test
mise run lint
```

Expected: PASS, no lint errors.

- [ ] **Step 4: Commit**

```bash
jj new -m "test(eventbus): add entity lifecycle integration tests"
```
