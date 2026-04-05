# Wherehouse Schema Validation Report

**Date**: 2026-02-19
**Status**: No implementation exists -- design-only review
**Documents reviewed**: DESIGN.md, events.md, projections.md, business-rules.md

---

## Executive Summary

There is **no database implementation** in the repository. The codebase contains only design documentation (CLAUDE.md, docs/DESIGN.md, .claude/knowledge/ files). No Go source files, no SQL files, no migration scripts, and no `internal/` directory exist. This report validates the design documents for internal consistency and provides the complete reference DDL for initial implementation.

---

## Design Document Consistency Analysis

### What is well-designed (no issues found)

1. **Event table schema** - Clean separation of common fields (event_id, event_type, timestamp_utc, actor_user_id) with JSON payload for polymorphic data and indexed denormalized columns (item_id, location_id, project_id) for filtered queries.

2. **Projection table schemas** - Properly denormalized for read performance. Cached computed fields (full_path_display, full_path_canonical, depth) on locations_current avoid recursive queries at read time.

3. **Index strategy** - Partial indexes on sparse data (events item_id/location_id/project_id WHERE NOT NULL, items in_temporary_use WHERE = 1). Covering indexes for common query patterns.

4. **Foreign key design** - locations_current self-referencing FK for tree structure, items_current FK to locations and projects. Proper nullable FKs for optional relationships.

5. **Event ordering** - Strict event_id INTEGER AUTOINCREMENT for deterministic replay. Timestamps are informational only.

6. **Validation data in events** - from_location_id, from_parent_id, previous_location_id stored in events for integrity checking during replay.

7. **Schema versioning** - schema_metadata table with key-value pairs for version tracking.

### Minor design gaps identified

1. **Missing `location.created` event schema in events.md** - The projections.md references `location.created` as a trigger for inserting into locations_current, but events.md has no explicit schema for this event type. DESIGN.md also omits it. The event must include: location_id, display_name, canonical_name, parent_id (nullable), is_system, actor_user_id, timestamp_utc.

2. **Missing display_name/canonical_name in item.created event** - The events.md schema for item.created omits display_name and canonical_name fields, but projections.md shows items_current requires both. The event payload JSON must carry these.

3. **Missing display_name/canonical_name in location event fields** - Similar to items, location.created needs display_name and canonical_name but no explicit event schema exists.

4. **No `location.renamed` event** - DESIGN.md mentions system locations "Cannot be renamed" implying rename is possible for non-system locations, but no rename event type exists in the catalog. This may be intentional for v1 (rename = delete + recreate) but should be documented.

5. **items_current missing display_name in item.created projection SQL** - The projection update SQL in events.md for item.created does not include display_name or canonical_name in the INSERT, but the table requires them.

6. **schema_metadata table not in main DDL** - Defined in projections.md migration section but not included alongside the main table definitions.

7. **No explicit event for location.created** - The events.md catalog covers location.reparented and location.deleted but not location.created. This is the most significant gap.

---

## Complete Reference DDL

This is the authoritative DDL derived from all design documents, with gaps resolved.

### SQLite Configuration (runtime)

```sql
PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;
PRAGMA synchronous=NORMAL;
PRAGMA busy_timeout=30000;
PRAGMA wal_autocheckpoint=1000;
```

### Event Storage

```sql
CREATE TABLE events (
    event_id         INTEGER PRIMARY KEY AUTOINCREMENT,
    event_type       TEXT NOT NULL,
    timestamp_utc    TEXT NOT NULL,
    actor_user_id    TEXT NOT NULL,
    payload          TEXT NOT NULL,   -- JSON
    item_id          TEXT,            -- denormalized for indexed queries
    location_id      TEXT,            -- denormalized for indexed queries
    project_id       TEXT,            -- denormalized for indexed queries
    note             TEXT
);

CREATE INDEX idx_events_type ON events(event_type);
CREATE INDEX idx_events_item_id ON events(item_id) WHERE item_id IS NOT NULL;
CREATE INDEX idx_events_location_id ON events(location_id) WHERE location_id IS NOT NULL;
CREATE INDEX idx_events_project_id ON events(project_id) WHERE project_id IS NOT NULL;
```

### Projection Tables

```sql
CREATE TABLE locations_current (
    location_id           TEXT PRIMARY KEY,
    display_name          TEXT NOT NULL,
    canonical_name        TEXT NOT NULL UNIQUE,
    parent_id             TEXT,
    full_path_display     TEXT NOT NULL,
    full_path_canonical   TEXT NOT NULL,
    depth                 INTEGER NOT NULL,
    is_system             BOOLEAN NOT NULL DEFAULT 0,
    updated_at            TEXT NOT NULL,

    FOREIGN KEY (parent_id) REFERENCES locations_current(location_id)
);

CREATE INDEX idx_locations_parent ON locations_current(parent_id);
CREATE INDEX idx_locations_canonical ON locations_current(canonical_name);
CREATE INDEX idx_locations_path_canonical ON locations_current(full_path_canonical);
CREATE INDEX idx_locations_system ON locations_current(is_system);
```

```sql
CREATE TABLE projects_current (
    project_id     TEXT PRIMARY KEY,
    status         TEXT NOT NULL,
    updated_at     TEXT NOT NULL,

    CHECK (status IN ('active', 'completed'))
);

CREATE INDEX idx_projects_status ON projects_current(status);
```

```sql
CREATE TABLE items_current (
    item_id                  TEXT PRIMARY KEY,
    display_name             TEXT NOT NULL,
    canonical_name           TEXT NOT NULL,
    location_id              TEXT NOT NULL,
    in_temporary_use         BOOLEAN NOT NULL DEFAULT 0,
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
CREATE INDEX idx_items_project ON items_current(project_id);
CREATE INDEX idx_items_temp_use ON items_current(in_temporary_use) WHERE in_temporary_use = 1;
CREATE INDEX idx_items_last_event ON items_current(last_event_id);
```

### Schema Metadata

```sql
CREATE TABLE schema_metadata (
    key    TEXT PRIMARY KEY,
    value  TEXT NOT NULL
);

INSERT INTO schema_metadata (key, value) VALUES
    ('schema_version', '1'),
    ('created_at', datetime('now'));
```

### Table creation order (FK dependencies)

1. `events` (no FK dependencies)
2. `schema_metadata` (no FK dependencies)
3. `locations_current` (self-referencing FK only)
4. `projects_current` (no FK dependencies)
5. `items_current` (depends on locations_current, projects_current)

### System location seed data

```sql
-- Insert after locations_current table creation
-- UUIDs should be generated as v7 at application init time
-- These are placeholder values; actual UUIDs set by application
INSERT INTO locations_current (
    location_id, display_name, canonical_name,
    parent_id, full_path_display, full_path_canonical,
    depth, is_system, updated_at
) VALUES
    ('MISSING_UUID_HERE', 'Missing', 'missing',
     NULL, 'Missing', 'missing',
     0, 1, datetime('now')),
    ('BORROWED_UUID_HERE', 'Borrowed', 'borrowed',
     NULL, 'Borrowed', 'borrowed',
     0, 1, datetime('now'));
```

---

## SQLite Optimization Opportunities

### Already specified in design (good)

- WAL journal mode for read concurrency
- Partial indexes on sparse columns (events entity IDs, temp use flag)
- NORMAL synchronous mode (safe with WAL)
- busy_timeout for network-mounted databases

### Additional recommendations

1. **Page size tuning** - Consider `PRAGMA page_size=4096` (default) or `PRAGMA page_size=8192` if average row sizes are large. Must be set before any data is written.

2. **Covering index for item lookup by location** - The most common query pattern (list items at a location) would benefit from:
   ```sql
   CREATE INDEX idx_items_location_covering
   ON items_current(location_id, display_name, canonical_name);
   ```
   This avoids table lookups for the most frequent query.

3. **Composite index for selector resolution** - The LOCATION:ITEM selector pattern queries both canonical_name and location_id:
   ```sql
   CREATE INDEX idx_items_canonical_location
   ON items_current(canonical_name, location_id);
   ```

4. **Event replay performance** - For incremental replay, a composite index helps:
   ```sql
   CREATE INDEX idx_events_replay ON events(event_id, event_type);
   ```
   Though event_id is already the PK, this would be a covering index for the replay scan that reads event_type without touching the main table. Marginal benefit -- likely not needed for v1 data volumes.

5. **ANALYZE after bulk operations** - Run `ANALYZE` after initial data import or projection rebuild to update query planner statistics.

6. **Strict tables (SQLite 3.37+)** - Consider using `STRICT` table mode to enforce type affinity:
   ```sql
   CREATE TABLE events (...) STRICT;
   ```
   This prevents SQLite's flexible typing from allowing wrong types. Requires SQLite 3.37.0+ (2021-11-27), which is available in both modernc.org/sqlite and mattn/go-sqlite3 current versions.

---

## Missing Event Type: location.created

This is the most significant gap in the design documents. The event must exist for the system to function. Recommended schema:

```json
{
    "event_type": "location.created",
    "location_id": "uuid-v7",
    "display_name": "Shelf A",
    "canonical_name": "shelf_a",
    "parent_id": "uuid-or-null",
    "is_system": false,
    "actor_user_id": "alice",
    "timestamp_utc": "2026-02-19T10:00:00Z",
    "note": "optional"
}
```

Projection update:
```sql
INSERT INTO locations_current (
    location_id, display_name, canonical_name,
    parent_id, full_path_display, full_path_canonical,
    depth, is_system, updated_at
) VALUES (
    event.location_id, event.display_name, event.canonical_name,
    event.parent_id, computed_path_display, computed_path_canonical,
    computed_depth, event.is_system, event.timestamp_utc
);
```

Validation:
- location_id must not already exist
- canonical_name must be globally unique
- parent_id must exist if not NULL
- canonical_name must not contain ':'
- display_name must not be empty
- is_system should only be true for seed data (Missing, Borrowed)

---

## Recommendations for Implementation

### Priority order

1. **Define location.created event** in events.md before any implementation begins
2. **Add display_name/canonical_name** to item.created event schema in events.md
3. **Create initial migration** (schema version 1) with all DDL above
4. **Implement system location seeding** with deterministic UUIDs or well-known constants
5. **Consider STRICT tables** if targeting SQLite 3.37+

### Implementation notes for Go developers

- Use `BEGIN IMMEDIATE` transactions for all write operations (event insert + projection update)
- Set PRAGMAs on every connection open (they are per-connection, not persistent except journal_mode)
- foreign_keys=ON must be set on every connection
- Use prepared statements for projection updates (high frequency)
- JSON payload encoding/decoding should use Go structs with `json` tags
- Consider a migration framework (golang-migrate, goose) or embed SQL files

### Open questions for design review

1. Should system locations (Missing, Borrowed) use well-known constant UUIDs or be generated at database init time? Constants simplify code but are less "correct" UUID usage.
2. Should location.renamed be a v1 event type, or is delete+recreate acceptable?
3. Should the events table use STRICT mode to enforce TEXT types?

---

**Version**: 1.0
**Author**: db-developer agent
**Based on**: DESIGN.md v1, events.md v1, projections.md v1, business-rules.md v1
