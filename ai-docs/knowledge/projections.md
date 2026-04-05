# Projection Strategy

**Source**: docs/DESIGN.md
**Purpose**: Projection tables, rebuild strategy, consistency validation

---

## Projection Philosophy

- **Projections are disposable** - can be rebuilt from events
- **Events are source of truth** - projections are derived
- **Fast reads** - projections optimized for query performance
- **Strict replay** - validation failures stop replay immediately
- **No silent repair** - corruption must be explicit and diagnosed

---

## Projection Tables

### locations_current

**Purpose**: Current state of location tree with cached paths

```sql
CREATE TABLE locations_current (
  location_id           TEXT PRIMARY KEY,
  display_name          TEXT NOT NULL,
  canonical_name        TEXT NOT NULL UNIQUE,
  parent_id             TEXT,  -- NULL for root locations
  full_path_display     TEXT NOT NULL,  -- cached, e.g. "Garage >> Shelf A >> Tote F"
  full_path_canonical   TEXT NOT NULL,  -- cached, e.g. "garage:shelf_a:tote_f"
  depth                 INTEGER NOT NULL,  -- 0 = root
  is_system             BOOLEAN NOT NULL DEFAULT 0,  -- true for Missing, Borrowed
  updated_at            TEXT NOT NULL,  -- last event timestamp

  FOREIGN KEY (parent_id) REFERENCES locations_current(location_id)
);

CREATE INDEX idx_locations_parent ON locations_current(parent_id);
CREATE INDEX idx_locations_canonical ON locations_current(canonical_name);
CREATE INDEX idx_locations_path_canonical ON locations_current(full_path_canonical);
CREATE INDEX idx_locations_system ON locations_current(is_system);
```

**Cached Fields**:
- `full_path_display` - user-facing path representation
- `full_path_canonical` - normalized path for matching
- `depth` - distance from root (for tree queries)

**Update Triggers**:
- `location.created` → insert row
- `location.reparented` → update row + all descendants (recursive)
- `location.deleted` → delete row

**Path Recomputation** (on reparent):
```
1. Walk up from location to root, collecting names
2. Build full_path_display: join with " >> " separator
3. Build full_path_canonical: join with ":" separator
4. Set depth = number of ancestors
5. Recursively update all descendants
```

---

### items_current

**Purpose**: Current state of items and their locations

```sql
CREATE TABLE items_current (
  item_id                  TEXT PRIMARY KEY,
  display_name             TEXT NOT NULL,
  canonical_name           TEXT NOT NULL,  -- NOT unique
  location_id              TEXT NOT NULL,
  in_temporary_use         BOOLEAN NOT NULL DEFAULT 0,
  temp_origin_location_id  TEXT,  -- NULL if not in temporary use
  project_id               TEXT,  -- NULL if no project association
  last_event_id            INTEGER NOT NULL,  -- replay checkpoint
  updated_at               TEXT NOT NULL,    -- last event timestamp

  FOREIGN KEY (location_id) REFERENCES locations_current(location_id),
  FOREIGN KEY (temp_origin_location_id) REFERENCES locations_current(location_id),
  FOREIGN KEY (project_id) REFERENCES projects_current(project_id)
);

CREATE INDEX idx_items_location ON items_current(location_id);
CREATE INDEX idx_items_canonical ON items_current(canonical_name);
CREATE INDEX idx_items_project ON items_current(project_id);
CREATE INDEX idx_items_temp_use ON items_current(in_temporary_use) WHERE in_temporary_use = 1;
CREATE INDEX idx_items_last_event ON items_current(last_event_id);
CREATE INDEX idx_items_canonical_location ON items_current(canonical_name, location_id);
CREATE INDEX idx_items_location_covering ON items_current(location_id, display_name, canonical_name);
```

**State Fields**:
- `in_temporary_use` - true if item in temporary location
- `temp_origin_location_id` - original location before temporary use
- `project_id` - current project association
- `last_event_id` - last event that modified this item

**Update Triggers**:
- `item.created` → insert row
- `item.moved` → update location, temp use state, project
- `item.borrowed` → update location to Borrowed
- `item.marked_missing` → update location to Missing
- `item.marked_found` → update location, set temp use state
- `item.deleted` → delete row

**Temporary Use Logic**:
```
On first temporary_use move:
  in_temporary_use = true
  temp_origin_location_id = from_location_id

On subsequent temporary_use moves:
  in_temporary_use = true
  temp_origin_location_id = (unchanged, preserve original)

On rehome move:
  in_temporary_use = false
  temp_origin_location_id = NULL
```

---

### projects_current

**Purpose**: Current state of projects

```sql
CREATE TABLE projects_current (
  project_id     TEXT PRIMARY KEY,  -- user-provided slug
  status         TEXT NOT NULL,     -- 'active' | 'completed'
  updated_at     TEXT NOT NULL,

  CHECK (status IN ('active', 'completed'))
);

CREATE INDEX idx_projects_status ON projects_current(status);
```

**Update Triggers**:
- `project.created` → insert with status='active'
- `project.completed` → update status='completed'
- `project.reopened` → update status='active'
- `project.deleted` → delete row

**Simplicity**:
- No additional metadata in v1
- Could add `description`, `created_by` in future
- Status transitions are unrestricted (can reopen completed)

---

## Replay Strategy

### Full Rebuild Process

```sql
-- 1. Clear all projections
DELETE FROM items_current;
DELETE FROM locations_current;
DELETE FROM projects_current;

-- 2. Replay events in order
SELECT * FROM events ORDER BY event_id ASC;

FOR EACH event:
  -- 3. Validate event
  CALL validate_event(event)

  -- 4. Apply projection update
  CALL apply_projection(event)

  -- 5. On error: STOP, report event_id
```

### Incremental Replay

**Use Case**: Projection is up-to-date, apply only new events

```sql
-- 1. Find max event_id in projections
SELECT MAX(last_event_id) FROM items_current;
SELECT MAX(event_id) FROM (
  SELECT event_id FROM events WHERE location_id IS NOT NULL
  -- similar for projects
);

-- 2. Replay events after checkpoint
SELECT * FROM events
WHERE event_id > last_checkpoint
ORDER BY event_id ASC;

FOR EACH event:
  CALL validate_event(event)
  CALL apply_projection(event)
```

**Checkpoint Tracking**:
- `items_current.last_event_id` tracks per-item
- Global checkpoint = MIN(all last_event_id values)
- Safe to replay from global checkpoint

---

## Validation During Replay

### Strict Validation Rules

**item.moved**:
```
ASSERT projection.location_id = event.from_location_id
  "Location mismatch: projection has {projection.location_id}, event expects {from_location_id}"

ASSERT location_exists(event.to_location_id)
  "Target location {to_location_id} does not exist"

ASSERT event.from_location_id != event.to_location_id
  "Cannot move to same location"
```

**location.reparented**:
```
ASSERT projection.parent_id = event.from_parent_id
  "Parent mismatch: projection has {projection.parent_id}, event expects {from_parent_id}"

ASSERT !creates_cycle(event.location_id, event.to_parent_id)
  "Reparenting would create cycle"
```

**item.deleted**:
```
ASSERT item_exists(event.item_id)
  "Item {item_id} does not exist"

ASSERT projection.location_id = event.previous_location_id
  "Location mismatch during deletion"
```

### On Validation Failure

**Stop Immediately**:
```
ERROR: Replay validation failed at event_id={event_id}
Event type: {event_type}
Error: {validation_error_message}

Replay aborted. Projection may be inconsistent.
Manual intervention required.
```

**No Silent Repair**:
- Do NOT skip event
- Do NOT "best guess" the fix
- Fail loudly with diagnostic info

**Recovery**:
- Investigate event log corruption
- Fix event (if possible)
- Rebuild projection from scratch

---

## Consistency Validation

### Doctor Command

**Purpose**: Validate projection consistency with event log

```bash
wherehouse doctor
```

**Process**:
```
1. Snapshot current projection (backup)
2. Rebuild projection from scratch into temp tables
3. Compare temp tables with current projection
4. Report any mismatches
5. Exit 0 if consistent, non-zero if mismatched
```

**Comparison Checks**:
```sql
-- Items
SELECT item_id FROM items_current
EXCEPT
SELECT item_id FROM items_current_temp;
-- (and reverse)

-- Row-level comparison
SELECT * FROM items_current i1
JOIN items_current_temp i2 USING (item_id)
WHERE i1.location_id != i2.location_id
   OR i1.in_temporary_use != i2.in_temporary_use
   OR ...;
```

**Output**:
```
✓ Locations: 45 consistent
✓ Items: 312 consistent
✓ Projects: 8 consistent

No inconsistencies found.
```

**On Mismatch**:
```
✗ Items: 2 inconsistencies found

Item "10mm socket" (id: abc123):
  Current projection: location_id=xyz789
  Rebuilt projection: location_id=abc456

Item "screwdriver" (id: def456):
  Current projection: in_temporary_use=false
  Rebuilt projection: in_temporary_use=true

Projection is INCONSISTENT with event log.
Recommend: wherehouse doctor --rebuild
```

### Doctor Flags

- `--rebuild` - Destructively replace projection with rebuilt version
- `--verbose` - Show detailed comparison output
- `--json` - Machine-readable output

---

## Path Recomputation

### Location Reparenting

**Trigger**: `location.reparented` event

**Algorithm**:
```python
def recompute_paths(location_id, new_parent_id):
    # 1. Compute new path for this location
    path_segments_display = []
    path_segments_canonical = []

    current = new_parent_id
    while current is not None:
        loc = get_location(current)
        path_segments_display.insert(0, loc.display_name)
        path_segments_canonical.insert(0, loc.canonical_name)
        current = loc.parent_id

    # 2. Update this location
    loc = get_location(location_id)
    path_segments_display.append(loc.display_name)
    path_segments_canonical.append(loc.canonical_name)

    loc.full_path_display = " >> ".join(path_segments_display)
    loc.full_path_canonical = ":".join(path_segments_canonical)
    loc.depth = len(path_segments_canonical) - 1

    # 3. Recursively update all descendants
    for child in get_children(location_id):
        recompute_paths(child.location_id, location_id)
```

**Path Format**:
- Display: `"Garage >> Shelf A >> Tote F"`
- Canonical: `"garage:shelf_a:tote_f"`

**Performance**:
- Recursive subtree update required
- Can be expensive for large subtrees
- Acceptable for v1 (location reparenting is rare)

---

## Projection Optimization

### Query Patterns

**Find item by name**:
```sql
SELECT * FROM items_current
WHERE canonical_name = canonicalize('10mm socket')
  AND location_id = (SELECT location_id FROM locations_current
                     WHERE canonical_name = 'garage:toolbox');
```

**List items in location (hierarchical)**:
```sql
SELECT i.* FROM items_current i
JOIN locations_current l ON i.location_id = l.location_id
WHERE l.full_path_canonical LIKE 'garage%'
ORDER BY l.full_path_canonical, i.display_name;
```

**Items to return for project**:
```sql
SELECT i.*, l.full_path_display
FROM items_current i
JOIN locations_current l ON i.location_id = l.location_id
WHERE i.project_id = 'my-project'
ORDER BY l.full_path_display;
```

**Items in temporary use**:
```sql
SELECT i.*,
       l_current.full_path_display AS current_location,
       l_origin.full_path_display AS origin_location
FROM items_current i
JOIN locations_current l_current ON i.location_id = l_current.location_id
LEFT JOIN locations_current l_origin ON i.temp_origin_location_id = l_origin.location_id
WHERE i.in_temporary_use = true
ORDER BY l_current.full_path_display;
```

### Index Strategy

**Critical Indexes**:
- `items_current(canonical_name)` - name lookup
- `items_current(location_id)` - location grouping
- `items_current(project_id)` - project queries
- `locations_current(canonical_name)` - unique constraint + lookup
- `locations_current(parent_id)` - tree traversal
- `locations_current(full_path_canonical)` - hierarchical queries

**Composite Indexes** (query optimization):
- `items_current(canonical_name, location_id)` - selector resolution (LOCATION:ITEM pattern)
- `items_current(location_id, display_name, canonical_name)` - covering index for listing items at location

**Partial Indexes**:
- `items_current(in_temporary_use) WHERE in_temporary_use = 1` - temp use queries

---

## Concurrency & Locking

### Write Serialization

**Per-Item Locking**:
- SQLite provides row-level locking (WAL mode)
- Concurrent reads allowed
- Concurrent writes to different items allowed
- Writes to same item serialized

**Event Insertion**:
```sql
BEGIN IMMEDIATE TRANSACTION;
  -- Validate event
  -- Insert into events table
  -- Update projection
COMMIT;
```

**Retry Logic**:
- On `SQLITE_BUSY` → retry with exponential backoff
- Max retries configurable (default: 5)
- Timeout configurable (default: 30s)

### Network Storage

**Compatibility**:
- SQLite WAL mode works on network mounts (NFS, SMB)
- Requires `wal_autocheckpoint` tuning
- May be slower than local storage

**Recommendations**:
```sql
PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;  -- faster on network storage
PRAGMA wal_autocheckpoint=1000;
PRAGMA busy_timeout=30000;  -- 30 seconds
```

---

## Migration Strategy

### Schema Versioning

**Metadata Table**:
```sql
CREATE TABLE schema_metadata (
  key    TEXT PRIMARY KEY,
  value  TEXT NOT NULL
);

INSERT INTO schema_metadata (key, value) VALUES
  ('schema_version', '1'),
  ('created_at', '2026-02-19T10:00:00Z');
```

**Version Check**:
```go
func checkSchemaVersion(db *sql.DB) error {
    var version string
    err := db.QueryRow("SELECT value FROM schema_metadata WHERE key='schema_version'").Scan(&version)
    if err != nil {
        return fmt.Errorf("schema_metadata not found: %w", err)
    }
    if version != expectedVersion {
        return fmt.Errorf("schema version mismatch: expected %s, got %s", expectedVersion, version)
    }
    return nil
}
```

### Migrations

**Future Migrations**:
- Event log is append-only (never altered)
- Projection schema can change
- Add new projection tables as needed
- Rebuild projections after schema change

**Migration Process**:
```
1. Check current schema_version
2. Apply migrations in order (v1→v2, v2→v3, ...)
3. Rebuild projections if schema changed
4. Update schema_version
```

---

**Version**: 1.0 (from DESIGN.md v1)
**Last Updated**: 2026-02-19
