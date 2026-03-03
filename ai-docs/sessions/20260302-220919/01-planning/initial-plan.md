# Architecture Plan: EventType Typed Enum Refactor

## Overview

Refactor `EventType string` on the `Event` struct to `EventType EventType` (the typed `int` enum from `internal/database/eventTypes.go`). The database column must remain a string. The stringer tool generates `String()` methods automatically, and custom `driver.Valuer` / `sql.Scanner` implementations bridge the typed int to its string representation in the database layer.

---

## Current State Analysis

### Event struct (`internal/database/events.go`)
```go
type Event struct {
    EventID      int64
    EventType    string   // <-- to become: EventType EventType
    TimestampUTC string
    ActorUserID  string
    Payload      json.RawMessage
    Note         *string
    ItemID       *string
    LocationID   *string
    ProjectID    *string
}
```

### EventType enum (`internal/database/eventTypes.go`)
- Typed `int` enum with 16 constants (`ItemCreatedEvent` through `ProjectDeletedEvent`)
- Comments on each constant include the intended string value (e.g., `// item.created`)
- No stringer generation, no `Value()`/`Scan()` methods yet

### String usage inventory

All callers pass plain string literals. The affected files and their call sites:

| File | Call site | String value used |
|------|-----------|-------------------|
| `internal/database/eventHandler.go` | `switch event.EventType` | 14 string case literals |
| `internal/database/events.go` | `AppendEvent` signature + struct assignment + `GetEventsByType` param | Raw string passthrough |
| `internal/database/helper_test.go` | `insertEvent(ctx, "location.created", ...)` | Many raw strings |
| `internal/database/integration_test.go` | `insertEvent(ctx, "location.created", ...)`, `GetEventsByType(ctx, "location.created")` | Many raw strings |
| `internal/cli/add.go` | `AppendEvent(ctx, "item.created", ...)` | 1 string |
| `internal/cli/selectors_test.go` | `AppendEvent(ctx, "location.created", ...)` etc. | ~13 calls |
| `cmd/history/output.go` | `event.EventType` compared as string, passed to `EventStyle()` | 5+ comparisons |
| `cmd/lost/item.go` | `AppendEvent(ctx, "item.missing", ...)` | 1 string |
| `cmd/found/found.go` | `AppendEvent(ctx, "item.found", ...)`, `AppendEvent(ctx, "item.moved", ...)` | 2 strings |
| `cmd/loan/item.go` | `AppendEvent(ctx, "item.loaned", ...)` | 1 string |
| `cmd/add/location.go` | `AppendEvent(ctx, "location.created", ...)` | 1 string |
| `cmd/move/item.go` | `AppendEvent(ctx, "item.moved", ...)` | 1 string |
| `cmd/move/mover.go` | interface `AppendEvent` signature | Signature |
| `internal/styles/styles.go` | `EventStyle(key string)` switch on string | Receives string from caller |

---

## Architecture Design

### Layer 1: EventType Stringer (db-developer scope)

Add a `//go:generate` directive to `internal/database/eventTypes.go` for the `stringer` tool. This generates `eventTypes_string.go` automatically.

The stringer comment format on each constant already documents the intended string value (e.g., `// item.created`). However, the default `stringer` output would produce names like `ItemCreatedEvent`, not `item.created`.

**Decision**: Use `-linecomment` flag so stringer reads the inline comment as the string value. This makes `ItemCreatedEvent.String()` return `"item.created"` exactly.

Directive to add at top of `eventTypes.go`:
```go
//go:generate stringer -type=EventType -linecomment
```

Generated file `eventTypes_string.go` will implement:
```go
func (i EventType) String() string { ... }
```

### Layer 2: Database Bridge (db-developer scope)

Add `driver.Valuer` and `sql.Scanner` to `EventType` in `internal/database/eventTypes.go` (or a new file `internal/database/eventTypes_sql.go`).

**Recommended approach**: New file `internal/database/eventTypes_sql.go` to keep generated and hand-written code separate.

```go
package database

import (
    "database/sql/driver"
    "fmt"
)

// Value implements driver.Valuer, persisting EventType as its string representation.
// This ensures the database column stores "item.created" not an integer.
func (e EventType) Value() (driver.Value, error) {
    return e.String(), nil
}

// Scan implements sql.Scanner, reading the string from the database and converting
// back to the typed EventType constant.
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

This requires a `ParseEventType(s string) (EventType, error)` function that maps strings back to constants. The stringer tool does NOT generate a reverse mapping, so this must be hand-written. It belongs in `internal/database/eventTypes.go` (or `eventTypes_sql.go`).

```go
// ParseEventType converts a string representation to an EventType constant.
// Returns an error for unrecognized strings to fail loudly on mismatch.
func ParseEventType(s string) (EventType, error) {
    switch s {
    case "item.created":
        return ItemCreatedEvent, nil
    case "item.moved":
        return ItemMovedEvent, nil
    case "item.missing":
        return ItemMissingEvent, nil
    case "item.borrowed":
        return ItemBorrowedEvent, nil
    case "item.loaned":
        return ItemLoanedEvent, nil
    case "item.found":
        return ItemFoundEvent, nil
    case "item.deleted":
        return ItemDeletedEvent, nil
    case "location.created":
        return LocationCreatedEvent, nil
    case "location.renamed":
        return LocationRenamedEvent, nil
    case "location.reparented":
        return LocationMovedEvent, nil
    case "location.deleted":
        return LocationDeletedEvent, nil
    case "project.created":
        return ProjectCreatedEvent, nil
    case "project.completed":
        return ProjectCompletedEvent, nil
    case "project.reopened":
        return ProjectReopenedEvent, nil
    case "project.deleted":
        return ProjectDeletedEvent, nil
    default:
        return 0, fmt.Errorf("unknown event type: %q", s)
    }
}
```

### Layer 3: Event Struct Change (db-developer scope)

In `internal/database/events.go`, change the `EventType` field:

```go
// Before
EventType string

// After
EventType EventType
```

The `Scan` and `Value` implementations mean `rows.Scan(&event.EventType)` will continue to work transparently.

### Layer 4: AppendEvent Signature Change (db-developer scope)

Change `AppendEvent` and `insertEvent` to accept `EventType` instead of `string`:

```go
// Before
func (d *Database) AppendEvent(ctx context.Context, eventType, actorUserID string, ...) (int64, error)

// After
func (d *Database) AppendEvent(ctx context.Context, eventType EventType, actorUserID string, ...) (int64, error)
```

When passing `eventType` to SQL, the `driver.Valuer` interface handles conversion automatically.

The `insertEvent` helper (in `helper_test.go`) must also change:
```go
// Before
func (d *Database) insertEvent(ctx context.Context, eventType, actorUserID string, ...)

// After
func (d *Database) insertEvent(ctx context.Context, eventType EventType, actorUserID string, ...)
```

### Layer 5: eventHandler.go Switch Update (db-developer scope)

Replace string case literals with typed constants:

```go
// Before
switch event.EventType {
case "location.created":
    return d.handleLocationCreated(ctx, tx, event)

// After
switch event.EventType {
case LocationCreatedEvent:
    return d.handleLocationCreated(ctx, tx, event)
```

This is a pure mechanical change; all 14 cases update to their constant.

### Layer 6: GetEventsByType Signature (db-developer scope)

```go
// Before
func (d *Database) GetEventsByType(ctx context.Context, eventType string) ([]*Event, error)

// After
func (d *Database) GetEventsByType(ctx context.Context, eventType EventType) ([]*Event, error)
```

### Layer 7: Call Sites in cmd/ and internal/cli/ (golang-developer / golang-ui-developer scope)

All call sites that currently pass string literals must change to typed constants.

**Call site mapping** (string literal → constant):

| String | Constant |
|--------|----------|
| `"item.created"` | `database.ItemCreatedEvent` |
| `"item.moved"` | `database.ItemMovedEvent` |
| `"item.missing"` | `database.ItemMissingEvent` |
| `"item.borrowed"` | `database.ItemBorrowedEvent` |
| `"item.loaned"` | `database.ItemLoanedEvent` |
| `"item.found"` | `database.ItemFoundEvent` |
| `"item.deleted"` | `database.ItemDeletedEvent` |
| `"location.created"` | `database.LocationCreatedEvent` |
| `"location.renamed"` | `database.LocationRenamedEvent` |
| `"location.reparented"` | `database.LocationMovedEvent` |
| `"location.deleted"` | `database.LocationDeletedEvent` |
| `"project.created"` | `database.ProjectCreatedEvent` |
| `"project.completed"` | `database.ProjectCompletedEvent` |
| `"project.reopened"` | `database.ProjectReopenedEvent` |
| `"project.deleted"` | `database.ProjectDeletedEvent` |

**Files requiring call-site changes**:

- `internal/cli/add.go` (golang-developer): 1 call
- `cmd/add/location.go` (golang-ui-developer): 1 call
- `cmd/lost/item.go` (golang-ui-developer): 1 call
- `cmd/found/found.go` (golang-ui-developer): 2 calls
- `cmd/loan/item.go` (golang-ui-developer): 1 call
- `cmd/move/item.go` (golang-ui-developer): 1 call
- `cmd/move/mover.go` (golang-ui-developer): interface signature update

### Layer 8: history output.go (golang-ui-developer scope)

`cmd/history/output.go` uses `event.EventType` in:

1. String comparison: `event.EventType == "item.found"` → `event.EventType == database.ItemFoundEvent`
2. String comparison: `event.EventType == eventTypeMissing` where `eventTypeMissing = "item.missing"` → the constant `eventTypeMissing` becomes `database.ItemMissingEvent`
3. `switch event.EventType` with string cases → switch on typed constant cases
4. Calling `appStyles.EventStyle(event.EventType)` where `EventStyle` takes `string` → call `appStyles.EventStyle(event.EventType.String())`

The `EventStyle(key string)` in `internal/styles/styles.go` is in the TUI/golang-ui-developer scope. It can remain `string`-based (receiving `.String()`) OR be updated to accept `database.EventType` directly. **Recommended**: keep `EventStyle(key string)` as-is, call with `.String()`. This avoids introducing a dependency from `internal/styles` on `internal/database`.

### Layer 9: The moveDB Interface (golang-ui-developer scope)

`cmd/move/mover.go` defines a local `moveDB` interface:

```go
AppendEvent(ctx context.Context, eventType, actorUserID string, payload any, note string) (int64, error)
```

This must update to:
```go
AppendEvent(ctx context.Context, eventType database.EventType, actorUserID string, payload any, note string) (int64, error)
```

The mocks in `cmd/move/mocks/` must be regenerated or updated accordingly.

---

## TDD Implementation Order

The strict TDD requirement means tests must be written first and must fail, then implementation makes them pass. Here is the ordered sequence:

### Step 1: Test ParseEventType (fails first)

File: `internal/database/eventTypes_test.go` (new file, db-developer scope)

```go
func TestParseEventType(t *testing.T) {
    tests := []struct {
        input    string
        expected EventType
    }{
        {"item.created", ItemCreatedEvent},
        {"item.moved", ItemMovedEvent},
        // ... all 16 entries
    }
    for _, tt := range tests {
        result, err := ParseEventType(tt.input)
        require.NoError(t, err)
        assert.Equal(t, tt.expected, result)
    }

    _, err := ParseEventType("unknown.type")
    assert.Error(t, err)
}
```

**This test fails** because `ParseEventType` does not exist yet.

### Step 2: Test EventType.String() (fails first)

Same file or `eventTypes_string_test.go`:

```go
func TestEventTypeString(t *testing.T) {
    assert.Equal(t, "item.created", ItemCreatedEvent.String())
    assert.Equal(t, "location.reparented", LocationMovedEvent.String())
    // ... key cases
}
```

**This test fails** because `String()` does not exist yet (stringer not run).

### Step 3: Test EventType Value/Scan round-trip (fails first)

File: `internal/database/eventTypes_sql_test.go` (new file, db-developer scope)

```go
func TestEventTypeValuer(t *testing.T) {
    v, err := ItemCreatedEvent.Value()
    require.NoError(t, err)
    assert.Equal(t, "item.created", v)
}

func TestEventTypeScanner(t *testing.T) {
    var et EventType
    require.NoError(t, et.Scan("item.created"))
    assert.Equal(t, ItemCreatedEvent, et)

    assert.Error(t, et.Scan("bogus"))
    assert.Error(t, et.Scan(42)) // wrong type
}
```

**These tests fail** because `Value()` and `Scan()` do not exist yet.

### Step 4: Test Event struct field type (compile-time verification)

Update existing tests in `integration_test.go` that assert on `event.EventType`:

```go
// Before (will still compile because EventType currently is string)
assert.Equal(t, "location.created", evt.EventType)

// After (add alongside or replace)
assert.Equal(t, database.LocationCreatedEvent, evt.EventType)
assert.Equal(t, "location.created", evt.EventType.String())
```

### Step 5: Test database round-trip (integration, fails after struct change)

In `integration_test.go`, existing `GetEventsByType` call uses a string. After the signature changes, it must use a constant:

```go
// Before (fails to compile after signature change)
events, err := db.GetEventsByType(ctx, "location.created")

// After
events, err := db.GetEventsByType(ctx, database.LocationCreatedEvent)
```

### Step 6: Implement in order

1. Add `//go:generate` directive, run `go generate ./internal/database/...` → produces `eventTypes_string.go`
2. Write `ParseEventType` in `eventTypes.go`
3. Write `Value()` and `Scan()` in `eventTypes_sql.go`
4. Change `Event.EventType` field from `string` to `EventType`
5. Change `AppendEvent` and `insertEvent` signatures
6. Change `GetEventsByType` signature
7. Update `eventHandler.go` switch to use constants
8. Update all call sites in `cmd/` and `internal/cli/`
9. Update `mover.go` interface and regenerate mocks

---

## File Change Summary

### New files
- `internal/database/eventTypes_string.go` — generated by stringer (do not edit manually)
- `internal/database/eventTypes_sql.go` — `Value()`, `Scan()`, `ParseEventType()`
- `internal/database/eventTypes_test.go` — unit tests for `String()`, `ParseEventType()`
- `internal/database/eventTypes_sql_test.go` — unit tests for `Value()`, `Scan()`

### Modified files (db-developer scope)
- `internal/database/eventTypes.go` — add `//go:generate` directive
- `internal/database/events.go` — `Event.EventType` field type; `AppendEvent` signature; `GetEventsByType` signature
- `internal/database/eventHandler.go` — switch cases use constants
- `internal/database/helper_test.go` — `insertEvent` signature; all `insertEvent` call sites use constants; `GetEventsByType` call sites use constants
- `internal/database/integration_test.go` — `insertEvent` and `GetEventsByType` call sites; any direct `.EventType` string comparisons

### Modified files (golang-developer scope)
- `internal/cli/add.go` — 1 `AppendEvent` call site

### Modified files (golang-ui-developer scope)
- `cmd/add/location.go` — 1 `AppendEvent` call site
- `cmd/lost/item.go` — 1 `AppendEvent` call site
- `cmd/found/found.go` — 2 `AppendEvent` call sites
- `cmd/loan/item.go` — 1 `AppendEvent` call site
- `cmd/move/item.go` — 1 `AppendEvent` call site
- `cmd/move/mover.go` — interface signature
- `cmd/move/mocks/` — regenerate or hand-update mocks
- `cmd/history/output.go` — string comparisons → constant comparisons; `EventStyle` call gets `.String()`
- `internal/cli/selectors_test.go` — ~13 `AppendEvent` call sites

---

## Agent Routing

| Task | Agent |
|------|-------|
| stringer directive, `ParseEventType`, `Value`, `Scan`, enum constants, `Event` struct field, `AppendEvent`/`insertEvent`/`GetEventsByType` signatures, `eventHandler.go` switch, `eventTypes_test.go`, `eventTypes_sql_test.go`, `integration_test.go`, `helper_test.go` | db-developer |
| `internal/cli/add.go`, `internal/cli/selectors_test.go` | golang-developer |
| `cmd/add/location.go`, `cmd/lost/item.go`, `cmd/found/found.go`, `cmd/loan/item.go`, `cmd/move/item.go`, `cmd/move/mover.go`, `cmd/move/mocks/`, `cmd/history/output.go` | golang-ui-developer |

---

## Trade-offs and Alternatives Considered

### Alternative A: Keep AppendEvent as string, only type the Event struct
- Pro: fewer call-site changes
- Con: callers still use magic strings; the type system cannot catch typos at compile time
- **Rejected**: defeats the purpose of the typed enum

### Alternative B: Add a type alias `type EventType = string`
- Pro: zero call-site changes
- Con: not a real type; provides no compile-time safety
- **Rejected**: does not satisfy the requirement of replacing `EventType string` with a typed int enum

### Alternative C: Keep `EventStyle(key string)` and call `.String()` at call sites
- Pro: no dependency from `styles` package on `database` package; clean layering
- **Accepted**: minimal change, preserves package boundaries

### Alternative D: Store integers in the database
- Con: violates requirement 3 ("database must still store the string representation")
- **Rejected**: explicitly excluded by requirements

---

## Key Constraints

1. The `stringer -linecomment` flag is critical: without it, `ItemCreatedEvent.String()` returns `"ItemCreatedEvent"` not `"item.created"`.
2. `ParseEventType` is the only reverse-mapping; it must be kept in sync with any new enum values added to `eventTypes.go`.
3. The `insertEvent` function is in `helper_test.go` (build tag: only compiled during tests) — it is in the `database` package but is test-only. It must also accept `EventType` to avoid using `string` in test seeding.
4. No migration is needed: the database column type and stored values are unchanged.
