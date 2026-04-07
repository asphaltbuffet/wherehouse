# Domain Model

**Source**: docs/DESIGN.md
**Purpose**: Core entities, identifiers, canonicalization rules, and relationships

---

## Core Entities

### Item

**Identity**:
- `item_id` = UUID (v7 preferred)
- Globally unique, stable across moves

**Naming**:
- `display_name` - preserved exactly as entered (may include emoji)
- `canonical_name` - normalized for matching
- Names NOT unique (multiple items can share canonical name)
- **Constraint**: `:` NOT allowed in item names (conflicts with selector syntax)

**State** (in projection):
- `location_id` - current immediate location (not full path)
- `in_temporary_use` - boolean
- `temp_origin_location_id` - original location for temporary use
- `project_id` - current project association (nullable)
- `last_event_id` - replay checkpoint
- `updated_at` - projection timestamp

**Lifecycle**:
- Created via `item.created` event
- Moved via `item.moved` event
- Borrowed via `item.borrowed` event
- Can be marked missing via `item.marked_missing`
- Can be marked found via `item.marked_found`
- Can be removed via `item.removed` (moves to "Removed" system location)

**Duplicate Name Handling**:
- Warn when `canonical_name` already exists (warning only, not error)
- Warn when duplicate `canonical_name` in same location
- Disambiguation via location-scoped selector: `LOCATION:ITEM`

---

### Location

**Identity**:
- `location_id` = UUID
- Globally unique

**Naming**:
- `display_name` - preserved as entered
- `canonical_name` - **globally unique** (enforced)
- No colons in names (separator reserved for paths)

**Structure**:
- Hierarchical tree
- `parent_id` - nullable (NULL = root location)
- Must be acyclic (cycles rejected)
- Unlimited depth supported

**Path Representation**:
- Separator: `:` (colon)
- Examples: `Basement:Toolbox`, `Garage:Workbench:Drawer`
- Full path cached in projection as `full_path_display` and `full_path_canonical`

**Special System Locations**:
- `Missing` - for lost items
- `Borrowed` - for borrowed items
- `Removed` - for removed items
- All are real rows in locations table
- `is_system = true`
- Cannot be removed, renamed, or reparented
- Root-level only (no parent)

**Capabilities**:
- Can contain items
- Can contain sub-locations
- Can be reparented via `location.reparented` event
- Can be removed via `location.removed` (only if empty)

**Removal Rules**:
- Only allowed if:
  - No items present
  - No sub-locations exist

**Creation**:
- Parent locations NOT auto-created
- Use `--parents` flag for recursive creation

---

### Project

**Identity**:
- `project_id` = user-provided slug (NOT UUID)
- Globally unique
- **Constraint**: Cannot contain `:`

**State**:
- `status`: `active` | `completed`
- Can transition: `active → completed → reopened → ...`
- No restrictions on transitions (can reopen completed projects)

**Lifecycle Events**:
- `project.created`
- `project.completed`
- `project.reopened`

**Item Association**:
- Items associated via `project_id` field in item projection
- Association set during `item.moved` with `--project` flag
- Default movement behavior: clears project
- Explicit flags control association:
  - `--project <id>` - set project
  - `--keep-project` - preserve current project
  - `--clear-project` - remove project (default)

**Completion Behavior**:
- Display "items to return" list (items with `project_id` = this project)
- Does NOT automatically move items
- User must manually return items

**No Removal**:
- Projects cannot be removed; use `project.completed` to close out a project

---

### User Identity

**Model**: Attribution only (no permissions in v1)

**Identity**:
- `user_id` = string (username)
- Default: OS username (`$USER` or `%USERNAME%`)
- Override: `--as <user_id>` flag

**Configuration**:
- `[user_identity.os_username_map]` - map OS usernames to declared users
- If unmapped → warn but record OS username

**Special User for Borrowed Events**:
- `item.borrowed` requires `borrowed_by` field (cannot be empty)
- Must be explicit user identity

**No Permissions**:
- All users can perform all operations
- No authentication
- No access control
- Attribution for tracking only

---

## Canonicalization Rules

### Item Names

```
1. Trim leading/trailing whitespace
2. Case-insensitive (lowercase)
3. Collapse internal whitespace runs to single `_`
4. Normalize separators (-, _, space) to `_`
5. Strip/normalize other punctuation (consistently)
6. Result: ASCII-safe comparable string
```

**Examples**:
- `"10mm Socket Wrench"` → `"10mm_socket_wrench"`
- `"screwdriver - phillips"` → `"screwdriver_phillips"`
- `"Tool Box"` → `"tool_box"`

**Display Name Preservation**:
- Original entered as `"10mm socket 🔧"` stored in `display_name`
- Canonical form used for matching only

### Location Names

**Same rules as item names**, plus:
- **Global uniqueness enforced** on `canonical_name`
- Prevents ambiguous path resolution

**Path Canonicalization**:
- Each segment canonicalized independently
- Joined with `:` separator
- Example: `"Garage:Tool Box:Drawer 1"` → `"garage:tool_box:drawer_1"`

---

## Selector Syntax

**Purpose**: Disambiguate items with duplicate names

**Formats**:

1. **By ID**: `--id <ITEM_ID>`
   - Always unambiguous
   - UUID format

2. **Location-scoped**: `LOCATION:ITEM`
   - Both resolved via canonical names
   - Example: `tote_f:10mm_socket`
   - Handles spaces: `"Garage:10mm Socket"` or `Garage:10mm_socket`

**Matching**:
- Exact match on canonical name (no fuzzy, no substring)
- Case-insensitive (via canonicalization)
- No partial matches in command parsing
- (Completion layer may use fuzzy/fzf, but execution is exact)

**Multiple Matches**:
- If selector matches multiple items → return all
- User should use location-scoped selector to disambiguate

---

## Entity Relationships

```
Item
  - belongs_to Location (current_location_id)
  - has_many MovementEvents (via item_id)
  - belongs_to Project (optional, via project_id)
  - has_one TemporaryOrigin (via temp_origin_location_id, when in_temporary_use)

Location
  - has_many Items
  - has_many Locations (children, via parent_id)
  - belongs_to Location (parent, via parent_id)
  - has_many MovementEvents (from/to)

Project
  - has_many Items (via items.project_id in projection)
  - has_many MovementEvents (via movement.project_id in event log)

Event
  - references Item (for item events)
  - references Location (for location events)
  - references Project (optional, for item.moved)
  - has_actor User (via actor_user_id)
```

---

## Constraints Summary

| Constraint | Enforcement |
|------------|-------------|
| `item.canonical_name` unique | No (warn only) |
| `location.canonical_name` unique | Yes (global) |
| `project.project_id` unique | Yes (global) |
| `:` in item names | Forbidden |
| `:` in project IDs | Forbidden |
| Location tree acyclic | Yes (validated) |
| System locations removable | No (forbidden) |
| System locations renamable | No (forbidden) |
| Item references valid location | Yes (FK-like) |
| Project removal | No (forbidden) |
| Location removal with children | No (forbidden) |
| Location removal with items | No (forbidden) |

---

## Special Semantics

### Temporary Use Tracking

**First `temporary_use` move**:
- Sets `in_temporary_use = true`
- Sets `temp_origin_location_id` = previous location
- Associates with project (optional)

**Subsequent `temporary_use` moves**:
- Preserves original `temp_origin_location_id`
- Item can move through multiple temporary locations
- Origin remains the first location before temporary use began

**Rehome clears temporary state**:
- Sets `in_temporary_use = false`
- Clears `temp_origin_location_id`
- Item is now "at home" in new location

### Borrowed Items

**Borrowing**:
- Special event type: `item.borrowed`
- Moves item to system `Borrowed` location
- Requires `borrowed_by` user identity (cannot be blank)
- Records `from_location_id` (not temp_origin)

**Returning**:
- Normal `item.moved` event from `Borrowed` to real location
- Move type typically `rehome` (unless returned to temp use)

### Missing Items

**Marking Missing**:
- Event: `item.marked_missing`
- Moves item to system `Missing` location
- Records `previous_location_id`

**Marking Found**:
- Event: `item.marked_found`
- Moves item from `Missing` to `found_location_id`
- Sets `in_temporary_use = true`
- Sets `temp_origin_location_id = home_location_id` (specified)

### Project Clearing Behavior

**Default**: Movement clears project association
**Override**:
- `--project <id>` - set new project
- `--keep-project` - preserve current project
- `--clear-project` - explicit clear (default behavior)

**Encoded in Event**:
- `project_action` enum: `clear` | `keep` | `set`
- `project_id` field (nullable)

---

**Version**: 1.0 (from DESIGN.md v1)
**Last Updated**: 2026-02-19
