# Refactor 02: `internal/store` Package Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create `internal/store` as the new persistence-only layer — raw SQL reads and writes, connection management, migrations — importing `internal/inventory` for types but containing zero business logic.

**Architecture:** `store` wraps a `*sql.DB` and exposes typed read/write operations. It knows about `inventory` types so it can return them directly from scans, but it never decides *what* to write — that's the caller's job. All business rules, derived events, and validation live in `eventbus` (plan 03). This plan does not delete `internal/database` — that happens in a final cleanup plan after all callers are migrated.

**Tech Stack:** Go 1.25, `modernc.org/sqlite`, `golang-migrate/migrate v4`, `internal/inventory` (plan 01 — must be complete first).

**Prerequisite:** Plan 01 (`internal/inventory`) must be complete.

---

## Target File Map

```
internal/store/
  doc.go          # package doc
  store.go        # Store struct, Config, Open, Close, ExecInTransaction, WithRetry
  events.go       # AppendRawEvent, GetEventByID, GetEventsByEntity, GetAllEvents, GetEventsAfter
  entities.go     # InsertEntity, UpdateEntity, GetEntity, GetEntitiesByCanonicalName, GetChildren, GetDescendants, ListEntities, ComputeEntityPathTx
  migrations.go   # RunMigrations, GetMigrationVersion (reuses existing SQL files)
  metadata.go     # GetMetadata, SetMetadata
  errors.go       # ErrDatabasePathRequired, ErrNotFound
```

The existing `internal/database/migrations/` SQL files are **reused as-is** — this plan does not move or alter them.

---

### Task 1: `Store` connection and transaction helpers

**Files:**
- Create: `internal/store/doc.go`
- Create: `internal/store/store.go`
- Create: `internal/store/errors.go`
- Test: `internal/store/store_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/store/store_test.go
package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/asphaltbuffet/wherehouse/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpen_EmptyPath(t *testing.T) {
	_, err := store.Open(store.Config{})
	assert.ErrorIs(t, err, store.ErrDatabasePathRequired)
}

func TestOpen_ValidPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := store.Open(store.Config{Path: path, AutoMigrate: true})
	require.NoError(t, err)
	require.NotNil(t, s)
	assert.NoError(t, s.Close())
}

func TestExecInTransaction_Commit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := store.Open(store.Config{Path: path, AutoMigrate: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	err = s.ExecInTransaction(context.Background(), func(tx store.Tx) error {
		return nil
	})
	assert.NoError(t, err)
}

func TestExecInTransaction_Rollback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := store.Open(store.Config{Path: path, AutoMigrate: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	sentinelErr := errors.New("rollback me")
	err = s.ExecInTransaction(context.Background(), func(tx store.Tx) error {
		return sentinelErr
	})
	assert.ErrorIs(t, err, sentinelErr)
}
```

Note: add `"errors"` to the import block above.

- [ ] **Step 2: Run test to verify it fails**

```bash
gotestsum -- ./internal/store/...
```

Expected: compilation error — package does not exist.

- [ ] **Step 3: Create `doc.go`**

```go
// Package store provides mechanical SQLite persistence for wherehouse.
// It owns connection management, migrations, and typed SQL read/write operations.
// It contains no business logic — callers decide what to write.
package store
```

- [ ] **Step 4: Create `errors.go`**

```go
package store

import "errors"

var (
	ErrDatabasePathRequired = errors.New("database path is required")
	ErrNotFound             = errors.New("not found")
)
```

- [ ] **Step 5: Create `store.go`**

```go
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	_ "modernc.org/sqlite" // SQLite driver
)

const (
	DefaultBusyTimeout    = 5000
	DefaultBaseRetryDelay = 50 * time.Millisecond
)

// Tx is a handle to an active database transaction.
// Passed to functions running inside ExecInTransaction.
type Tx = *sql.Tx

// Config holds connection parameters for opening a Store.
type Config struct {
	Path        string
	BusyTimeout int
	AutoMigrate bool
}

// Store wraps a SQLite database connection.
type Store struct {
	db  *sql.DB
	cfg Config
}

// Open opens (or creates) the SQLite database at cfg.Path.
func Open(cfg Config) (*Store, error) {
	if cfg.Path == "" {
		return nil, ErrDatabasePathRequired
	}

	if cfg.BusyTimeout == 0 {
		cfg.BusyTimeout = DefaultBusyTimeout
	}

	dsn := fmt.Sprintf("file:%s?_busy_timeout=%d&_journal_mode=WAL&_foreign_keys=on", cfg.Path, cfg.BusyTimeout)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	s := &Store{db: db, cfg: cfg}

	if cfg.AutoMigrate {
		if err := s.RunMigrations(); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("auto-migrate: %w", err)
		}
	}

	return s, nil
}

// Close releases the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB for use with golang-migrate.
func (s *Store) DB() *sql.DB {
	return s.db
}

// ExecInTransaction runs fn inside a transaction, committing on success and rolling back on error.
func (s *Store) ExecInTransaction(ctx context.Context, fn func(Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// WithRetry retries fn on SQLite BUSY/LOCKED errors with jittered backoff.
func (s *Store) WithRetry(ctx context.Context, fn func() error) error {
	delay := DefaultBaseRetryDelay
	for {
		err := fn()
		if err == nil {
			return nil
		}
		if !isRetryableError(err) {
			return err
		}
		jitter := time.Duration(rand.Int64N(int64(delay)))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay + jitter):
		}
		delay *= 2
	}
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return errors.Is(err, context.DeadlineExceeded) ||
		contains(msg, "SQLITE_BUSY") ||
		contains(msg, "SQLITE_LOCKED") ||
		contains(msg, "database is locked")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsRune(s, substr))
}

func containsRune(s, substr string) bool {
	for i := range s {
		if s[i:] >= substr && s[i:len(i)+len(substr)] == substr {
			return true
		}
	}
	return false
}
```

Note: replace the `contains`/`containsRune` helpers with `strings.Contains` — import `"strings"` and use `strings.Contains(msg, "SQLITE_BUSY")` etc. The above is a placeholder showing intent; use stdlib directly.

- [ ] **Step 6: Run tests**

```bash
gotestsum -- ./internal/store/...
```

Expected: PASS (migrations test will fail until Task 2 — that's fine, run only store_test.go for now via `-run TestOpen ./internal/store/...`)

- [ ] **Step 7: Commit**

```bash
jj new -m "feat(store): add Store, Config, ExecInTransaction, WithRetry"
```

---

### Task 2: Migrations

**Files:**
- Create: `internal/store/migrations.go`
- Test: `internal/store/migrations_test.go`

Note: The SQL migration files live in `internal/database/migrations/`. This task adds a symlink or embed directive pointing there. **Do not copy or move the SQL files.**

- [ ] **Step 1: Write the failing test**

```go
// internal/store/migrations_test.go
package store_test

import (
	"path/filepath"
	"testing"

	"github.com/asphaltbuffet/wherehouse/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunMigrations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "migrate.db")
	s, err := store.Open(store.Config{Path: path})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	require.NoError(t, s.RunMigrations())

	ver, dirty, err := s.GetMigrationVersion()
	require.NoError(t, err)
	assert.False(t, dirty)
	assert.Greater(t, ver, uint(0))
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
gotestsum -- -run TestRunMigrations ./internal/store/...
```

Expected: compilation error — RunMigrations undefined.

- [ ] **Step 3: Create `migrations.go`**

The SQL files are embedded from `internal/database/migrations/` using a relative embed path. Because Go `//go:embed` requires the path to be within the package tree, create a minimal shim:

```go
package store

import (
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// migrationsFS embeds the shared SQL migration files.
// The path is relative to this file's location in the module.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS
```

**Important:** The embed directive requires the SQL files to be in `internal/store/migrations/`. Copy (do not move) the SQL files:

```bash
cp -r internal/database/migrations internal/store/migrations
```

Then continue `migrations.go`:

```go
// RunMigrations applies all pending migrations to the database.
func (s *Store) RunMigrations() error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("create migration source: %w", err)
	}

	driver, err := sqlite.WithInstance(s.db, &sqlite.Config{})
	if err != nil {
		return fmt.Errorf("create migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "sqlite", driver)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}

// GetMigrationVersion returns the current schema version and dirty state.
func (s *Store) GetMigrationVersion() (uint, bool, error) {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return 0, false, fmt.Errorf("create migration source: %w", err)
	}

	driver, err := sqlite.WithInstance(s.db, &sqlite.Config{})
	if err != nil {
		return 0, false, fmt.Errorf("create migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "sqlite", driver)
	if err != nil {
		return 0, false, fmt.Errorf("create migrator: %w", err)
	}

	ver, dirty, err := m.Version()
	if err != nil {
		return 0, false, fmt.Errorf("get migration version: %w", err)
	}

	return ver, dirty, nil
}
```

- [ ] **Step 4: Run tests**

```bash
gotestsum -- ./internal/store/...
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
jj new -m "feat(store): add migrations support"
```

---

### Task 3: Event persistence

**Files:**
- Create: `internal/store/events.go`
- Test: `internal/store/events_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/store/events_test.go
package store_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := store.Open(store.Config{Path: path, AutoMigrate: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestAppendRawEvent(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	payload := json.RawMessage(`{"entity_id":"abc","display_name":"Garage","entity_type":"place"}`)
	entityID := "abc"

	eventID, err := s.AppendRawEvent(ctx, inventory.EntityCreatedEvent, "alice", payload, nil, &entityID)
	require.NoError(t, err)
	assert.Greater(t, eventID, int64(0))
}

func TestGetEventByID(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	payload := json.RawMessage(`{"entity_id":"abc","display_name":"Garage","entity_type":"place"}`)
	entityID := "abc"
	eventID, err := s.AppendRawEvent(ctx, inventory.EntityCreatedEvent, "alice", payload, nil, &entityID)
	require.NoError(t, err)

	ev, err := s.GetEventByID(ctx, eventID)
	require.NoError(t, err)
	assert.Equal(t, inventory.EntityCreatedEvent, ev.EventType)
	assert.Equal(t, "alice", ev.ActorUserID)
}

func TestGetEventsByEntity(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	payload := json.RawMessage(`{"entity_id":"abc","display_name":"Garage","entity_type":"place"}`)
	entityID := "abc"
	_, err := s.AppendRawEvent(ctx, inventory.EntityCreatedEvent, "alice", payload, nil, &entityID)
	require.NoError(t, err)

	events, err := s.GetEventsByEntity(ctx, entityID)
	require.NoError(t, err)
	assert.Len(t, events, 1)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
gotestsum -- -run TestAppendRawEvent ./internal/store/...
```

Expected: compilation error — AppendRawEvent undefined.

- [ ] **Step 3: Create `events.go`**

```go
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// AppendRawEvent inserts a pre-constructed event into the events table.
// It does NOT apply projections — that is eventbus's responsibility.
func (s *Store) AppendRawEvent(
	ctx context.Context,
	eventType inventory.EventType,
	actorUserID string,
	payload json.RawMessage,
	note *string,
	entityID *string,
) (int64, error) {
	timestamp := time.Now().UTC().Format(time.RFC3339)

	var eventID int64
	err := s.ExecInTransaction(ctx, func(tx Tx) error {
		const query = `
			INSERT INTO events (event_type, timestamp_utc, actor_user_id, payload, note, entity_id)
			VALUES (?, ?, ?, ?, ?, ?)`

		result, err := tx.ExecContext(ctx, query, eventType, timestamp, actorUserID, string(payload), note, entityID)
		if err != nil {
			return fmt.Errorf("insert event: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("get event ID: %w", err)
		}
		eventID = id
		return nil
	})
	if err != nil {
		return 0, err
	}
	return eventID, nil
}

// GetEventByID retrieves a single event by its ID.
func (s *Store) GetEventByID(ctx context.Context, eventID int64) (*inventory.Event, error) {
	const query = `
		SELECT event_id, event_type, timestamp_utc, actor_user_id, payload, note, entity_id
		FROM events WHERE event_id = ?`

	row := s.db.QueryRowContext(ctx, query, eventID)
	ev, err := scanEvent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return ev, err
}

// GetEventsByEntity retrieves all events for a given entity ID, ordered by event_id ASC.
func (s *Store) GetEventsByEntity(ctx context.Context, entityID string) ([]*inventory.Event, error) {
	const query = `
		SELECT event_id, event_type, timestamp_utc, actor_user_id, payload, note, entity_id
		FROM events WHERE entity_id = ? ORDER BY event_id ASC`

	rows, err := s.db.QueryContext(ctx, query, entityID)
	if err != nil {
		return nil, fmt.Errorf("query events for entity %s: %w", entityID, err)
	}
	defer rows.Close()
	return scanEvents(rows)
}

// GetAllEvents retrieves all events ordered by event_id ASC.
func (s *Store) GetAllEvents(ctx context.Context) ([]*inventory.Event, error) {
	const query = `
		SELECT event_id, event_type, timestamp_utc, actor_user_id, payload, note, entity_id
		FROM events ORDER BY event_id ASC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query all events: %w", err)
	}
	defer rows.Close()
	return scanEvents(rows)
}

// GetEventsAfter retrieves events with event_id > afterID, ordered by event_id ASC.
func (s *Store) GetEventsAfter(ctx context.Context, afterID int64) ([]*inventory.Event, error) {
	const query = `
		SELECT event_id, event_type, timestamp_utc, actor_user_id, payload, note, entity_id
		FROM events WHERE event_id > ? ORDER BY event_id ASC`

	rows, err := s.db.QueryContext(ctx, query, afterID)
	if err != nil {
		return nil, fmt.Errorf("query events after %d: %w", afterID, err)
	}
	defer rows.Close()
	return scanEvents(rows)
}

func scanEvent(row *sql.Row) (*inventory.Event, error) {
	var ev inventory.Event
	err := row.Scan(
		&ev.EventID,
		&ev.EventType,
		&ev.TimestampUTC,
		&ev.ActorUserID,
		&ev.Payload,
		&ev.Note,
		&ev.EntityID,
	)
	if err != nil {
		return nil, err
	}
	return &ev, nil
}

func scanEvents(rows *sql.Rows) ([]*inventory.Event, error) {
	var events []*inventory.Event
	for rows.Next() {
		var ev inventory.Event
		if err := rows.Scan(
			&ev.EventID,
			&ev.EventType,
			&ev.TimestampUTC,
			&ev.ActorUserID,
			&ev.Payload,
			&ev.Note,
			&ev.EntityID,
		); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, &ev)
	}
	return events, rows.Err()
}
```

- [ ] **Step 4: Run tests**

```bash
gotestsum -- ./internal/store/...
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
jj new -m "feat(store): add event persistence (AppendRawEvent, GetEvent*)"
```

---

### Task 4: Entity projection reads and writes

**Files:**
- Create: `internal/store/entities.go`
- Test: `internal/store/entities_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/store/entities_test.go
package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInsertEntity(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	e := &inventory.Entity{
		EntityID:          "e1",
		DisplayName:       "Garage",
		CanonicalName:     "garage",
		EntityType:        inventory.EntityTypePlace,
		ParentID:          nil,
		FullPathDisplay:   "Garage",
		FullPathCanonical: "garage",
		Depth:             0,
		Status:            inventory.EntityStatusOk,
		LastEventID:       1,
		UpdatedAt:         time.Now(),
	}

	err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
		return s.InsertEntityTx(ctx, tx, e)
	})
	require.NoError(t, err)

	got, err := s.GetEntity(ctx, "e1")
	require.NoError(t, err)
	assert.Equal(t, "Garage", got.DisplayName)
	assert.Equal(t, inventory.EntityTypePlace, got.EntityType)
}

func TestGetEntity_NotFound(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	_, err := s.GetEntity(ctx, "nonexistent")
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestGetEntitiesByCanonicalName(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	e := &inventory.Entity{
		EntityID: "e1", DisplayName: "Garage", CanonicalName: "garage",
		EntityType: inventory.EntityTypePlace, FullPathDisplay: "Garage",
		FullPathCanonical: "garage", Status: inventory.EntityStatusOk,
		LastEventID: 1, UpdatedAt: time.Now(),
	}
	err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
		return s.InsertEntityTx(ctx, tx, e)
	})
	require.NoError(t, err)

	results, err := s.GetEntitiesByCanonicalName(ctx, "garage")
	require.NoError(t, err)
	assert.Len(t, results, 1)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
gotestsum -- -run TestInsertEntity ./internal/store/...
```

Expected: compilation error — InsertEntityTx undefined.

- [ ] **Step 3: Create `entities.go`**

```go
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// InsertEntityTx inserts a new entity projection row inside an existing transaction.
func (s *Store) InsertEntityTx(ctx context.Context, tx Tx, e *inventory.Entity) error {
	const query = `
		INSERT INTO entities_current (
			entity_id, display_name, canonical_name, entity_type,
			parent_id, full_path_display, full_path_canonical,
			depth, status, status_context, last_event_id, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := tx.ExecContext(ctx, query,
		e.EntityID, e.DisplayName, e.CanonicalName, e.EntityType,
		e.ParentID, e.FullPathDisplay, e.FullPathCanonical,
		e.Depth, e.Status, e.StatusContext, e.LastEventID,
		e.UpdatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert entity %s: %w", e.EntityID, err)
	}
	return nil
}

// UpdateEntityTx updates an existing entity projection row inside an existing transaction.
func (s *Store) UpdateEntityTx(ctx context.Context, tx Tx, e *inventory.Entity) error {
	const query = `
		UPDATE entities_current SET
			display_name = ?, canonical_name = ?, entity_type = ?,
			parent_id = ?, full_path_display = ?, full_path_canonical = ?,
			depth = ?, status = ?, status_context = ?,
			last_event_id = ?, updated_at = ?
		WHERE entity_id = ?`

	_, err := tx.ExecContext(ctx, query,
		e.DisplayName, e.CanonicalName, e.EntityType,
		e.ParentID, e.FullPathDisplay, e.FullPathCanonical,
		e.Depth, e.Status, e.StatusContext,
		e.LastEventID, e.UpdatedAt.UTC().Format(time.RFC3339),
		e.EntityID,
	)
	if err != nil {
		return fmt.Errorf("update entity %s: %w", e.EntityID, err)
	}
	return nil
}

// GetEntity retrieves a single entity by ID.
func (s *Store) GetEntity(ctx context.Context, entityID string) (*inventory.Entity, error) {
	const query = `
		SELECT entity_id, display_name, canonical_name, entity_type,
		       parent_id, full_path_display, full_path_canonical,
		       depth, status, status_context, last_event_id, updated_at
		FROM entities_current WHERE entity_id = ?`

	row := s.db.QueryRowContext(ctx, query, entityID)
	e, err := scanEntity(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get entity %s: %w", entityID, err)
	}
	return e, nil
}

// GetEntitiesByCanonicalName retrieves all entities with a given canonical name,
// ordered by full_path_canonical ASC, entity_id ASC.
func (s *Store) GetEntitiesByCanonicalName(ctx context.Context, canonical string) ([]*inventory.Entity, error) {
	const query = `
		SELECT entity_id, display_name, canonical_name, entity_type,
		       parent_id, full_path_display, full_path_canonical,
		       depth, status, status_context, last_event_id, updated_at
		FROM entities_current WHERE canonical_name = ?
		ORDER BY full_path_canonical ASC, entity_id ASC`

	rows, err := s.db.QueryContext(ctx, query, canonical)
	if err != nil {
		return nil, fmt.Errorf("query by canonical name %q: %w", canonical, err)
	}
	defer rows.Close()
	return scanEntities(rows)
}

// GetChildren retrieves direct children of a parent entity,
// ordered by display_name ASC, entity_id ASC.
func (s *Store) GetChildren(ctx context.Context, parentID string) ([]*inventory.Entity, error) {
	const query = `
		SELECT entity_id, display_name, canonical_name, entity_type,
		       parent_id, full_path_display, full_path_canonical,
		       depth, status, status_context, last_event_id, updated_at
		FROM entities_current WHERE parent_id = ?
		ORDER BY display_name ASC, entity_id ASC`

	rows, err := s.db.QueryContext(ctx, query, parentID)
	if err != nil {
		return nil, fmt.Errorf("get children of %s: %w", parentID, err)
	}
	defer rows.Close()
	return scanEntities(rows)
}

// GetDescendants retrieves all descendants of a given entity using path prefix matching,
// ordered by depth ASC, display_name ASC, entity_id ASC.
func (s *Store) GetDescendants(ctx context.Context, entityID string) ([]*inventory.Entity, error) {
	parent, err := s.GetEntity(ctx, entityID)
	if err != nil {
		return nil, err
	}

	prefix := parent.FullPathCanonical + ":"
	const query = `
		SELECT entity_id, display_name, canonical_name, entity_type,
		       parent_id, full_path_display, full_path_canonical,
		       depth, status, status_context, last_event_id, updated_at
		FROM entities_current WHERE full_path_canonical LIKE ?
		ORDER BY depth ASC, display_name ASC, entity_id ASC`

	rows, err := s.db.QueryContext(ctx, query, prefix+"%")
	if err != nil {
		return nil, fmt.Errorf("get descendants of %s: %w", entityID, err)
	}
	defer rows.Close()
	return scanEntities(rows)
}

// ListEntities retrieves all entities ordered by full_path_display ASC, entity_id ASC.
func (s *Store) ListEntities(ctx context.Context) ([]*inventory.Entity, error) {
	const query = `
		SELECT entity_id, display_name, canonical_name, entity_type,
		       parent_id, full_path_display, full_path_canonical,
		       depth, status, status_context, last_event_id, updated_at
		FROM entities_current
		ORDER BY full_path_display ASC, entity_id ASC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list entities: %w", err)
	}
	defer rows.Close()
	return scanEntities(rows)
}

// ComputeEntityPathTx computes full_path_display, full_path_canonical, and depth
// for a new entity given its parent. Runs inside an existing transaction.
func (s *Store) ComputeEntityPathTx(ctx context.Context, tx Tx, displayName, canonicalName string, parentID *string) (fullPathDisplay, fullPathCanonical string, depth int, err error) {
	if parentID == nil {
		return displayName, canonicalName, 0, nil
	}

	const query = `
		SELECT full_path_display, full_path_canonical, depth
		FROM entities_current WHERE entity_id = ?`

	var parentDisplay, parentCanonical string
	var parentDepth int
	if err := tx.QueryRowContext(ctx, query, *parentID).Scan(&parentDisplay, &parentCanonical, &parentDepth); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", 0, fmt.Errorf("parent entity %q not found", *parentID)
		}
		return "", "", 0, fmt.Errorf("query parent %s: %w", *parentID, err)
	}

	return strings.Join([]string{parentDisplay, displayName}, ":"),
		strings.Join([]string{parentCanonical, canonicalName}, ":"),
		parentDepth + 1,
		nil
}

type scanFunc func(dest ...any) error

func scanEntity(scan scanFunc) (*inventory.Entity, error) {
	var e inventory.Entity
	var updatedAtStr string
	if err := scan(
		&e.EntityID, &e.DisplayName, &e.CanonicalName, &e.EntityType,
		&e.ParentID, &e.FullPathDisplay, &e.FullPathCanonical,
		&e.Depth, &e.Status, &e.StatusContext, &e.LastEventID, &updatedAtStr,
	); err != nil {
		return nil, err
	}
	t, err := time.Parse(time.RFC3339, updatedAtStr)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at %q: %w", updatedAtStr, err)
	}
	e.UpdatedAt = t
	return &e, nil
}

func scanEntities(rows *sql.Rows) ([]*inventory.Entity, error) {
	var entities []*inventory.Entity
	for rows.Next() {
		e, err := scanEntity(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("scan entity: %w", err)
		}
		entities = append(entities, e)
	}
	return entities, rows.Err()
}
```

- [ ] **Step 4: Run tests**

```bash
gotestsum -- ./internal/store/...
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
jj new -m "feat(store): add entity projection reads and writes"
```

---

### Task 5: Metadata and final lint

**Files:**
- Create: `internal/store/metadata.go`
- Test: `internal/store/metadata_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/store/metadata_test.go
package store_test

import (
	"context"
	"testing"

	"github.com/asphaltbuffet/wherehouse/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadataRoundtrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.SetMetadata(ctx, "schema_version", "1"))

	val, err := s.GetMetadata(ctx, "schema_version")
	require.NoError(t, err)
	assert.Equal(t, "1", val)
}

func TestGetMetadata_Missing(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	_, err := s.GetMetadata(ctx, "nonexistent")
	assert.ErrorIs(t, err, store.ErrNotFound)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
gotestsum -- -run TestMetadata ./internal/store/...
```

Expected: compilation error.

- [ ] **Step 3: Create `metadata.go`**

```go
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// GetMetadata retrieves a value from the schema_metadata table by key.
func (s *Store) GetMetadata(ctx context.Context, key string) (string, error) {
	const query = `SELECT value FROM schema_metadata WHERE key = ?`
	var val string
	err := s.db.QueryRowContext(ctx, query, key).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get metadata %q: %w", key, err)
	}
	return val, nil
}

// SetMetadata upserts a key/value pair in the schema_metadata table.
func (s *Store) SetMetadata(ctx context.Context, key, value string) error {
	const query = `
		INSERT INTO schema_metadata (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`
	_, err := s.db.ExecContext(ctx, query, key, value)
	if err != nil {
		return fmt.Errorf("set metadata %q: %w", key, err)
	}
	return nil
}
```

- [ ] **Step 4: Run all store tests**

```bash
gotestsum -- ./internal/store/...
```

Expected: PASS

- [ ] **Step 5: Run lint**

```bash
mise run lint
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
jj new -m "feat(store): add metadata persistence; complete store package"
```
