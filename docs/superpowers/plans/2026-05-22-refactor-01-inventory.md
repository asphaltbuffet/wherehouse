# Refactor 01: `internal/inventory` Package Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract all pure domain types from `internal/database` into a new `internal/inventory` package that has zero external dependencies and is importable by every other layer.

**Architecture:** `internal/inventory` is the dependency-free foundation of the rewrite. It owns all domain enums (`EntityType`, `EntityStatus`, `EventType`), domain structs (`Entity`, `Event`), and pure functions (`CanonicalizeString`). It imports nothing from this codebase. Every other new package imports it; it imports none of them.

**Tech Stack:** Go 1.25, `database/sql/driver` (for `Value`/`Scan` SQL interfaces), `golang.org/x/text` (already in go.sum for unicode normalization if needed — otherwise stdlib only), `go generate` + `stringer`.

---

## Target File Map

```
internal/inventory/
  doc.go              # package-level doc comment
  entity_type.go      # EntityType iota, stringer directive, ParseEntityType
  entity_status.go    # EntityStatus iota, stringer directive, ParseEntityStatus
  event_type.go       # EventType iota, stringer directive, ParseEventType
  entity_type_sql.go  # Value/Scan for EntityType (driver.Valuer + sql.Scanner)
  entity_status_sql.go# Value/Scan for EntityStatus
  event_type_sql.go   # Value/Scan for EventType
  entity.go           # Entity struct
  event.go            # Event struct
  canonical.go        # CanonicalizeString
  errors.go           # sentinel errors scoped to this package
```

Generated (do not edit):
```
  entitytype_string.go
  entitystatus_string.go
  eventtype_string.go
```

---

### Task 1: Package scaffold and `EntityType`

**Files:**
- Create: `internal/inventory/doc.go`
- Create: `internal/inventory/entity_type.go`
- Create: `internal/inventory/entity_type_sql.go`
- Test: `internal/inventory/entity_type_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/inventory/entity_type_test.go
package inventory_test

import (
	"testing"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntityType_String(t *testing.T) {
	assert.Equal(t, "place", inventory.EntityTypePlace.String())
	assert.Equal(t, "container", inventory.EntityTypeContainer.String())
	assert.Equal(t, "leaf", inventory.EntityTypeLeaf.String())
}

func TestParseEntityType(t *testing.T) {
	got, err := inventory.ParseEntityType("place")
	require.NoError(t, err)
	assert.Equal(t, inventory.EntityTypePlace, got)

	got, err = inventory.ParseEntityType("container")
	require.NoError(t, err)
	assert.Equal(t, inventory.EntityTypeContainer, got)

	_, err = inventory.ParseEntityType("unknown")
	assert.Error(t, err)
}

func TestEntityType_SQLRoundtrip(t *testing.T) {
	et := inventory.EntityTypeContainer
	v, err := et.Value()
	require.NoError(t, err)
	assert.Equal(t, "container", v)

	var scanned inventory.EntityType
	require.NoError(t, scanned.Scan("container"))
	assert.Equal(t, inventory.EntityTypeContainer, scanned)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
gotestsum -- ./internal/inventory/...
```

Expected: compilation error — package does not exist yet.

- [ ] **Step 3: Create `doc.go`**

```go
// Package inventory defines the core domain types for wherehouse:
// entity types and statuses, event types, and the canonical naming rules.
// It has no dependencies on storage, CLI, or configuration.
package inventory
```

- [ ] **Step 4: Create `entity_type.go`**

```go
package inventory

import "fmt"

//go:generate stringer -type=EntityType -linecomment

// EntityType classifies the behavioral role of an entity in the inventory.
//
//nolint:recvcheck // Value() requires value receiver; Scan() requires pointer receiver.
type EntityType int

const (
	// EntityTypePlace is immovable and may only be nested inside other place entities.
	EntityTypePlace EntityType = iota + 1 // place
	// EntityTypeContainer is movable and may contain any entity type.
	EntityTypeContainer // container
	// EntityTypeLeaf is movable and may not contain other entities.
	EntityTypeLeaf // leaf
)

var entityTypeByName = map[string]EntityType{
	EntityTypePlace.String():     EntityTypePlace,
	EntityTypeContainer.String(): EntityTypeContainer,
	EntityTypeLeaf.String():      EntityTypeLeaf,
}

// ParseEntityType converts a string like "place" to its EntityType constant.
func ParseEntityType(s string) (EntityType, error) {
	if et, ok := entityTypeByName[s]; ok {
		return et, nil
	}
	return 0, fmt.Errorf("unknown entity type %q: must be place, container, or leaf", s)
}
```

- [ ] **Step 5: Create `entity_type_sql.go`**

```go
package inventory

import (
	"database/sql/driver"
	"fmt"
)

// Value implements driver.Valuer — stores the string representation in SQLite.
func (e EntityType) Value() (driver.Value, error) {
	return e.String(), nil
}

// Scan implements sql.Scanner — reads the string representation from SQLite.
func (e *EntityType) Scan(src any) error {
	s, ok := src.(string)
	if !ok {
		return fmt.Errorf("EntityType.Scan: expected string, got %T", src)
	}
	parsed, err := ParseEntityType(s)
	if err != nil {
		return err
	}
	*e = parsed
	return nil
}
```

- [ ] **Step 6: Run stringer to generate `entitytype_string.go`**

```bash
go generate ./internal/inventory/...
```

Expected: `internal/inventory/entitytype_string.go` created.

- [ ] **Step 7: Run tests to verify they pass**

```bash
gotestsum -- ./internal/inventory/...
```

Expected: PASS

- [ ] **Step 8: Commit**

```bash
jj new -m "feat(inventory): add EntityType with SQL interfaces"
```

---

### Task 2: `EntityStatus`

**Files:**
- Create: `internal/inventory/entity_status.go`
- Create: `internal/inventory/entity_status_sql.go`
- Test: `internal/inventory/entity_status_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/inventory/entity_status_test.go
package inventory_test

import (
	"testing"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntityStatus_String(t *testing.T) {
	assert.Equal(t, "ok", inventory.EntityStatusOk.String())
	assert.Equal(t, "borrowed", inventory.EntityStatusBorrowed.String())
	assert.Equal(t, "missing", inventory.EntityStatusMissing.String())
	assert.Equal(t, "loaned", inventory.EntityStatusLoaned.String())
	assert.Equal(t, "removed", inventory.EntityStatusRemoved.String())
}

func TestParseEntityStatus(t *testing.T) {
	got, err := inventory.ParseEntityStatus("ok")
	require.NoError(t, err)
	assert.Equal(t, inventory.EntityStatusOk, got)

	_, err = inventory.ParseEntityStatus("unknown")
	assert.Error(t, err)
}

func TestEntityStatus_SQLRoundtrip(t *testing.T) {
	es := inventory.EntityStatusMissing
	v, err := es.Value()
	require.NoError(t, err)
	assert.Equal(t, "missing", v)

	var scanned inventory.EntityStatus
	require.NoError(t, scanned.Scan("missing"))
	assert.Equal(t, inventory.EntityStatusMissing, scanned)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
gotestsum -- ./internal/inventory/...
```

Expected: compilation error — EntityStatus undefined.

- [ ] **Step 3: Create `entity_status.go`**

```go
package inventory

import "fmt"

//go:generate stringer -type=EntityStatus -linecomment

// EntityStatus describes the lifecycle state of an entity.
//
//nolint:recvcheck // Value() requires value receiver; Scan() requires pointer receiver.
type EntityStatus int

const (
	EntityStatusOk       EntityStatus = iota + 1 // ok
	EntityStatusBorrowed                          // borrowed
	EntityStatusMissing                           // missing
	EntityStatusLoaned                            // loaned
	EntityStatusRemoved                           // removed
)

var entityStatusByName = map[string]EntityStatus{
	EntityStatusOk.String():       EntityStatusOk,
	EntityStatusBorrowed.String(): EntityStatusBorrowed,
	EntityStatusMissing.String():  EntityStatusMissing,
	EntityStatusLoaned.String():   EntityStatusLoaned,
	EntityStatusRemoved.String():  EntityStatusRemoved,
}

// ParseEntityStatus converts a string like "missing" to its EntityStatus constant.
func ParseEntityStatus(s string) (EntityStatus, error) {
	if es, ok := entityStatusByName[s]; ok {
		return es, nil
	}
	return 0, fmt.Errorf("unknown entity status %q", s)
}
```

- [ ] **Step 4: Create `entity_status_sql.go`**

```go
package inventory

import (
	"database/sql/driver"
	"fmt"
)

// Value implements driver.Valuer.
func (e EntityStatus) Value() (driver.Value, error) {
	return e.String(), nil
}

// Scan implements sql.Scanner.
func (e *EntityStatus) Scan(src any) error {
	s, ok := src.(string)
	if !ok {
		return fmt.Errorf("EntityStatus.Scan: expected string, got %T", src)
	}
	parsed, err := ParseEntityStatus(s)
	if err != nil {
		return err
	}
	*e = parsed
	return nil
}
```

- [ ] **Step 5: Run stringer**

```bash
go generate ./internal/inventory/...
```

- [ ] **Step 6: Run tests**

```bash
gotestsum -- ./internal/inventory/...
```

Expected: PASS

- [ ] **Step 7: Commit**

```bash
jj new -m "feat(inventory): add EntityStatus with SQL interfaces"
```

---

### Task 3: `EventType`

**Files:**
- Create: `internal/inventory/event_type.go`
- Create: `internal/inventory/event_type_sql.go`
- Test: `internal/inventory/event_type_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/inventory/event_type_test.go
package inventory_test

import (
	"testing"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventType_String(t *testing.T) {
	assert.Equal(t, "entity.created", inventory.EntityCreatedEvent.String())
	assert.Equal(t, "entity.renamed", inventory.EntityRenamedEvent.String())
	assert.Equal(t, "entity.reparented", inventory.EntityReparentedEvent.String())
	assert.Equal(t, "entity.path_changed", inventory.EntityPathChangedEvent.String())
	assert.Equal(t, "entity.status_changed", inventory.EntityStatusChangedEvent.String())
	assert.Equal(t, "entity.removed", inventory.EntityRemovedEvent.String())
}

func TestParseEventType(t *testing.T) {
	got, err := inventory.ParseEventType("entity.created")
	require.NoError(t, err)
	assert.Equal(t, inventory.EntityCreatedEvent, got)

	_, err = inventory.ParseEventType("unknown")
	assert.Error(t, err)
}

func TestEventType_SQLRoundtrip(t *testing.T) {
	et := inventory.EntityRenamedEvent
	v, err := et.Value()
	require.NoError(t, err)
	assert.Equal(t, "entity.renamed", v)

	var scanned inventory.EventType
	require.NoError(t, scanned.Scan("entity.renamed"))
	assert.Equal(t, inventory.EntityRenamedEvent, scanned)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
gotestsum -- ./internal/inventory/...
```

Expected: compilation error — EventType undefined.

- [ ] **Step 3: Create `event_type.go`**

```go
package inventory

import "fmt"

//go:generate stringer -type=EventType -linecomment

// EventType identifies the kind of domain event stored in the events table.
//
//nolint:recvcheck // Value() requires value receiver; Scan() requires pointer receiver.
type EventType int

// Domain event types.
const (
	EntityCreatedEvent       EventType = iota + 1 // entity.created
	EntityRenamedEvent                            // entity.renamed
	EntityReparentedEvent                         // entity.reparented
	EntityPathChangedEvent                        // entity.path_changed
	EntityStatusChangedEvent                      // entity.status_changed
	EntityRemovedEvent                            // entity.removed
)

var eventTypeByName = map[string]EventType{
	EntityCreatedEvent.String():       EntityCreatedEvent,
	EntityRenamedEvent.String():       EntityRenamedEvent,
	EntityReparentedEvent.String():    EntityReparentedEvent,
	EntityPathChangedEvent.String():   EntityPathChangedEvent,
	EntityStatusChangedEvent.String(): EntityStatusChangedEvent,
	EntityRemovedEvent.String():       EntityRemovedEvent,
}

// ParseEventType converts a string like "entity.created" to its EventType constant.
func ParseEventType(s string) (EventType, error) {
	if et, ok := eventTypeByName[s]; ok {
		return et, nil
	}
	return 0, fmt.Errorf("unknown event type %q", s)
}
```

- [ ] **Step 4: Create `event_type_sql.go`**

```go
package inventory

import (
	"database/sql/driver"
	"fmt"
)

// Value implements driver.Valuer.
func (e EventType) Value() (driver.Value, error) {
	return e.String(), nil
}

// Scan implements sql.Scanner.
func (e *EventType) Scan(src any) error {
	s, ok := src.(string)
	if !ok {
		return fmt.Errorf("EventType.Scan: expected string, got %T", src)
	}
	parsed, err := ParseEventType(s)
	if err != nil {
		return err
	}
	*e = parsed
	return nil
}
```

- [ ] **Step 5: Run stringer**

```bash
go generate ./internal/inventory/...
```

- [ ] **Step 6: Run tests**

```bash
gotestsum -- ./internal/inventory/...
```

Expected: PASS

- [ ] **Step 7: Commit**

```bash
jj new -m "feat(inventory): add EventType with SQL interfaces"
```

---

### Task 4: `Entity` and `Event` structs

**Files:**
- Create: `internal/inventory/entity.go`
- Create: `internal/inventory/event.go`
- Test: `internal/inventory/entity_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/inventory/entity_test.go
package inventory_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntity_Fields(t *testing.T) {
	e := inventory.Entity{
		EntityID:          "abc123",
		DisplayName:       "Garage",
		CanonicalName:     "garage",
		EntityType:        inventory.EntityTypePlace,
		ParentID:          nil,
		FullPathDisplay:   "Garage",
		FullPathCanonical: "garage",
		Depth:             0,
		Status:            inventory.EntityStatusOk,
		StatusContext:     nil,
		LastEventID:       1,
		UpdatedAt:         time.Now(),
	}

	assert.Equal(t, "abc123", e.EntityID)
	assert.Equal(t, inventory.EntityTypePlace, e.EntityType)
	assert.Equal(t, inventory.EntityStatusOk, e.Status)
}

func TestEvent_Fields(t *testing.T) {
	payload := json.RawMessage(`{"entity_id":"abc123"}`)
	note := "a note"
	entityID := "abc123"

	ev := inventory.Event{
		EventID:      1,
		EventType:    inventory.EntityCreatedEvent,
		TimestampUTC: "2026-05-22T00:00:00Z",
		ActorUserID:  "alice",
		Payload:      payload,
		Note:         &note,
		EntityID:     &entityID,
	}

	require.NotNil(t, ev.Note)
	assert.Equal(t, "a note", *ev.Note)
	assert.Equal(t, inventory.EntityCreatedEvent, ev.EventType)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
gotestsum -- ./internal/inventory/...
```

Expected: compilation error — Entity and Event undefined.

- [ ] **Step 3: Create `entity.go`**

```go
package inventory

import "time"

// Entity represents the current projected state of an inventory entity.
// Values are read from the entities_current projection table.
type Entity struct {
	EntityID          string
	DisplayName       string
	CanonicalName     string
	EntityType        EntityType
	ParentID          *string
	FullPathDisplay   string
	FullPathCanonical string
	Depth             int
	Status            EntityStatus
	StatusContext     *string
	LastEventID       int64
	UpdatedAt         time.Time
}
```

- [ ] **Step 4: Create `event.go`**

```go
package inventory

import "encoding/json"

// Event represents a stored domain event from the events table.
type Event struct {
	EventID      int64
	EventType    EventType
	TimestampUTC string
	ActorUserID  string
	Payload      json.RawMessage
	Note         *string
	EntityID     *string
}
```

- [ ] **Step 5: Run tests**

```bash
gotestsum -- ./internal/inventory/...
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
jj new -m "feat(inventory): add Entity and Event structs"
```

---

### Task 5: `CanonicalizeString` and `errors`

**Files:**
- Create: `internal/inventory/canonical.go`
- Create: `internal/inventory/errors.go`
- Test: `internal/inventory/canonical_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/inventory/canonical_test.go
package inventory_test

import (
	"testing"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/stretchr/testify/assert"
)

func TestCanonicalizeString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Garage", "garage"},
		{"  Socket Set  ", "socket_set"},
		{"Drawer-3", "drawer_3"},
		{"multiple   spaces", "multiple_spaces"},
		{"trailing_", "trailing"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, inventory.CanonicalizeString(tt.input))
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
gotestsum -- ./internal/inventory/...
```

Expected: compilation error — CanonicalizeString undefined.

- [ ] **Step 3: Create `canonical.go`**

```go
package inventory

import (
	"strings"
	"unicode"
)

// CanonicalizeString normalizes a display name for case-insensitive matching.
// Lowercases, trims whitespace, and collapses runs of whitespace/hyphens/underscores
// into a single underscore. Trailing underscores are removed.
func CanonicalizeString(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimSpace(s)

	var result strings.Builder
	var prevWasUnderscore bool

	for _, r := range s {
		if unicode.IsSpace(r) || r == '-' || r == '_' {
			if !prevWasUnderscore {
				result.WriteRune('_')
				prevWasUnderscore = true
			}
		} else {
			result.WriteRune(r)
			prevWasUnderscore = false
		}
	}

	return strings.TrimRight(result.String(), "_")
}
```

- [ ] **Step 4: Create `errors.go`**

```go
package inventory

import "errors"

// Sentinel errors for the inventory domain.
var (
	ErrEntityNotFound = errors.New("entity not found")
	ErrEventNotFound  = errors.New("event not found")
)
```

- [ ] **Step 5: Run tests**

```bash
gotestsum -- ./internal/inventory/...
```

Expected: PASS

- [ ] **Step 6: Run full lint check**

```bash
mise run lint
```

Expected: no errors. Fix any that appear before committing.

- [ ] **Step 7: Commit**

```bash
jj new -m "feat(inventory): add CanonicalizeString and sentinel errors"
```

---

### Task 6: Verify package is dependency-free

- [ ] **Step 1: Check imports**

```bash
go list -f '{{join .Imports "\n"}}' github.com/asphaltbuffet/wherehouse/internal/inventory
```

Expected: only stdlib packages (`database/sql/driver`, `encoding/json`, `errors`, `fmt`, `strings`, `time`, `unicode`). No `github.com/asphaltbuffet/wherehouse/...` imports.

- [ ] **Step 2: Run full test suite to verify nothing broken**

```bash
mise run test
```

Expected: PASS (existing `internal/database` tests still pass — this plan does not touch that package).

- [ ] **Step 3: Commit**

```bash
jj new -m "chore(inventory): verify package has no internal dependencies"
```
