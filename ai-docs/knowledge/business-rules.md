# Business Rules

**Source**: docs/DESIGN.md
**Purpose**: Validation rules, constraints, and invariants for event-sourced Wherehouse

---

## Critical Invariants

### Event Ordering
```
INVARIANT: Replay order determined STRICTLY by event_id
- Timestamps are informational only
- Clock skew does not affect ordering
- event_id is INTEGER AUTOINCREMENT
```

### Projection Integrity
```
INVARIANT: Projections are disposable
- Can be rebuilt from events at any time
- Rebuild must produce identical result to incremental updates
- Mismatch = corruption, must fail
```

### No Silent Repair
```
INVARIANT: Validation failures stop replay immediately
- Never skip invalid events
- Never "best guess" repairs
- Fail loudly with diagnostics
```

### Immutable Events
```
INVARIANT: Events never modified or deleted
- Event log is append-only
- Corrections use compensating events
- History is permanent
```

---

## Entity Validation Rules

### Item Names

**Format Constraints**:
```
RULE: Item names cannot contain ':'
  Reason: Conflicts with selector syntax LOCATION:ITEM

RULE: Item canonical_name NOT unique
  Action: Warn on duplicate, but allow
  Location-scoped selector disambiguates
```

**Canonicalization**:
```
1. Trim whitespace
2. Lowercase
3. Collapse internal whitespace to '_'
4. Normalize separators (-, _, space) to '_'
5. Strip/normalize punctuation consistently
```

**Examples**:
```
"10mm Socket Wrench" → "10mm_socket_wrench"
"Tool - Phillips #2" → "tool_phillips_2"
```

---

### Location Names

**Uniqueness Constraint**:
```
RULE: Location canonical_name MUST be globally unique
  Enforcement: Database UNIQUE constraint
  Reason: Prevents ambiguous path resolution
```

**System Locations**:
```
RULE: System locations cannot be modified
  Locations: "Missing", "Borrowed"
  is_system = true
  Cannot: rename, delete, reparent
```

**Tree Structure**:
```
RULE: Location tree MUST be acyclic
  Validation: Before location.reparented
  Algorithm: Walk up from to_parent_id, ensure no cycle

RULE: Location tree has unlimited depth
  No artificial depth limit
```

**Deletion**:
```
RULE: Can only delete empty locations
  Check: No items with location_id
  Check: No sub-locations with parent_id
```

---

### Project IDs

**Format Constraints**:
```
RULE: Project ID cannot contain ':'
  Reason: Reserved for future path syntax

RULE: Project ID is user-provided slug
  Not UUID
  Globally unique
  Case-sensitive (but recommend lowercase)
```

**Lifecycle**:
```
RULE: Projects can transition between any states
  active → completed → reopened → ...
  No restrictions on transitions

RULE: Can only delete projects with no item associations
  Check: No items in projection with project_id
```

---

## Event Validation Rules

### item.created

```
VALIDATE:
  ✓ item_id must not exist in projection
  ✓ location_id must exist in projection
  ✓ display_name must not be empty
  ✓ display_name must not contain ':'
```

---

### item.moved

```
VALIDATE:
  ✓ item_id must exist
  ✓ from_location_id MUST match projection.location_id
    CRITICAL: Detects projection corruption
  ✓ to_location_id must exist
  ✓ from_location_id ≠ to_location_id
  ✓ move_type in ['temporary_use', 'rehome']

IF project_action = 'set':
  ✓ project_id must not be null
  ✓ project_id must exist in projects_current
  ✓ project status must be 'active'

IF project_action = 'keep':
  ✓ item must have current project_id

IF project_action = 'clear':
  ✓ (no additional validation)
```

**Projection Logic**:
```
IF move_type = 'temporary_use':
  IF item.in_temporary_use = false:
    SET in_temporary_use = true
    SET temp_origin_location_id = from_location_id
  ELSE:
    PRESERVE temp_origin_location_id (keep original)

IF move_type = 'rehome':
  SET in_temporary_use = false
  SET temp_origin_location_id = NULL

SET location_id = to_location_id
APPLY project_action
SET last_event_id = event.event_id
```

---

### item.borrowed

```
VALIDATE:
  ✓ item_id must exist
  ✓ from_location_id MUST match projection.location_id
  ✓ borrowed_by must not be empty/blank
  ✓ borrowed_by should reference declared user (warn if not)
```

**Projection Logic**:
```
SET location_id = <BORROWED_LOCATION_UUID>
SET last_event_id = event.event_id
(Does NOT affect in_temporary_use state)
```

**Return**:
```
Returning = normal item.moved from Borrowed location
Move type typically 'rehome' (unless returning to temp use)
```

---

### item.marked_missing

```
VALIDATE:
  ✓ item_id must exist
  ✓ previous_location_id should match projection.location_id
    (warn if mismatch, but allow - may already be missing)
```

**Projection Logic**:
```
SET location_id = <MISSING_LOCATION_UUID>
SET last_event_id = event.event_id
PRESERVE in_temporary_use state
PRESERVE temp_origin_location_id
PRESERVE project_id
```

---

### item.marked_found

```
VALIDATE:
  ✓ item_id must exist
  ✓ found_location_id must exist
  ✓ home_location_id must exist
  ⚠ current location should be Missing (warn if not)
```

**Projection Logic**:
```
SET location_id = found_location_id
SET in_temporary_use = true
SET temp_origin_location_id = home_location_id
SET last_event_id = event.event_id
(User must later move back to home)
```

---

### item.deleted

```
VALIDATE:
  ✓ item_id must exist
  ✓ previous_location_id should match projection.location_id
```

**Projection Logic**:
```
DELETE FROM items_current WHERE item_id = event.item_id
```

**Finality**:
```
RULE: Deletion is permanent
  Cannot resurrect item
  Event remains in log for history
```

---

### location.reparented

```
VALIDATE:
  ✓ location_id must exist
  ✓ location_id must not be system location
  ✓ from_parent_id MUST match projection.parent_id
  ✓ to_parent_id must exist (if not NULL)
  ✓ MUST NOT create cycle:
    - Walk up from to_parent_id to root
    - Ensure location_id not in ancestor chain
```

**Cycle Detection Algorithm**:
```python
def would_create_cycle(location_id, new_parent_id):
    visited = set()
    current = new_parent_id

    while current is not None:
        if current == location_id:
            return True  # Cycle detected
        if current in visited:
            return True  # Cycle detected (malformed tree)
        visited.add(current)
        current = get_parent(current)

    return False  # No cycle
```

**Projection Logic**:
```
SET parent_id = to_parent_id
RECOMPUTE full_path_display, full_path_canonical, depth
RECURSIVELY recompute all descendants
SET last_event_id = event.event_id
```

---

### location.deleted

```
VALIDATE:
  ✓ location_id must exist
  ✓ location_id must not be system location
  ✓ previous_parent_id should match projection.parent_id
  ✓ MUST have no children:
    - No sub-locations with parent_id = location_id
  ✓ MUST have no items:
    - No items with location_id = location_id
```

**Projection Logic**:
```
DELETE FROM locations_current WHERE location_id = event.location_id
```

---

### project.created

```
VALIDATE:
  ✓ project_id must not exist
  ✓ project_id must not contain ':'
  ✓ project_id must not be empty
```

**Projection Logic**:
```
INSERT INTO projects_current (project_id, status, updated_at)
VALUES (event.project_id, 'active', event.timestamp_utc)
```

---

### project.completed

```
VALIDATE:
  ✓ project_id must exist
  (No restriction on current status)
```

**Projection Logic**:
```
UPDATE projects_current
SET status = 'completed', updated_at = event.timestamp_utc
WHERE project_id = event.project_id
```

**Command Layer Side Effect**:
```
Query items with project_id = this project
Display "items to return" list
Do NOT auto-move items
```

---

### project.reopened

```
VALIDATE:
  ✓ project_id must exist
  (No restriction on current status)
```

**Projection Logic**:
```
UPDATE projects_current
SET status = 'active', updated_at = event.timestamp_utc
WHERE project_id = event.project_id
```

---

### project.deleted

```
VALIDATE:
  ✓ project_id must exist
  ✓ MUST have no item associations:
    - No items in projection with project_id = this project_id
```

**Projection Logic**:
```
DELETE FROM projects_current WHERE project_id = event.project_id
```

---

## Selector Resolution Rules

### LOCATION:ITEM Syntax

```
RULE: Both parts resolve via canonical_name
  "Garage:10mm Socket" → "garage:10mm_socket"
  Exact match required (no fuzzy)

RULE: Location part may be full path or leaf name
  "garage:toolbox:10mm_socket" (full path)
  "toolbox:10mm_socket" (assumes unique leaf name)

RULE: Multiple matches return all
  If canonical resolution matches multiple items → return all
  User must disambiguate with more specific selector
```

### Ambiguity Handling

```
RULE: Duplicate canonical names in same location → warn
  Not an error, but flag for user attention

RULE: Duplicate canonical names across locations → OK
  Use location-scoped selector to disambiguate
```

---

## Project Association Rules

### Default Clearing

```
RULE: Default movement clears project association
  User must explicitly request preservation or new project

FLAGS:
  --project <id>       Set project (project_action='set')
  --keep-project       Preserve current (project_action='keep')
  --clear-project      Clear association (project_action='clear', default)
```

### Active Project Requirement

```
RULE: Can only associate items with active projects
  VALIDATE: project.status = 'active'
  Completed projects cannot accept new associations
  (Existing associations preserved if project completed)
```

---

## Temporary Use Semantics

### Origin Tracking

```
RULE: First temporary_use sets origin
  temp_origin_location_id = from_location_id
  in_temporary_use = true

RULE: Subsequent temporary_use preserves origin
  temp_origin_location_id = (unchanged)
  in_temporary_use = true
  Item can move through multiple temp locations

RULE: Rehome clears temporary state
  in_temporary_use = false
  temp_origin_location_id = NULL
  Item is now "at home" in new location
```

### Return Behavior

```
RULE: No automatic return on project completion
  User must manually move items back
  "Items to return" list shown as reminder
```

---

## Borrowed Items

### Borrowing

```
RULE: borrowed_by field REQUIRED and non-blank
  Must specify who borrowed item
  Should reference declared user (warn if not)

RULE: Item moves to system Borrowed location
  Not just a flag, actual location change
```

### Returning

```
RULE: Return via normal item.moved
  from_location_id = Borrowed
  to_location_id = (destination)
  move_type typically 'rehome'
```

---

## Missing Items

### Marking Missing

```
RULE: Item moves to system Missing location
  Records previous_location_id for suggestions

RULE: Preserves temporary use state
  If item was in temporary use before missing:
    - in_temporary_use remains true
    - temp_origin_location_id preserved
```

### Marking Found

```
RULE: Sets temporary use state
  in_temporary_use = true
  temp_origin_location_id = home_location_id (user-specified)
  Item should be returned to home eventually
```

---

## Replay Validation

### Strict Consistency Checks

```
RULE: from_location_id MUST match projection
  For: item.moved, item.borrowed, item.marked_missing, item.deleted
  Purpose: Detect concurrent writes or projection corruption
  On mismatch: FAIL replay immediately

RULE: from_parent_id MUST match projection
  For: location.reparented
  Purpose: Validate tree integrity
  On mismatch: FAIL replay immediately
```

### Failure Handling

```
RULE: On validation failure, STOP immediately
  Report: event_id, event_type, error message
  Do NOT skip event
  Do NOT attempt repair
  Require manual intervention
```

---

## Database Constraints

### Schema Enforcement

```sql
-- Location canonical uniqueness
CREATE UNIQUE INDEX idx_locations_canonical
ON locations_current(canonical_name);

-- Check constraints
ALTER TABLE projects_current
  ADD CONSTRAINT chk_status
  CHECK (status IN ('active', 'completed'));

-- Foreign key enforcement
PRAGMA foreign_keys = ON;
```

### Index Requirements

```
REQUIRED INDEXES:
  - items_current(canonical_name)
  - items_current(location_id)
  - items_current(project_id)
  - locations_current(canonical_name) UNIQUE
  - locations_current(parent_id)
  - locations_current(full_path_canonical)
  - projects_current(status)
  - events(event_type)
  - events(item_id) partial
  - events(location_id) partial
  - events(project_id) partial
```

---

## Configuration Rules

### Database Path

```
RULE: db_path MUST be absolute path
  Relative paths not allowed
  Prevents ambiguity with working directory

RULE: Must support network-mounted paths
  SQLite WAL mode compatible with NFS/SMB
  Configure busy_timeout appropriately
```

### User Identity

```
RULE: Default user from OS username
  $USER (Unix) or %USERNAME% (Windows)

RULE: os_username_map for aliasing
  Map OS username to declared user
  Warn if unmapped user used

RULE: --as flag overrides default
  No authentication required (attribution only)
```

---

## Critical Don'ts

❌ **Never modify events** - append-only log
❌ **Never skip invalid events during replay** - fail fast
❌ **Never silently repair projections** - explicit diagnosis required
❌ **Never allow location cycles** - breaks tree structure
❌ **Never auto-move items on project completion** - user decision
❌ **Never auto-create locations** - require explicit creation
❌ **Never resurrect deleted entities** - deletions are final
❌ **Never use timestamps for ordering** - event_id is authoritative

---

**Version**: 1.0 (from DESIGN.md v1)
**Last Updated**: 2026-02-19
