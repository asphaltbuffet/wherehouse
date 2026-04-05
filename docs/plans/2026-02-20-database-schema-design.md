# Database Schema Design

> **Status:** Approved by db-developer agent (agent ID: a438aa9)
> **Created:** 2026-02-20
> **Author:** Design collaboration
> **Review Status:** ✅ Production-ready

## Executive Summary

This document defines the SQLite database schema for wherehouse's event-sourced inventory system. The design follows a minimal/pragmatic approach with embedded schema definitions, explicit transaction boundaries, and raw SQL for clarity.

**Key Decisions:**
- **Database**: SQLite with modernc.org/sqlite (pure Go, no CGo)
- **Event storage**: Hybrid approach - JSON payload (source of truth) + indexed entity ID columns (for efficient history queries)
- **Connection strategy**: Single connection with WAL mode
- **Schema versioning**: Embedded DDL with version tracking
- **Transaction scope**: Event insertion + projection update (atomic)

**Trade-off Documented:**
- **V1**: Embedded schema in Go, single version tracking
- **Future migration cost**: Adding schema v2 requires building migration framework
- **Acceptable because**: Event sourcing allows projection rebuilds; event table only needs additive changes

---

## 1. Core Tables

### 1.1 Events Table (Append-Only Log)

**Purpose**: Immutable event log, source of truth for all state changes

```sql
CREATE TABLE events (
  event_id         INTEGER PRIMARY KEY AUTOINCREMENT,
  event_type       TEXT NOT NULL,
  timestamp_utc    TEXT NOT NULL,  -- RFC3339 with Z
  actor_user_id    TEXT NOT NULL,
  payload          TEXT NOT NULL,  -- JSON object with event-specific fields (source of truth)
  note             TEXT,            -- Optional free-text annotation

  -- Indexed entity ID columns (extracted from payload for query performance)
  item_id          TEXT,            -- For item.* events
  location_id      TEXT,            -- For location.* events
  project_id       TEXT             -- For project.* events
);

-- Query indexes
CREATE INDEX idx_events_type ON events(event_type);
CREATE INDEX idx_events_timestamp ON events(timestamp_utc);

-- Entity history indexes (partial indexes for efficiency)
CREATE INDEX idx_events_item_id ON events(item_id) WHERE item_id IS NOT NULL;
CREATE INDEX idx_events_location_id ON events(location_id) WHERE location_id IS NOT NULL;
CREATE INDEX idx_events_project_id ON events(project_id) WHERE project_id IS NOT NULL;
```

**Design Notes:**
- `event_id` (integer autoincrement) defines deterministic replay ordering
- `timestamp_utc` is informational only, not used for ordering
- `payload` contains all event data as JSON (single source of truth)
- **Hybrid approach**: Entity IDs extracted to separate columns for efficient history queries
- Partial indexes only index non-NULL entity IDs (space efficient)
- No foreign keys from events table (events are immutable history)

**Payload JSON Examples:**

```json
// item.created
{
  "item_id": "uuid-v7",
  "display_name": "10mm Socket Wrench",
  "canonical_name": "10mm_socket_wrench",
  "location_id": "uuid"
}

// item.moved
{
  "item_id": "uuid",
  "from_location_id": "uuid",
  "to_location_id": "uuid",
  "move_type": "temporary_use",
  "project_action": "clear",
  "project_id": null
}

// location.created
{
  "location_id": "uuid-v7",
  "display_name": "Garage",
  "canonical_name": "garage",
  "parent_id": null,
  "is_system": false
}

// project.created
{
  "project_id": "weekend-cleanup"
}
```

### 1.2 Schema Metadata Table

**Purpose**: Track schema version for future migrations

```sql
CREATE TABLE schema_metadata (
  key    TEXT PRIMARY KEY,
  value  TEXT NOT NULL
);

-- Initial seed data
INSERT INTO schema_metadata (key, value) VALUES
  ('schema_version', '1'),
  ('created_at', datetime('now'));
```

---

## 2. Projection Tables

### 2.1 Locations Projection

**Purpose**: Current location hierarchy with cached paths for fast queries

```sql
CREATE TABLE locations_current (
  location_id           TEXT PRIMARY KEY,
  display_name          TEXT NOT NULL,
  canonical_name        TEXT NOT NULL UNIQUE,
  parent_id             TEXT,
  full_path_display     TEXT NOT NULL,
  full_path_canonical   TEXT NOT NULL,
  depth                 INTEGER NOT NULL,
  is_system             INTEGER NOT NULL DEFAULT 0,  -- SQLite boolean (0/1)
  updated_at            TEXT NOT NULL,

  FOREIGN KEY (parent_id) REFERENCES locations_current(location_id)
);

CREATE INDEX idx_locations_parent ON locations_current(parent_id);
CREATE INDEX idx_locations_canonical ON locations_current(canonical_name);
CREATE INDEX idx_locations_path_canonical ON locations_current(full_path_canonical);
CREATE INDEX idx_locations_system ON locations_current(is_system) WHERE is_system = 1;
```

**Cached Fields:**
- `full_path_display`: User-facing path (e.g., `"Garage >> Shelf A >> Tote F"`)
- `full_path_canonical`: Normalized path for matching (e.g., `"garage:shelf_a:tote_f"`)
- `depth`: Distance from root (0 = root location)

**System Locations** (seeded during initialization):
- `Missing` (UUID assigned, `is_system=1`)
- `Borrowed` (UUID assigned, `is_system=1`)

### 2.2 Items Projection

**Purpose**: Current item state and locations

```sql
CREATE TABLE items_current (
  item_id                  TEXT PRIMARY KEY,
  display_name             TEXT NOT NULL,
  canonical_name           TEXT NOT NULL,  -- NOT unique (duplicates allowed)
  location_id              TEXT NOT NULL,
  in_temporary_use         INTEGER NOT NULL DEFAULT 0,
  temp_origin_location_id  TEXT,
  project_id               TEXT,
  last_event_id            INTEGER NOT NULL,
  updated_at               TEXT NOT NULL,

  FOREIGN KEY (location_id) REFERENCES locations_current(location_id),
  FOREIGN KEY (temp_origin_location_id) REFERENCES locations_current(location_id),
  FOREIGN KEY (project_id) REFERENCES projects_current(project_id)
);

CREATE INDEX idx_items_location ON items_current(location_id);
CREATE INDEX idx_items_canonical ON items_current(canonical_name);
CREATE INDEX idx_items_project ON items_current(project_id) WHERE project_id IS NOT NULL;
CREATE INDEX idx_items_temp_use ON items_current(in_temporary_use) WHERE in_temporary_use = 1;
CREATE INDEX idx_items_canonical_location ON items_current(canonical_name, location_id);
CREATE INDEX idx_items_location_covering ON items_current(location_id, display_name, canonical_name);
```

**Temporary Use State Machine:**
- **First temporary move**: `in_temporary_use=1`, capture `temp_origin_location_id`
- **Subsequent temporary moves**: Preserve original `temp_origin_location_id`
- **Rehome move**: Clear both fields (`in_temporary_use=0`, `temp_origin_location_id=NULL`)

### 2.3 Projects Projection

**Purpose**: Current project status

```sql
CREATE TABLE projects_current (
  project_id     TEXT PRIMARY KEY,
  status         TEXT NOT NULL CHECK (status IN ('active', 'completed')),
  updated_at     TEXT NOT NULL
);

CREATE INDEX idx_projects_status ON projects_current(status);
```

**Simple Design**: Projects are slugs with status in v1 (no additional metadata)

---

## 3. Connection Management

### 3.1 SQLite Configuration

**Required PRAGMAs** (executed on every connection):

```go
const sqlitePragmas = `
PRAGMA journal_mode = WAL;           -- Write-Ahead Logging for concurrency
PRAGMA foreign_keys = ON;            -- Enforce foreign key constraints
PRAGMA synchronous = NORMAL;         -- Balance durability/performance
PRAGMA busy_timeout = 30000;         -- 30 seconds for lock contention
PRAGMA wal_autocheckpoint = 1000;    -- Checkpoint every 1000 pages
`
```

**Rationale:**
- **WAL mode**: Concurrent reads during writes, network-mount compatible
- **foreign_keys=ON**: Critical for referential integrity (OFF by default in SQLite!)
- **synchronous=NORMAL**: Safe for WAL, faster than FULL on network storage
- **busy_timeout=30000**: Long timeout for network-mounted databases
- **wal_autocheckpoint**: Prevents unbounded WAL file growth

### 3.2 Connection Setup

```go
type Database struct {
    db   *sql.DB
    path string
}

func Open(dbPath string) (*Database, error) {
    // 1. Validate absolute path (per design spec)
    if !filepath.IsAbs(dbPath) {
        return nil, fmt.Errorf("database path must be absolute: %s", dbPath)
    }

    // 2. Open connection (modernc.org/sqlite)
    db, err := sql.Open("sqlite", dbPath)
    if err != nil {
        return nil, fmt.Errorf("open database: %w", err)
    }

    // 3. Apply PRAGMAs
    if _, err := db.Exec(sqlitePragmas); err != nil {
        db.Close()
        return nil, fmt.Errorf("apply pragmas: %w", err)
    }

    // 4. Initialize schema if needed
    if err := ensureSchema(db); err != nil {
        db.Close()
        return nil, fmt.Errorf("ensure schema: %w", err)
    }

    return &Database{db: db, path: dbPath}, nil
}
```

**Connection Strategy:**
- **Single connection**: SQLite serializes writes; pooling provides no benefit
- **Auto-initialization**: Create schema on first run if not exists
- **Version validation**: Fail fast on schema version mismatch

### 3.3 Schema Initialization

```go
func ensureSchema(db *sql.DB) error {
    // Check if schema exists
    var version string
    err := db.QueryRow("SELECT value FROM schema_metadata WHERE key='schema_version'").Scan(&version)

    if err == sql.ErrNoRows {
        // Schema doesn't exist - create it
        return initializeSchema(db)
    } else if err != nil {
        return fmt.Errorf("check schema version: %w", err)
    }

    // Schema exists - verify version
    if version != "1" {
        return fmt.Errorf("unsupported schema version: %s (expected 1)", version)
    }

    return nil
}

func initializeSchema(db *sql.DB) error {
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // 1. Create tables (schema_metadata, events, locations_current, items_current, projects_current)
    // 2. Seed system locations (Missing, Borrowed) with generated UUIDs
    // 3. Insert schema_metadata

    return tx.Commit()
}
```

**Initialization Strategy:**
- Auto-create on first open if not exists
- Seed system locations during init
- All DDL in single transaction (atomic)
- Fail fast on version mismatch

---

## 4. Transaction Boundaries

### 4.1 Transaction Strategy

**Core Principle**: Event insertion + projection update must be atomic

```go
func (db *Database) CreateItem(ctx context.Context, event ItemCreatedEvent) error {
    tx, err := db.db.BeginTx(ctx, &sql.TxOptions{
        Isolation: sql.LevelDefault,  // SQLite uses SERIALIZABLE
    })
    if err != nil {
        return fmt.Errorf("begin transaction: %w", err)
    }
    defer tx.Rollback()  // Safe to call after commit

    // 1. Validate (read projection state)
    if err := validateItemCreation(tx, event); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }

    // 2. Insert event
    eventID, err := insertEvent(tx, "item.created", event)
    if err != nil {
        return fmt.Errorf("insert event: %w", err)
    }

    // 3. Update projection
    if err := applyItemCreatedProjection(tx, event, eventID); err != nil {
        return fmt.Errorf("update projection: %w", err)
    }

    // 4. Commit
    return tx.Commit()
}
```

**Transaction Scope**: Entire command operation (validate → event → projection)

**Isolation Level**: SQLite default (SERIALIZABLE) - strongest guarantee

### 4.2 Event Insertion Pattern

```go
func insertEvent(tx *sql.Tx, eventType string, payload interface{}) (int64, error) {
    // Marshal payload to JSON
    payloadJSON, err := json.Marshal(payload)
    if err != nil {
        return 0, fmt.Errorf("marshal payload: %w", err)
    }

    // Extract common fields
    note := extractNote(payload)
    userID := getCurrentUserFromContext()

    // Extract entity IDs for indexing (based on event type)
    itemID := extractItemID(payload)
    locationID := extractLocationID(payload)
    projectID := extractProjectID(payload)

    // Insert event with payload + indexed entity IDs
    result, err := tx.Exec(`
        INSERT INTO events (
            event_type, timestamp_utc, actor_user_id, payload, note,
            item_id, location_id, project_id
        )
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `,
        eventType,
        time.Now().UTC().Format(time.RFC3339),
        userID,
        string(payloadJSON),
        note,
        nullString(itemID),
        nullString(locationID),
        nullString(projectID),
    )
    if err != nil {
        return 0, fmt.Errorf("insert event: %w", err)
    }

    eventID, err := result.LastInsertId()
    if err != nil {
        return 0, fmt.Errorf("get event ID: %w", err)
    }

    return eventID, nil
}

// Helper to extract entity IDs from payload
func extractItemID(payload interface{}) string {
    type hasItemID interface{ GetItemID() string }
    if p, ok := payload.(hasItemID); ok {
        return p.GetItemID()
    }
    return ""
}

func extractLocationID(payload interface{}) string {
    type hasLocationID interface{ GetLocationID() string }
    if p, ok := payload.(hasLocationID); ok {
        return p.GetLocationID()
    }
    return ""
}

func extractProjectID(payload interface{}) string {
    type hasProjectID interface{ GetProjectID() string }
    if p, ok := payload.(hasProjectID); ok {
        return p.GetProjectID()
    }
    return ""
}

func nullString(s string) sql.NullString {
    return sql.NullString{String: s, Valid: s != ""}
}
```

**Event Storage:**
- Timestamps generated at insertion (UTC, RFC3339)
- Payload marshaled to JSON (source of truth)
- Entity IDs extracted and stored in indexed columns (for efficient queries)
- Returns `event_id` for projection tracking

**Hybrid Storage Benefit**: Enables efficient history queries (`SELECT * FROM events WHERE item_id = ?`) without JSON parsing

### 4.3 Projection Update Pattern

**Example: item.created**
```go
func applyItemCreatedProjection(tx *sql.Tx, event ItemCreatedEvent, eventID int64) error {
    _, err := tx.Exec(`
        INSERT INTO items_current (
            item_id, display_name, canonical_name, location_id,
            in_temporary_use, temp_origin_location_id, project_id,
            last_event_id, updated_at
        ) VALUES (?, ?, ?, ?, 0, NULL, NULL, ?, ?)
    `,
        event.ItemID,
        event.DisplayName,
        event.CanonicalName,
        event.LocationID,
        eventID,
        time.Now().UTC().Format(time.RFC3339),
    )

    return err
}
```

**Example: item.moved (with state machine logic)**
```go
func applyItemMovedProjection(tx *sql.Tx, event ItemMovedEvent, eventID int64) error {
    // Read current state for temporary use logic
    var item struct {
        InTemporaryUse       bool
        TempOriginLocationID sql.NullString
        CurrentProjectID     sql.NullString
    }

    err := tx.QueryRow(`
        SELECT in_temporary_use, temp_origin_location_id, project_id
        FROM items_current WHERE item_id = ?
    `, event.ItemID).Scan(&item.InTemporaryUse, &item.TempOriginLocationID, &item.CurrentProjectID)
    if err != nil {
        return fmt.Errorf("read item state: %w", err)
    }

    // Compute new temporary use state
    var newInTempUse bool
    var newTempOrigin sql.NullString

    if event.MoveType == "temporary_use" {
        newInTempUse = true
        if !item.InTemporaryUse {
            // First temporary move - capture origin
            newTempOrigin = sql.NullString{String: event.FromLocationID, Valid: true}
        } else {
            // Subsequent temporary move - preserve origin
            newTempOrigin = item.TempOriginLocationID
        }
    } else { // "rehome"
        newInTempUse = false
        newTempOrigin = sql.NullString{Valid: false}
    }

    // Compute project_id based on project_action
    newProjectID := computeProjectID(event.ProjectAction, event.ProjectID, item.CurrentProjectID)

    // Update projection
    _, err = tx.Exec(`
        UPDATE items_current
        SET location_id = ?,
            in_temporary_use = ?,
            temp_origin_location_id = ?,
            project_id = ?,
            last_event_id = ?,
            updated_at = ?
        WHERE item_id = ?
    `,
        event.ToLocationID,
        newInTempUse,
        newTempOrigin,
        newProjectID,
        eventID,
        time.Now().UTC().Format(time.RFC3339),
        event.ItemID,
    )

    return err
}

func computeProjectID(action string, eventProjectID string, currentProjectID sql.NullString) sql.NullString {
    switch action {
    case "clear":
        return sql.NullString{Valid: false}
    case "keep":
        return currentProjectID
    case "set":
        return sql.NullString{String: eventProjectID, Valid: true}
    default:
        return sql.NullString{Valid: false}  // Default: clear
    }
}
```

---

## 5. Validation & Replay

### 5.1 Pre-Insert Validation

**Philosophy**: Validate before event insertion (events are immutable)

**Example: item.moved validation**
```go
func validateItemMoved(tx *sql.Tx, event ItemMovedEvent) error {
    // 1. Item must exist
    var currentLocationID string
    err := tx.QueryRow(`
        SELECT location_id FROM items_current WHERE item_id = ?
    `, event.ItemID).Scan(&currentLocationID)
    if err == sql.ErrNoRows {
        return fmt.Errorf("item %s does not exist", event.ItemID)
    } else if err != nil {
        return fmt.Errorf("read item: %w", err)
    }

    // 2. CRITICAL: from_location_id must match projection
    if currentLocationID != event.FromLocationID {
        return fmt.Errorf(
            "location mismatch: projection has %s, event expects %s "+
            "(possible concurrent modification or projection corruption)",
            currentLocationID, event.FromLocationID,
        )
    }

    // 3. Target location must exist
    var exists bool
    err = tx.QueryRow(`
        SELECT EXISTS(SELECT 1 FROM locations_current WHERE location_id = ?)
    `, event.ToLocationID).Scan(&exists)
    if err != nil {
        return fmt.Errorf("check target location: %w", err)
    }
    if !exists {
        return fmt.Errorf("target location %s does not exist", event.ToLocationID)
    }

    // 4. Cannot move to same location
    if event.FromLocationID == event.ToLocationID {
        return fmt.Errorf("cannot move item to same location")
    }

    // 5. If project_action="set", project must exist and be active
    if event.ProjectAction == "set" {
        var status string
        err = tx.QueryRow(`
            SELECT status FROM projects_current WHERE project_id = ?
        `, event.ProjectID).Scan(&status)
        if err == sql.ErrNoRows {
            return fmt.Errorf("project %s does not exist", event.ProjectID)
        } else if err != nil {
            return fmt.Errorf("read project: %w", err)
        }
        if status != "active" {
            return fmt.Errorf("project %s is not active (status: %s)", event.ProjectID, status)
        }
    }

    return nil
}
```

**Validation Principles:**
- Read projection state to validate constraints
- **Critical check**: `from_location_id` must match (detects corruption/concurrency)
- Fail fast with descriptive errors
- No silent repair

### 5.2 Projection Rebuild (Doctor Command)

```go
func (db *Database) RebuildProjections(ctx context.Context) error {
    tx, err := db.db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("begin transaction: %w", err)
    }
    defer tx.Rollback()

    // 1. Clear existing projections (preserve system locations)
    if _, err := tx.Exec("DELETE FROM items_current"); err != nil {
        return fmt.Errorf("clear items: %w", err)
    }
    if _, err := tx.Exec("DELETE FROM locations_current WHERE is_system = 0"); err != nil {
        return fmt.Errorf("clear locations: %w", err)
    }
    if _, err := tx.Exec("DELETE FROM projects_current"); err != nil {
        return fmt.Errorf("clear projects: %w", err)
    }

    // 2. Replay all events in order
    rows, err := tx.Query(`
        SELECT event_id, event_type, payload, timestamp_utc
        FROM events
        ORDER BY event_id ASC
    `)
    if err != nil {
        return fmt.Errorf("query events: %w", err)
    }
    defer rows.Close()

    for rows.Next() {
        var eventID int64
        var eventType string
        var payloadJSON string
        var timestampUTC string

        if err := rows.Scan(&eventID, &eventType, &payloadJSON, &timestampUTC); err != nil {
            return fmt.Errorf("scan event: %w", err)
        }

        // 3. Apply projection update based on event type
        if err := applyEventToProjection(tx, eventID, eventType, payloadJSON, timestampUTC); err != nil {
            return fmt.Errorf("replay failed at event_id=%d: %w", eventID, err)
        }
    }

    if err := rows.Err(); err != nil {
        return fmt.Errorf("iterate events: %w", err)
    }

    // 4. Commit rebuilt projections
    return tx.Commit()
}

func applyEventToProjection(tx *sql.Tx, eventID int64, eventType, payloadJSON, timestampUTC string) error {
    switch eventType {
    case "item.created":
        var event ItemCreatedEvent
        if err := json.Unmarshal([]byte(payloadJSON), &event); err != nil {
            return fmt.Errorf("unmarshal payload: %w", err)
        }
        return applyItemCreatedProjection(tx, event, eventID)

    case "item.moved":
        var event ItemMovedEvent
        if err := json.Unmarshal([]byte(payloadJSON), &event); err != nil {
            return fmt.Errorf("unmarshal payload: %w", err)
        }
        // During replay: trust events (already validated at insertion)
        // Still validate projection consistency (from_location_id check)
        return applyItemMovedProjection(tx, event, eventID)

    case "location.created":
        var event LocationCreatedEvent
        if err := json.Unmarshal([]byte(payloadJSON), &event); err != nil {
            return fmt.Errorf("unmarshal payload: %w", err)
        }
        return applyLocationCreatedProjection(tx, event, eventID)

    // ... other event types ...

    default:
        return fmt.Errorf("unknown event type: %s", eventType)
    }
}
```

**Replay Strategy:**
- **Strict ordering**: `ORDER BY event_id ASC` (deterministic)
- **Clear projections**: Delete non-system data before replay
- **Trust events**: Don't re-validate business rules (already validated)
- **Validate consistency**: Still check `from_location_id` matches (detects corruption)
- **Fail fast**: Stop on first error with diagnostic info
- **Atomic**: All in one transaction

---

## 6. Error Handling & Testing

### 6.1 Error Handling

**Custom Error Types:**
```go
var (
    ErrNotFound               = errors.New("entity not found")
    ErrAlreadyExists          = errors.New("entity already exists")
    ErrValidation             = errors.New("validation failed")
    ErrConcurrentModification = errors.New("concurrent modification detected")
    ErrIntegrity              = errors.New("integrity constraint violation")
)
```

**Error Wrapping:**
```go
func (db *Database) CreateItem(ctx context.Context, event ItemCreatedEvent) error {
    // ... operation ...
    if err != nil {
        return fmt.Errorf("create item %s: %w", event.ItemID, err)
    }
    return nil
}
```

**SQLite Retry Logic:**
```go
func isBusyError(err error) bool {
    var sqliteErr *sqlite.Error
    if errors.As(err, &sqliteErr) {
        return sqliteErr.Code() == sqlite.SQLITE_BUSY
    }
    return false
}

func withRetry(ctx context.Context, maxRetries int, fn func() error) error {
    var lastErr error
    for i := 0; i < maxRetries; i++ {
        if err := fn(); err != nil {
            if isBusyError(err) && i < maxRetries-1 {
                // Exponential backoff
                time.Sleep(time.Duration(50*(1<<i)) * time.Millisecond)
                lastErr = err
                continue
            }
            return err
        }
        return nil
    }
    return fmt.Errorf("max retries exceeded: %w", lastErr)
}
```

### 6.2 Testing Strategy

**Test Organization:**
```
internal/database/
├── schema_test.go          # Schema creation, version checks
├── events_test.go          # Event insertion, JSON marshaling
├── projections_test.go     # Projection CRUD operations
├── replay_test.go          # Replay logic, consistency checks
├── validation_test.go      # Validation rules
└── integration_test.go     # End-to-end workflows
```

**Test Patterns:**
```go
// In-memory SQLite for tests
func setupTestDB(t *testing.T) *Database {
    db, err := Open(":memory:")
    require.NoError(t, err)
    t.Cleanup(func() { db.Close() })
    return db
}

// Test event + projection atomicity
func TestCreateItem(t *testing.T) {
    db := setupTestDB(t)
    loc := createTestLocation(t, db, "Garage")

    event := ItemCreatedEvent{
        ItemID: uuid.NewString(),
        DisplayName: "10mm Socket",
        CanonicalName: "10mm_socket",
        LocationID: loc.LocationID,
    }

    err := db.CreateItem(context.Background(), event)
    require.NoError(t, err)

    // Verify projection
    item, err := db.GetItem(event.ItemID)
    require.NoError(t, err)
    assert.Equal(t, event.DisplayName, item.DisplayName)

    // Verify event stored
    events, err := db.GetItemHistory(event.ItemID)
    require.NoError(t, err)
    assert.Len(t, events, 1)
}

// Test validation prevents invalid events
func TestCreateItem_ValidationError(t *testing.T) {
    db := setupTestDB(t)

    event := ItemCreatedEvent{
        ItemID: uuid.NewString(),
        LocationID: "nonexistent",  // Invalid
    }

    err := db.CreateItem(context.Background(), event)
    assert.Error(t, err)

    // Verify event NOT stored (rollback)
    events, _ := db.GetItemHistory(event.ItemID)
    assert.Empty(t, events)
}

// Test replay consistency
func TestRebuildProjections_Consistency(t *testing.T) {
    db := setupTestDB(t)
    setupTestData(t, db)

    original := snapshotProjections(t, db)

    err := db.RebuildProjections(context.Background())
    require.NoError(t, err)

    rebuilt := snapshotProjections(t, db)
    assert.Equal(t, original, rebuilt)
}

// Test concurrent modification detection
func TestItemMoved_ConcurrentModification(t *testing.T) {
    db := setupTestDB(t)
    item := createTestItem(t, db)

    // Move item
    moveItem(t, db, item.ID, "garage", "workshop")

    // Attempt move with stale from_location_id
    event := ItemMovedEvent{
        ItemID: item.ID,
        FromLocationID: "garage",  // Stale
        ToLocationID: "workshop",
    }

    err := db.MoveItem(context.Background(), event)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "location mismatch")
}
```

**Coverage Targets:**
- ✅ Schema initialization and version checks
- ✅ All event types (insert + projection)
- ✅ All validation rules
- ✅ Replay/rebuild logic
- ✅ Concurrent modification detection
- ✅ Foreign key enforcement
- ✅ Temporary use state machine
- ✅ Path recomputation (location reparenting)
- ✅ End-to-end workflows

---

## 7. File Organization

```
internal/database/
├── database.go            # Database struct, Open(), Close()
├── schema.go              # Embedded DDL statements, initializeSchema()
├── connection.go          # Connection setup, PRAGMAs
├── events.go              # insertEvent(), event CRUD
├── items.go               # Item operations (CreateItem, MoveItem, etc.)
├── locations.go           # Location operations
├── projects.go            # Project operations
├── projections.go         # Projection update helpers
├── replay.go              # RebuildProjections(), applyEventToProjection()
├── validation.go          # Validation helpers
├── errors.go              # Custom error types
└── *_test.go              # Tests for each file
```

---

## 8. Key Design Principles

1. **Event Immutability**: Events never modified after insertion
2. **Atomic Operations**: Event + projection in single transaction
3. **Explicit Validation**: Validate before event insertion (fail fast)
4. **Deterministic Replay**: Strict `event_id` ordering
5. **No Silent Repair**: Validation failures stop operation
6. **Projection Consistency**: `from_location_id` validation critical
7. **Simple Over Complex**: Raw SQL, explicit transactions, minimal abstractions

---

## 9. Future Migration Path

When schema v2 is needed:

1. **Add migration framework** (golang-migrate or custom)
2. **Migration files**: `internal/database/migrations/00X_description.sql`
3. **Event table**: Only additive changes (new event types, optional fields)
4. **Projection tables**: Can be altered freely (rebuild from events)
5. **Version check**: Update `schema_version` after successful migration

**Current trade-off**: V1 uses embedded schema for simplicity. Migration complexity deferred until needed.

---

## Review Checklist (for db-developer)

- [ ] Schema design follows event-sourcing principles
- [ ] Indexes cover all critical query patterns
- [ ] Foreign keys properly defined and enforced
- [ ] Transaction boundaries are correct and atomic
- [ ] Validation logic is complete and correct
- [ ] Replay logic handles all event types
- [ ] Temporary use state machine is correct
- [ ] Path recomputation logic is sound
- [ ] Error handling is appropriate
- [ ] Testing strategy is comprehensive
- [ ] SQLite PRAGMAs are correct for use case
- [ ] JSON payload structure is appropriate
- [ ] No SQL injection vulnerabilities
- [ ] Performance considerations addressed

---

**Next Steps:**
1. db-developer agent review
2. Address feedback
3. Commit design document
4. Create implementation plan (writing-plans skill)
