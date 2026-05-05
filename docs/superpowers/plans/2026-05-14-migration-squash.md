# Migration Squash Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Collapse 6 historical migrations into a single migration 001 containing the current schema, keeping golang-migrate and all Go infrastructure intact.

**Architecture:** Delete SQL files 000002–000006 (both up and down). Rewrite 000001 up/down to contain the full current schema. Update tests that hardcode version numbers or repeat rollback calls to reflect the new single-migration state.

**Tech Stack:** golang-migrate/migrate/v4, SQLite, Go testify

---

## File Map

| Action | File |
|--------|------|
| Rewrite | `internal/database/migrations/000001_initial_schema.up.sql` |
| Rewrite | `internal/database/migrations/000001_initial_schema.down.sql` |
| Delete | `internal/database/migrations/000002_add_loaned_system_location.up.sql` |
| Delete | `internal/database/migrations/000002_add_loaned_system_location.down.sql` |
| Delete | `internal/database/migrations/000003_nanoid_migration.up.sql` |
| Delete | `internal/database/migrations/000003_nanoid_migration.down.sql` |
| Delete | `internal/database/migrations/000004_add_removed_system_location.up.sql` |
| Delete | `internal/database/migrations/000004_add_removed_system_location.down.sql` |
| Delete | `internal/database/migrations/000005_remove_project_tables.up.sql` |
| Delete | `internal/database/migrations/000005_remove_project_tables.down.sql` |
| Delete | `internal/database/migrations/000006_entity_consolidation.up.sql` |
| Delete | `internal/database/migrations/000006_entity_consolidation.down.sql` |
| Modify | `internal/database/migrations_test.go` |

No changes to any Go source files outside the test.

---

### Task 1: Rewrite migration 001 up

**Files:**
- Modify: `internal/database/migrations/000001_initial_schema.up.sql`

- [ ] **Step 1: Replace the file contents**

The new file must contain the full current schema (what migration 006 produces
on a fresh database). Overwrite `000001_initial_schema.up.sql` with:

```sql
-- Initial schema for Wherehouse (unified entity model).
-- Migration: 000001

CREATE TABLE events (
    event_id         INTEGER PRIMARY KEY AUTOINCREMENT,
    event_type       TEXT NOT NULL,
    timestamp_utc    TEXT NOT NULL,
    actor_user_id    TEXT NOT NULL,
    payload          TEXT NOT NULL,
    note             TEXT,
    entity_id        TEXT
);

CREATE INDEX idx_events_type ON events(event_type);
CREATE INDEX idx_events_timestamp ON events(timestamp_utc);
CREATE INDEX idx_events_entity_id ON events(entity_id) WHERE entity_id IS NOT NULL;

CREATE TABLE entities_current (
    entity_id           TEXT PRIMARY KEY NOT NULL,
    display_name        TEXT NOT NULL,
    canonical_name      TEXT NOT NULL,
    entity_type         TEXT NOT NULL CHECK (entity_type IN ('place', 'container', 'leaf')),
    parent_id           TEXT,
    full_path_display   TEXT NOT NULL,
    full_path_canonical TEXT NOT NULL,
    depth               INTEGER NOT NULL DEFAULT 0 CHECK (depth >= 0),
    status              TEXT NOT NULL DEFAULT 'ok' CHECK (status IN ('ok', 'borrowed', 'missing', 'loaned', 'removed')),
    status_context      TEXT,
    last_event_id       INTEGER NOT NULL,
    updated_at          TEXT NOT NULL,
    FOREIGN KEY (parent_id) REFERENCES entities_current(entity_id)
);

CREATE INDEX idx_entities_canonical_name ON entities_current(canonical_name);
CREATE INDEX idx_entities_parent_id ON entities_current(parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX idx_entities_status ON entities_current(status);
CREATE INDEX idx_entities_entity_type ON entities_current(entity_type);

CREATE TABLE schema_metadata (
    key    TEXT PRIMARY KEY,
    value  TEXT NOT NULL
);

INSERT INTO schema_metadata (key, value) VALUES
    ('created_at', CURRENT_TIMESTAMP),
    ('app_version', '1.0.0');
```

---

### Task 2: Rewrite migration 001 down

**Files:**
- Modify: `internal/database/migrations/000001_initial_schema.down.sql`

- [ ] **Step 1: Replace the file contents**

The down migration must drop everything the up migration created, in reverse
dependency order (entities_current references itself via parent_id FK, so it
must be dropped before events; schema_metadata has no dependencies).

Overwrite `000001_initial_schema.down.sql` with:

```sql
-- Rollback migration 000001: drop all application tables and indexes.

DROP INDEX IF EXISTS idx_entities_entity_type;
DROP INDEX IF EXISTS idx_entities_status;
DROP INDEX IF EXISTS idx_entities_parent_id;
DROP INDEX IF EXISTS idx_entities_canonical_name;
DROP TABLE IF EXISTS entities_current;
DROP INDEX IF EXISTS idx_events_entity_id;
DROP INDEX IF EXISTS idx_events_timestamp;
DROP INDEX IF EXISTS idx_events_type;
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS schema_metadata;
```

---

### Task 3: Delete migration files 000002–000006

**Files:**
- Delete: all 10 files listed in the file map for 000002–000006

- [ ] **Step 1: Delete the files**

```bash
rm internal/database/migrations/000002_add_loaned_system_location.up.sql
rm internal/database/migrations/000002_add_loaned_system_location.down.sql
rm internal/database/migrations/000003_nanoid_migration.up.sql
rm internal/database/migrations/000003_nanoid_migration.down.sql
rm internal/database/migrations/000004_add_removed_system_location.up.sql
rm internal/database/migrations/000004_add_removed_system_location.down.sql
rm internal/database/migrations/000005_remove_project_tables.up.sql
rm internal/database/migrations/000005_remove_project_tables.down.sql
rm internal/database/migrations/000006_entity_consolidation.up.sql
rm internal/database/migrations/000006_entity_consolidation.down.sql
```

- [ ] **Step 2: Verify only migration 001 remains**

```bash
ls internal/database/migrations/
```

Expected output:
```
000001_initial_schema.down.sql
000001_initial_schema.up.sql
```

---

### Task 4: Update migrations_test.go

**Files:**
- Modify: `internal/database/migrations_test.go`

Three changes needed:

1. `version tracking` test: `assert.EqualValues(t, 6, version)` → `assert.EqualValues(t, 1, version)`
2. `dirty state detection` test: `db.SetMigrationVersion(ctx, 6, true)` → `db.SetMigrationVersion(ctx, 1, true)` and `assert.EqualValues(t, 6, version)` → `assert.EqualValues(t, 1, version)`
3. Both rollback tests: collapse six `require.NoError(t, db.RollbackMigration())` calls into one

- [ ] **Step 1: Fix version 6 → 1 in `version tracking` test**

Find this block in `TestMigrations / "version tracking"`:
```go
		version, dirty, err := db.GetMigrationVersion()
		require.NoError(t, err)
		assert.EqualValues(t, 6, version, "should be at version 6 after all migrations")
		assert.False(t, dirty, "migration should not be dirty")
```

Replace with:
```go
		version, dirty, err := db.GetMigrationVersion()
		require.NoError(t, err)
		assert.EqualValues(t, 1, version, "should be at version 1 after all migrations")
		assert.False(t, dirty, "migration should not be dirty")
```

- [ ] **Step 2: Fix version 6 → 1 in `dirty state detection` test**

Find this block in `TestMigrations / "dirty state detection"`:
```go
		require.NoError(t, db.SetMigrationVersion(ctx, 6, true))

		// Verify dirty state is detected
		version, dirty, err := db.GetMigrationVersion()
		require.NoError(t, err)
		assert.EqualValues(t, 6, version)
		assert.True(t, dirty, "dirty flag should be set")
```

Replace with:
```go
		require.NoError(t, db.SetMigrationVersion(ctx, 1, true))

		// Verify dirty state is detected
		version, dirty, err := db.GetMigrationVersion()
		require.NoError(t, err)
		assert.EqualValues(t, 1, version)
		assert.True(t, dirty, "dirty flag should be set")
```

- [ ] **Step 3: Collapse rollback calls in `TestMigrationRollback / "down migration removes all tables"`**

Find this block:
```go
		// Run rollback six times (we have 6 migrations now)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 6 (entity consolidation)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 5 (remove project tables)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 4 (Removed location)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 3 (nanoid marker)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 2 (Loaned location)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 1 (initial schema)
```

Replace with:
```go
		require.NoError(t, db.RollbackMigration())
```

- [ ] **Step 4: Collapse rollback calls in `TestMigrationRollback / "up after down restores schema"`**

Find this block (the first rollback block in that test):
```go
		// Rollback six times (we have 6 migrations now)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 6 (entity consolidation)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 5 (remove project tables)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 4 (Removed location)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 3 (nanoid marker)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 2 (Loaned location)
		require.NoError(t, db.RollbackMigration()) // Rollback migration 1 (initial schema)
```

Replace with:
```go
		require.NoError(t, db.RollbackMigration())
```

---

### Task 5: Verify

**Files:** none

- [ ] **Step 1: Run the migration tests**

```bash
$(go list ./internal/database/... | grep -v /mocks | grep -v /ai-docs) 
```

Actually run:
```bash
go test ./internal/database/... -run TestMigration -v
```

Expected: all `TestMigrations` and `TestMigrationRollback` subtests PASS.

- [ ] **Step 2: Run the full test suite**

```bash
go test $(go list ./... | grep -v /mocks | grep -v /ai-docs) -count=1
```

Expected: all tests pass.

- [ ] **Step 3: Run lint**

```bash
mise run lint
```

Expected: no errors.

---

### Task 6: Commit

- [ ] **Step 1: Stage changed and deleted files**

```bash
jj file track internal/database/migrations/000001_initial_schema.up.sql
jj file track internal/database/migrations/000001_initial_schema.down.sql
jj file track internal/database/migrations_test.go
```

Note: deleted files are automatically untracked in jujutsu — no need to stage deletions separately.

- [ ] **Step 2: Commit via `/commit` skill**

Run `/commit` and use this message as the basis:

```
refactor(db): squash migrations 001-006 into single initial schema

Replace the six historical migration files with a single 000001 that
contains the full current schema (events + entities_current +
schema_metadata). golang-migrate and all migration methods remain
unchanged. Update migration tests to reflect version 1 and single-step
rollback.
```
