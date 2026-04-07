# Event Catalog

**Source**: docs/DESIGN.md
**Purpose**: Complete event schema reference for event-sourced architecture

---

## Event Sourcing Principles

- **Events are source of truth** - not projections
- **Ordering**: Strictly by `event_id` (integer, auto-increment)
- **Timestamps**: Stored as UTC RFC3339 with `Z` (informational, not for ordering)
- **Immutability**: Events never deleted or modified
- **No undo**: Corrections create new compensating events
- **Replay**: Projections rebuilt by replaying events in `event_id` order
- **Validation**: Events validated before persistence (reject invalid)

---

## Common Fields (All Events)

```
event_id         INTEGER PRIMARY KEY AUTOINCREMENT
event_type       TEXT NOT NULL
timestamp_utc    TEXT NOT NULL  -- RFC3339 with Z
actor_user_id    TEXT NOT NULL  -- user who triggered event
note             TEXT NULL      -- optional free-text note
```

**Event ID Authority**:
- Integer sequence defines replay order
- Timestamps are informational only
- Clock skew does not affect determinism

---

## Item Events

### item.created

**Purpose**: Create new item in system

**Fields**:
```json
{
  "event_id": 1,
  "event_type": "item.created",
  "timestamp_utc": "2026-02-19T10:30:00Z",
  "actor_user_id": "alice",
  "item_id": "uuid-v7",
  "display_name": "10mm Socket Wrench",
  "canonical_name": "10mm_socket_wrench",
  "location_id": "uuid",
  "note": "optional context"
}
```

**Projection Updates**:
```sql
INSERT INTO items_current (
  item_id,
  display_name,
  canonical_name,
  location_id,
  in_temporary_use,
  temp_origin_location_id,
  project_id,
  last_event_id,
  updated_at
) VALUES (
  event.item_id,
  event.display_name,
  event.canonical_name,
  event.location_id,
  false,
  NULL,
  NULL,
  event.event_id,
  event.timestamp_utc
)
```

**Validation**:
- `item_id` must not exist in projection
- `display_name` must not be empty
- `canonical_name` must not contain `:` (reserved for selector separator)
- `canonical_name` is derived from display_name via canonicalization rules
- `location_id` must exist in locations_current
- `location_id` must not be a system location (optional constraint)

---

### item.moved

**Purpose**: Move item between locations

**Fields**:
```json
{
  "event_id": 2,
  "event_type": "item.moved",
  "timestamp_utc": "2026-02-19T10:35:00Z",
  "actor_user_id": "alice",
  "item_id": "uuid",
  "from_location_id": "uuid",
  "to_location_id": "uuid",
  "move_type": "temporary_use",  // or "rehome"
  "project_action": "set",        // "clear" | "keep" | "set"
  "project_id": "my-project",     // nullable
  "note": "optional reason"
}
```

**Move Types**:
- `temporary_use` - temporary move, expected to return
- `rehome` - permanent relocation

**Project Actions**:
- `clear` - remove project association (default)
- `keep` - preserve current project_id
- `set` - set to specified project_id (requires project_id field)

**Projection Updates** (temporary_use):
```sql
-- First temporary move
IF item.in_temporary_use = false THEN
  SET in_temporary_use = true
  SET temp_origin_location_id = from_location_id
END IF

-- All temporary moves
SET location_id = to_location_id
SET project_id = (according to project_action)
SET last_event_id = event.event_id
SET updated_at = event.timestamp_utc
```

**Projection Updates** (rehome):
```sql
SET location_id = to_location_id
SET in_temporary_use = false
SET temp_origin_location_id = NULL
SET project_id = (according to project_action)
SET last_event_id = event.event_id
SET updated_at = event.timestamp_utc
```

**Validation**:
- `item_id` must exist
- `from_location_id` must match current projection location (integrity check)
- `to_location_id` must exist
- `from_location_id` ≠ `to_location_id`
- If `project_action = "set"`, `project_id` must be active project
- `move_type` must be enum value

**Integrity Rule**:
- Replay MUST fail if `from_location_id` doesn't match projection
- This detects projection corruption or concurrent write issues

---

### item.borrowed

**Purpose**: Mark item as borrowed by specific person

**Fields**:
```json
{
  "event_id": 3,
  "event_type": "item.borrowed",
  "timestamp_utc": "2026-02-19T11:00:00Z",
  "actor_user_id": "alice",
  "item_id": "uuid",
  "from_location_id": "uuid",
  "borrowed_by": "bob",  // REQUIRED, non-blank
  "note": "optional"
}
```

**Projection Updates**:
```sql
SET location_id = <BORROWED_LOCATION_UUID>
SET last_event_id = event.event_id
SET updated_at = event.timestamp_utc
-- Note: Does NOT set in_temporary_use (borrowed is different state)
```

**Validation**:
- `item_id` must exist
- `from_location_id` must match current projection location
- `borrowed_by` must not be empty/blank
- `borrowed_by` should reference declared user (warn if not)

**Return Behavior**:
- Returning = normal `item.moved` from `Borrowed` location to real location
- Move type typically `rehome` (unless returning to temporary use)

---

### item.marked_missing

**Purpose**: Mark item as lost/missing

**Fields**:
```json
{
  "event_id": 4,
  "event_type": "item.marked_missing",
  "timestamp_utc": "2026-02-19T12:00:00Z",
  "actor_user_id": "alice",
  "item_id": "uuid",
  "previous_location_id": "uuid",
  "note": "checked toolbox, not there"
}
```

**Projection Updates**:
```sql
SET location_id = <MISSING_LOCATION_UUID>
SET last_event_id = event.event_id
SET updated_at = event.timestamp_utc
-- Preserves in_temporary_use and temp_origin_location_id
```

**Validation**:
- `item_id` must exist
- `previous_location_id` must match current projection location

**Use Case**:
- User expected item at location, but it's not there
- Records last known location for suggestions

---

### item.marked_found

**Purpose**: Mark previously missing item as found

**Fields**:
```json
{
  "event_id": 5,
  "event_type": "item.marked_found",
  "timestamp_utc": "2026-02-19T13:00:00Z",
  "actor_user_id": "alice",
  "item_id": "uuid",
  "found_location_id": "uuid",
  "home_location_id": "uuid",
  "note": "was in garage all along"
}
```

**Projection Updates**:
```sql
SET location_id = found_location_id
SET in_temporary_use = true
SET temp_origin_location_id = home_location_id
SET last_event_id = event.event_id
SET updated_at = event.timestamp_utc
```

**Semantics**:
- Item found at `found_location_id`
- Should return to `home_location_id` eventually (temporary use)
- User must later move item back to home (not automatic)

**Validation**:
- `item_id` must exist
- Current location should be `Missing` (warn if not)
- `found_location_id` must exist
- `home_location_id` must exist

---

### item.removed

**Purpose**: Remove item from active inventory by moving it to the "Removed" system location

**Fields**:
```json
{
  "event_id": 6,
  "event_type": "item.removed",
  "timestamp_utc": "2026-02-19T14:00:00Z",
  "actor_user_id": "alice",
  "item_id": "uuid",
  "previous_location_id": "uuid",
  "note": "broken beyond repair"
}
```

**Projection Updates**:
```sql
SET location_id = <REMOVED_LOCATION_UUID>
SET last_event_id = event.event_id
SET updated_at = event.timestamp_utc
```

**Validation**:
- `item_id` must exist
- `previous_location_id` must match current projection location

**History**:
- Item moves to the "Removed" system location
- History is preserved in the event log
- Item remains in projection at the "Removed" location

---

## Location Events

### location.created

**Purpose**: Create new location in system

**Fields**:
```json
{
  "event_id": 7,
  "event_type": "location.created",
  "timestamp_utc": "2026-02-19T14:00:00Z",
  "actor_user_id": "alice",
  "location_id": "uuid-v7",
  "display_name": "Shelf A",
  "canonical_name": "shelf_a",
  "parent_id": "uuid",
  "is_system": false,
  "note": "optional context"
}
```

**Projection Updates**:
```sql
-- Compute path fields by walking up from parent_id to root
INSERT INTO locations_current (
  location_id,
  display_name,
  canonical_name,
  parent_id,
  full_path_display,
  full_path_canonical,
  depth,
  is_system,
  updated_at
) VALUES (
  event.location_id,
  event.display_name,
  event.canonical_name,
  event.parent_id,
  computed_full_path_display,    -- e.g., "Garage >> Shelf A"
  computed_full_path_canonical,  -- e.g., "garage:shelf_a"
  computed_depth,                -- count of ancestors
  event.is_system,
  event.timestamp_utc
)
```

**Path Computation**:
```
1. If parent_id is NULL (root location):
   - full_path_display = display_name
   - full_path_canonical = canonical_name
   - depth = 0
2. If parent_id exists:
   - Walk up from parent to root, collecting display_name and canonical_name
   - full_path_display = parent_path + " >> " + display_name
   - full_path_canonical = parent_path + ":" + canonical_name
   - depth = parent_depth + 1
```

**Validation**:
- `location_id` must not exist in projection
- `canonical_name` must be globally unique in locations_current
- `canonical_name` must not contain `:` (reserved for path separator)
- `display_name` must not be empty
- `parent_id` must exist if not NULL
- `is_system` should only be true for seed data (Missing, Borrowed, Removed)
- Must not create cycles (location cannot be its own ancestor)

---

### location.reparented

**Purpose**: Move location to different parent in tree

**Fields**:
```json
{
  "event_id": 8,
  "event_type": "location.reparented",
  "timestamp_utc": "2026-02-19T15:00:00Z",
  "actor_user_id": "alice",
  "location_id": "uuid",
  "from_parent_id": "uuid",  // nullable (if was root)
  "to_parent_id": "uuid",    // nullable (if becoming root)
  "note": "reorganizing storage"
}
```

**Projection Updates**:
```sql
-- Validate cycle detection first
SET parent_id = to_parent_id
SET full_path_display = (recompute)
SET full_path_canonical = (recompute)
SET depth = (recompute)
SET updated_at = event.timestamp_utc

-- Also update all descendants (recursive)
-- Their full_path fields must be recalculated
```

**Validation**:
- `location_id` must exist
- `location_id` must not be system location
- `from_parent_id` must match current projection parent
- `to_parent_id` must exist (if not NULL)
- **Must reject cycles**: `location_id` cannot be ancestor of itself
- **Cycle check**: Walk up from `to_parent_id` until root, ensure no loop

**Subtree Update**:
- All descendant locations must update cached paths
- Recursive update through tree

---

### location.removed

**Purpose**: Remove an empty non-system location from the projection

**Fields**:
```json
{
  "event_id": 9,
  "event_type": "location.removed",
  "timestamp_utc": "2026-02-19T16:00:00Z",
  "actor_user_id": "alice",
  "location_id": "uuid",
  "previous_parent_id": "uuid",  // nullable
  "note": "no longer needed"
}
```

**Projection Updates**:
```sql
DELETE FROM locations_current WHERE location_id = event.location_id
```

**Validation**:
- `location_id` must exist
- `location_id` must not be system location
- Location must have no children (no sub-locations)
- Location must have no items (items must be moved or removed first)
- `previous_parent_id` must match current projection parent

**History**:
- Location is removed from the projection
- Event remains in log for history

---

## Project Events

### project.created

**Purpose**: Create new project

**Fields**:
```json
{
  "event_id": 10,
  "event_type": "project.created",
  "timestamp_utc": "2026-02-19T17:00:00Z",
  "actor_user_id": "alice",
  "project_id": "my-project"  // user-provided slug
}
```

**Projection Updates**:
```sql
INSERT INTO projects_current (
  project_id,
  status,
  updated_at
) VALUES (
  event.project_id,
  'active',
  event.timestamp_utc
)
```

**Validation**:
- `project_id` must not exist
- `project_id` must not contain `:`

---

### project.completed

**Purpose**: Mark project as completed

**Fields**:
```json
{
  "event_id": 11,
  "event_type": "project.completed",
  "timestamp_utc": "2026-02-19T18:00:00Z",
  "actor_user_id": "alice",
  "project_id": "my-project"
}
```

**Projection Updates**:
```sql
UPDATE projects_current
SET status = 'completed',
    updated_at = event.timestamp_utc
WHERE project_id = event.project_id
```

**Side Effects** (command layer, not projection):
- Display "items to return" list
- Query: items where `project_id = this project`
- Does NOT auto-move items

**Validation**:
- `project_id` must exist
- No restriction on current status (can complete already completed)

---

### project.reopened

**Purpose**: Reactivate completed project

**Fields**:
```json
{
  "event_id": 12,
  "event_type": "project.reopened",
  "timestamp_utc": "2026-02-19T19:00:00Z",
  "actor_user_id": "alice",
  "project_id": "my-project"
}
```

**Projection Updates**:
```sql
UPDATE projects_current
SET status = 'active',
    updated_at = event.timestamp_utc
WHERE project_id = event.project_id
```

**Validation**:
- `project_id` must exist
- No restriction on current status

---


## Event Storage Schema

```sql
CREATE TABLE events (
  event_id         INTEGER PRIMARY KEY AUTOINCREMENT,
  event_type       TEXT NOT NULL,
  timestamp_utc    TEXT NOT NULL,  -- RFC3339 with Z
  actor_user_id    TEXT NOT NULL,

  -- Polymorphic payload (JSON or separate columns)
  payload          TEXT NOT NULL,  -- JSON recommended

  -- Optional: separate indexed columns for critical fields
  item_id          TEXT,           -- for item events
  location_id      TEXT,           -- for location events
  project_id       TEXT,           -- for project events

  note             TEXT
);

CREATE INDEX idx_events_item_id ON events(item_id) WHERE item_id IS NOT NULL;
CREATE INDEX idx_events_location_id ON events(location_id) WHERE location_id IS NOT NULL;
CREATE INDEX idx_events_project_id ON events(project_id) WHERE project_id IS NOT NULL;
CREATE INDEX idx_events_type ON events(event_type);
```

---

## Replay Rules

### Replay Order
```
FOR each event ORDER BY event_id ASC:
  1. Validate event fields
  2. Validate against current projection state
  3. Apply projection updates
  4. Continue on success, FAIL on error
```

### Validation During Replay

**Critical Checks**:
- `from_location_id` must match projection (for moves)
- Referenced entities must exist (FKs)
- Constraints must hold (cycles, uniqueness)

**On Validation Failure**:
- STOP replay immediately
- Report event_id and error
- Do not silently repair
- Require manual intervention or event log fix

### Projection Consistency

**Doctor Command**:
- Rebuild projection from scratch
- Compare with existing projection
- Report mismatches
- Non-zero exit on inconsistency

**No Silent Repair**:
- Never auto-fix corrupted projections
- Never skip invalid events
- Fail fast and loud

---

## Event Design Patterns

### Explicit State in Events

✅ **Good**: Store `from_location_id` in `item.moved`
- Validates projection integrity
- Detects concurrent writes
- Makes events self-documenting

❌ **Bad**: Only store `to_location_id`
- Cannot validate projection
- Silent corruption possible

### Compensating Events

**No Undo**: Cannot delete or modify events

**Corrections**:
- Wrong move? Create new move back
- Removed by mistake? Create new item (different UUID)
- Wrong project? Move with new project

### Event Naming

**Pattern**: `entity.action_past_tense`
- `item.moved` (not `item.move`)
- `project.completed` (not `project.complete`)
- `location.removed` (not `location.remove`)

---

**Version**: 1.0 (from DESIGN.md v1)
**Last Updated**: 2026-02-19
