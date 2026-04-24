# Entity Consolidation Design

**Date:** 2026-04-20  
**Status:** Draft  
**Scope:** Replace separate `item` and `location` types with a unified `entity` model.

---

## Context

Wherehouse currently models inventory using two distinct types: `item` (movable, stateful) and `location` (immovable, hierarchical). These share significant structure (names, IDs, event sourcing, path-like addressing) but are implemented as entirely separate tables, event namespaces, and CLI commands.

The goal is to unify them into a single `entity` type distinguished by an `entity_type` enum. This enables richer real-world modeling: a Toolbox is both a movable object *and* a container for other objects. The existing dual-type model cannot express this.

No migration is required — existing databases must be reset. This is acceptable at the current stage.

---

## Entity Type Enum

```go
type EntityType int

const (
    EntityTypePlace     EntityType = iota // immovable; nestable only within other places
    EntityTypeContainer                   // movable; can contain any entity type
    EntityTypeLeaf                        // movable; cannot contain other entities (reserved, not enforced yet)
)
```

- **`place`** — Immovable. Can only be nested inside other `place` entities (or be top-level). Examples: `Garage`, `Garage::Floor`, `Workshop::Shelf`.
- **`container`** — Movable. Can contain any entity type. Examples: `Toolbox`, `Drillbit case`.
- **`leaf`** — Movable. Cannot contain other entities. Scaffolded now; containment enforcement deferred.

---

## Entity Status Enum

```go
type EntityStatus int

const (
    EntityStatusOk      EntityStatus = iota
    EntityStatusBorrowed
    EntityStatusMissing
    EntityStatusLoaned
    EntityStatusRemoved
)
```

Replaces the system locations `Missing`, `Borrowed`, `Loaned`, `Removed`. Status is a first-class field on the entity, not a containment relationship.

An optional `status_context` TEXT field carries free-form metadata (e.g., "loaned to Alice", "borrowed by Bob").

---

## Data Model

### `entities_current` projection table

| Column               | Type                  | Notes                                              |
|----------------------|-----------------------|----------------------------------------------------|
| `entity_id`          | TEXT (nanoid) PK      |                                                    |
| `display_name`       | TEXT                  | User-facing name                                   |
| `canonical_name`     | TEXT                  | Lowercased, normalized, whitespace-collapsed       |
| `entity_type`        | `EntityType` (iota)   | `place`, `container`, `leaf`                       |
| `parent_id`          | TEXT (nanoid) nullable FK | NULL = top-level                               |
| `full_path_display`  | TEXT                  | `Garage::Toolbox::screwdriver` (display names)     |
| `full_path_canonical`| TEXT                  | Same, using canonical names                        |
| `depth`              | INTEGER               | 0 = top-level                                      |
| `status`             | `EntityStatus` (iota) | `ok`, `borrowed`, `missing`, `loaned`, `removed`   |
| `status_context`     | TEXT nullable         | Free-form status metadata                          |
| `last_event_id`      | INTEGER               | Event causality tracking                           |
| `updated_at`         | TEXT (RFC3339)        | Informational timestamp                            |

**No unique index on `full_path_canonical`** — duplicate names within the same parent are valid (e.g., two screwdrivers in a Toolbox). Entity identity is always the nanoid.

**No unique index on `canonical_name`** — same name may appear anywhere in the tree.

### Constraints (enforced in event handlers, not DB)

- `place` entities may only have a `place` parent (or NULL).
- `container` and `leaf` entities may have any parent type.
- `leaf` entities may not be the `parent_id` of any other entity (deferred enforcement).

---

## Events

All events use the `entity.` prefix. Existing `item.*` and `location.*` event types are retired.

### `entity.created`

```json
{
  "entity_id": "<nanoid>",
  "display_name": "Toolbox",
  "entity_type": "container",
  "parent_id": "<nanoid or null>"
}
```

Triggers DB bootstrap if no database file exists. Inserts into `entities_current`. Recomputes `full_path_*` and `depth` from parent chain.

### `entity.renamed`

```json
{
  "entity_id": "<nanoid>",
  "display_name": "Big Toolbox"
}
```

Updates `display_name`, `canonical_name`, `full_path_*` for entity and all descendants. Emits `entity.path_changed` for each affected descendant.

### `entity.reparented`

```json
{
  "entity_id": "<nanoid>",
  "parent_id": "<nanoid or null>"
}
```

Moves entity to a new parent (or top-level). Recomputes `full_path_*` and `depth` for entity and all descendants. Emits `entity.path_changed` for each descendant. Forbidden for `place` entities.

### `entity.path_changed`

```json
{
  "entity_id": "<nanoid>",
  "full_path_display": "Workshop::Toolbox::screwdriver",
  "full_path_canonical": "workshop::toolbox::screwdriver",
  "depth": 2
}
```

A **derived event** — never directly user-initiated. Emitted for each descendant when an ancestor is renamed or reparented. Provides the audit trail for `scry` and `history` to show the full movement story of an entity even when it moved because an ancestor moved.

### `entity.status_changed`

```json
{
  "entity_id": "<nanoid>",
  "status": "loaned",
  "status_context": "loaned to Alice"
}
```

Replaces `item.borrowed`, `item.loaned`, `item.missing`, `item.found`. Transition back to `ok` (returned, found) uses `status=ok` with no `status_context`.

### `entity.removed`

```json
{
  "entity_id": "<nanoid>"
}
```

Sets `status=removed`. Forbidden if the entity has any non-removed children (enforced in handler).

---

## CLI

### Command reference

| Command    | Syntax                                                          | Notes                                                                 |
|------------|-----------------------------------------------------------------|-----------------------------------------------------------------------|
| `add`      | `wherehouse add <name> [--in <id>] [--type <place\|container\|leaf>]` | Default type: `container`. Bootstraps DB on first run.        |
| `move`     | `wherehouse move <id> --to <id>`                                | `entity.reparented`. Forbidden for `place` entities.                 |
| `status`   | `wherehouse status <id> --set <status> [--note <text>]`         | `entity.status_changed`. Covers borrow, loan, missing, found, return.|
| `rename`   | `wherehouse rename <id> --to <name>`                            | `entity.renamed`.                                                     |
| `remove`   | `wherehouse remove <id>`                                        | `entity.removed`. Forbidden if non-removed children exist.           |
| `scry`     | `wherehouse scry [<name>]`                                      | Search by canonical name; shows all matches with ID and path. No arg = show all. |
| `list`     | `wherehouse list [--under <id>] [--type <...>] [--status <...>]`| Filtered listing.                                                    |
| `history`  | `wherehouse history <id>`                                       | Event log for entity, including `path_changed` events.               |

### Addressing

- **All mutation commands** (`move`, `status`, `rename`, `remove`) require an **entity ID** (nanoid). This is unambiguous regardless of duplicate names.
- **`scry`** accepts a name (canonical) and returns all matches with their IDs and paths, so the user can identify the correct ID for subsequent commands.
- **`--in`** on `add` accepts an ID or an unambiguous canonical name (errors if multiple entities match).
- Shell autocompletion over IDs is the intended UX for reducing friction.

### Removed commands

| Old command     | Replacement                          |
|-----------------|--------------------------------------|
| `initialize`    | Gone — DB bootstraps on first write  |
| `add item`      | `add --type container` (default)     |
| `add location`  | `add --type place`                   |
| `found`         | `status <id> --set ok`               |
| `loan`          | `status <id> --set loaned --note <person>` |
| `lost`          | `status <id> --set missing`          |
| `remove item`   | `remove <id>`                        |
| `remove location` | `remove <id>`                      |

### Default type rationale

Defaulting `add` to `container` means `wherehouse add screwdriver` works for the common case without flags. Users explicitly opt into `--type place` for immovable locations. `leaf` is scaffolded in the enum but not yet enforced in containment logic.

---

## DB Bootstrap

When any write command runs and no database file exists:

1. Open/create the SQLite file.
2. Run all migrations (schema creation).
3. Proceed with the command normally.

No `initialize` command. No seed data. The first entity the user creates is the first record.

---

## What Is Deferred

- **`leaf` containment enforcement** — the `EntityTypeLeaf` value is defined and stored, but the handler does not yet reject children of leaf entities.
- **Shell autocompletion** — referenced as the UX mitigation for ID-based addressing, but implementation is out of scope for this refactor.

---

## Verification

1. `wherehouse add Garage --type place` — creates top-level place entity, bootstraps DB.
2. `wherehouse add Toolbox --in <garage-id>` — creates container inside place.
3. `wherehouse add screwdriver --in <toolbox-id>` — creates container inside container.
4. `wherehouse add screwdriver --in <toolbox-id>` — creates second screwdriver; no uniqueness error.
5. `wherehouse scry screwdriver` — shows both screwdrivers with distinct IDs and paths.
6. `wherehouse move <toolbox-id> --to <workshop-id>` — reparents toolbox; verify path of screwdriver updated; verify `entity.path_changed` events in history.
7. `wherehouse status <id> --set loaned --note "loaned to Alice"` — verify status + context stored.
8. `wherehouse remove <screwdriver-id>` — succeeds (no children).
9. `wherehouse remove <toolbox-id>` — fails (non-removed children exist).
10. `wherehouse add screwdriver` with no DB file — bootstraps DB, creates top-level entity.
