# Migration Squash Design

**Date**: 2026-05-14
**Status**: Approved

## Goal

Collapse all 6 historical database migrations into a single migration 001 that
reflects the current schema. golang-migrate, all migration methods, and all
call sites remain unchanged. There are no existing production databases to
preserve.

## Approach

Option A (squash-and-replace): delete migrations 000002–000006, rewrite
000001 to contain the current schema in full.

## Schema Changes

### `000001_initial_schema.up.sql` (rewritten)

Contains the full current schema — equivalent to what migration 006 leaves
behind after a fresh apply of all six migrations:

- `events` table with columns: `event_id`, `event_type`, `timestamp_utc`,
  `actor_user_id`, `payload`, `note`, `entity_id` (no legacy `item_id`,
  `location_id`, or `project_id` columns)
- Indexes on `events`: `idx_events_type`, `idx_events_timestamp`,
  `idx_events_entity_id`
- `entities_current` table with columns: `entity_id`, `display_name`,
  `canonical_name`, `entity_type` (CHECK: place/container/leaf), `parent_id`,
  `full_path_display`, `full_path_canonical`, `depth`, `status`
  (CHECK: ok/borrowed/missing/loaned/removed), `status_context`,
  `last_event_id`, `updated_at`, FK `parent_id → entities_current`
- Indexes on `entities_current`: `idx_entities_canonical_name`,
  `idx_entities_parent_id`, `idx_entities_status`, `idx_entities_entity_type`
- `schema_metadata` table with seed rows: `created_at`, `app_version`

### `000001_initial_schema.down.sql` (rewritten)

Drops all indexes and tables in reverse dependency order:

```sql
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

### Deleted files

- `000002_add_loaned_system_location.up.sql` / `.down.sql`
- `000003_nanoid_migration.up.sql` / `.down.sql`
- `000004_add_removed_system_location.up.sql` / `.down.sql`
- `000005_remove_project_tables.up.sql` / `.down.sql`
- `000006_entity_consolidation.up.sql` / `.down.sql`

## Go Code

No changes. The following remain untouched:

- `internal/database/migrations.go` — `RunMigrations`, `GetMigrationVersion`,
  `RollbackMigration`, `SetMigrationVersion`
- `internal/database/database.go` — `Config.AutoMigrate`, `Open`
- All `cmd/*/` test files that set `AutoMigrate: true`
- `internal/database/migrations_test.go` — kept as-is; rollback loop will
  exhaust after one step instead of six

## Testing

No test changes. Existing tests will naturally reflect the new single-migration
state:

- `GetMigrationVersion()` returns version 1 (was 6)
- Rollback loop in `migrations_test.go` succeeds once, then gets
  `ErrNilVersion` on subsequent calls (which the existing test already handles)
- All other tests using `AutoMigrate: true` are unaffected

## What is NOT changing

- golang-migrate dependency
- Migration infrastructure (`migrations.go`, embedded FS, `migrationsFS`)
- `Config.AutoMigrate` field and default
- Any command or projection code
