# WHEREHOUSE — Design Reference Document (v1)

## Summary

Wherehouse is a CLI-first, event-sourced inventory tracking system designed to answer a single core question:

> "Where did I put my 10mm socket wrench?"

It is:

- Event-sourced (events are the source of truth)
- Deterministic (replay order defined strictly by `event_id`)
- Multi-user (attribution-only, no permissions)
- SQLite-backed (network-mount compatible)
- Projection-driven for fast reads
- Strict about integrity and concurrency
- Explicit about lifecycle transitions
- CLI-first, with a TUI wrapper over the CLI layer

Wherehouse favors:

- Determinism over magic
- Explicit commands over silent behavior
- Strong invariants over convenience
- Human-readable defaults with machine-readable output options

## PURPOSE

Wherehouse answers:

```
Where did I put my 10mm socket wrench?
```

Primary daily workflows:

- Fast lookup (`where`)
- Intentional movement with context
    - Temporary use
    - Rehome
    - Borrowed
- Mark missing and found
- Associate items with projects

## ARCHITECTURAL PHILOSOPHY

- Events are the source of truth.
- Current state is a projection derived from events.
- No undo. Corrections create new events.
- SQLite is canonical storage.
- DB path must be absolute.
- Must support network-mounted DB.
- Timestamps stored as UTC (RFC3339 with `Z`).
- Replay ordering is strictly by `event_id`.
- Projections are disposable and rebuildable.
- No silent repair.

## DOMAIN MODEL

### ITEMS

- Individually tracked entities.
- `item_id` = UUID (v7 preferred).
- `display_name` preserved exactly as entered.
- `canonical_name` normalized for matching.
- Duplicate canonical names allowed (warn only).
- Duplicate canonical names in same location → warn.
- `:` not allowed in item names.
- Matching:
    - Case-insensitive exact match on canonical name.
- Emoji allowed in `display_name`.

Item creation requires explicit location:
```
wherehouse item create "10mm socket wrench" --location Basement:Toolbox
```

### LOCATIONS

- Hierarchical tree.
- `location_id` = UUID.
- Fields:
    - `display_name`
    - `canonical_name` (globally unique)
    - `parent_id`
    - `is_system` (for reserved locations)

Canonicalization rules:

- Case-insensitive
- Trim whitespace
- Collapse internal whitespace to `_`
- Normalize separators (`-`, `_`, space) to `_`

Paths use `:` as separator:

```
Basement:Toolbox
Garage:Workbench:Drawer
```

System locations:

- `Missing`
- `Borrowed`
- Stored as real rows
- `is_system = true`
- Cannot be renamed, deleted, or reparented

Location deletion:

- Only allowed if:
    - No sublocations
    - No items present

Creation with parent auto-creation requires `--parents` flag.

### EVENT TYPES

All events may include optional `note`.

#### item.created

- `item_id`
- `display_name`
- `canonical_name`
- `location_id`
- `actor_user_id`
- `timestamp_utc`
- optional `note`

#### item.moved

- `item_id`
- `from_location_id`
- `to_location_id`
- `move_type`:
    - `temporary_use`
    - `rehome`
- `project_action`: `clear` | `keep` | `set`
- `project_id` (nullable)
- `actor_user_id`
- `timestamp_utc`
- optional `note`

Temporary use semantics:

- First `temporary_use` sets `temp_origin_location_id`
- Subsequent temporary uses preserve original origin
- Rehome clears temporary state

#### item.borrowed

- `item_id`
- `from_location_id`
- `borrowed_by` (required, non-blank)
- `actor_user_id`
- `timestamp_utc`
- optional `note`

Return = normal `item.moved` from `Borrowed`.

#### item.missing

- `item_id`
- `previous_location_id`
- `actor_user_id`
- `timestamp_utc`
- optional `note`

Projection moves item to `MISSING`.

#### item.found

- `item_id`
- `found_location_id`
- `home_location_id`
- `actor_user_id`
- `timestamp_utc`
- optional `note`

Projection:

- `in_temporary_use = true`
- `temp_origin_location_id = home_location_id`

#### item.deleted

- `item_id`
- `previous_location_id`
- `actor_user_id`
- `timestamp_utc`
- optional `note`

Projection removes item permanently.

#### location.created

- `location_id`
- `display_name`
- `canonical_name`
- `parent_id` (nullable for root locations)
- `is_system` (boolean, should only be true for system locations)
- `actor_user_id`
- `timestamp_utc`
- optional `note`

Validation:
- `canonical_name` must be globally unique
- `canonical_name` must not contain `:`
- `parent_id` must exist if not NULL
- Must not create cycles

Projection computes `full_path_display`, `full_path_canonical`, and `depth` by walking up from parent to root.

#### location.reparented

- `location_id`
- `from_parent_id`
- `to_parent_id`
- `actor_user_id`
- `timestamp_utc`
- optional `note`

Must reject cycles.

#### location.deleted

- `location_id`
- `previous_parent_id`
- `actor_user_id`
- `timestamp_utc`
- optional `note`

Only allowed if empty.

#### project.created

- `project_id` (user-provided slug)
- `actor_user_id`
- `timestamp_utc`

#### project.completed

- `project_id`
- `actor_user_id`
- `timestamp_utc`

Return-needed list = items currently associated.

#### project.reopened

- `project_id`
- `actor_user_id`
- `timestamp_utc`

#### project.deleted

- `project_id`
- `actor_user_id`
- `timestamp_utc`

Only allowed if no items currently associated.

## PROJECTION STRATEGY

Projection tables:

### locations_current

- `location_id`
- `display_name`
- `canonical_name`
- `parent_id`
- `full_path_display`
- `full_path_canonical`
- `depth`
- `is_system`
- `updated_at`

### items_current

- `item_id`
- `display_name`
- `canonical_name`
- `location_id`
- `in_temporary_use`
- `temp_origin_location_id`
- `project_id`
- `last_event_id`
- `updated_at`

### projects_current

- `project_id`
- `status`
- `updated_at`

Replay rules:

- Replay strictly by `event_id`.
- Validate `from_location_id`.
- Fail on inconsistency.
- Projections must match rebuild during `doctor`.

## PROJECTS

- ID = user-provided slug.
- Globally unique.
- Cannot contain `:`.
- Must be active to associate with items.
- Default movement clears project.
- Explicit flags:
    - `--project`
    - `--keep-project`
    - `--clear-project`

Example:

```
wherehouse move "step ladder" Garage --project change_light_bulb
```

## MULTI-USER MODEL

- Attribution only.
- Default user from OS username.
- `--as <user_id>` must reference declared user.
- Config supports OS username mapping.
- If unmapped → warn but record OS username.

## CLI CONTRACT

Command name: `wherehouse`

Verb-first.

All commands:

- Non-interactive safe.
- `--json`
- `-q`, `-qq`
- `-v`, `-vv`

Selector syntax:

```
LOCATION:ITEM
```

No fuzzy parsing.

Example:

```
Basement:Toolbox:"10mm socket wrench"
```

Multiple matches return all.

## CONFIGURATION (TOML via spf13/viper)

Resolution order:

1. CLI flags
2. Environment variables (`WHEREHOUSE_*`)
3. `--config`
4. `./wherehouse.toml`
5. XDG config
6. Defaults

Required:

```toml
config_version = 1
```

Configurable:

- `db_path` (absolute only)
- `sqlite_journal_mode`
- `[users.*]`
- `[user_identity.os_username_map]`
- `default_grouping`
- `logging_level`

Env override supported.

## TUI

- Thin wrapper over CLI layer.
- No direct event writes.
- No duplicated domain logic.

Architecture:

```
TUI -> CLI Layer -> Domain Engine
```

## EXPORT / IMPORT

`wherehouse export`:

Includes:

- Events (ordered)
- Projection snapshot
- Schema version
- Config version
- Checksums

Future import:

- Default rebuild projection
- `--fast` trusts snapshot

## DOCTOR

`wherehouse doctor`

Validates:

- Structural integrity
- Event stream integrity
- Projection consistency
- Transition validation

No silent repair.
Non-zero exit on failure.

## HISTORY

`wherehouse history <item>`

Default:

- Semantic summaries
- icons for event type:
    - `MISSING` (⁇)
    - `FOUND` (󰍉)
    - `MOVE` (󱞬)
    - `RETURN` (󱞤)
    - `CREATE`/`ADD` (󰙴)
    - `BORROW` ()
    - `DELETED`/`REMOVED` (󰆴)
    - `TEMPORARY_USE` (󱁤)
`--raw`:
    - Exact event types
    - Full fields

Example:

```
󰙴  Basement:Toolbox
󱁤  Basement:Toolbox → Kitchen
⁇  Last known: Kitchen
󰍉  Found in Garage (home: Basement:Toolbox)
```

## KEY DECISIONS, TRADE-OFFS, AND NOTES

### Event Spine Authority

- `event_id` defines truth ordering.
- Timestamps are informational.
- Replay determinism prioritized over clock accuracy.
- UUID Entities + Integer Event IDs
- UUID for identity portability.
- Integer for ordering and replay stability.

### No Silent Magic

- No auto-repair.
- No implicit retries.
- No implicit project carry-over.
- No resurrection of deleted entities.

### Strict Integrity

- `from_location_id` stored and validated.
- Per-item write serialization.
- Projection mismatch = failure.

### Global Canonical Location Names

- Simpler matching.
- No ambiguous paths.
- Slightly less filesystem-like.

### Projects as Ephemeral Context

- Default clear on move.
- Explicit carry-forward required.
- Lightweight, reusable.

### Deletions are Final

- Items: deletable anytime.
- Locations: only when empty.
- Projects: only when no associations.

### Human-Centered Defaults

- History shows interpreted summaries.
- Project visible in `where`.
- Examples embedded in help.
- UUID hidden unless requested.

### Determinism Over Convenience

- Exact matching only.
- No fuzzy parsing.
- No ambiguous resolution.

---

Wherehouse v1 is:

- Small
- Deterministic
- Explicit
- Auditable
- Rebuildable
- Network-safe
- CLI-native

It is not trying to be everything.
It is trying to be correct.
