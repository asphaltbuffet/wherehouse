# Wherehouse — Domain Context

Wherehouse answers one question: **"Where did I put my 10mm socket wrench?"**

It is a CLI-first, event-sourced inventory tracking system. Small, deterministic, explicit, auditable.

---

## Glossary

### Entity

The single unified domain type. Everything in the inventory — a storage place, a container, a physical item — is an `Entity`. Entities are distinguished by their `EntityType`. This is **not** split into separate "item" and "location" types.

### EntityType

The three values that classify an entity:

| Value | String | Meaning |
|---|---|---|
| `EntityTypePlace` | `"place"` | A top-level storage place (e.g. Garage, Basement) |
| `EntityTypeContainer` | `"container"` | A container within a place (e.g. Toolbox, Drawer) |
| `EntityTypeLeaf` | `"leaf"` | A tracked physical object with no children |

### EntityStatus

The lifecycle state of an entity. Valid values:

| Value | String | Meaning |
|---|---|---|
| `EntityStatusOk` | `"ok"` | Normal, at its location |
| `EntityStatusMissing` | `"missing"` | Location unknown |
| `EntityStatusBorrowed` | `"borrowed"` | Lent to someone temporarily |
| `EntityStatusLoaned` | `"loaned"` | Out on loan |
| `EntityStatusRemoved` | `"removed"` | Soft-deleted from the inventory |

Status changes may carry an optional `StatusContext` (free-text, e.g. borrower name).

### Event

An immutable, append-only record of something that happened. Events are the **source of truth**. The `event_id` (integer, autoincrement) defines replay order — timestamps are informational only.

### EventType

The six event types currently implemented:

| Constant | Meaning |
|---|---|
| `EntityCreatedEvent` | A new entity was added |
| `EntityRenamedEvent` | An entity's display name changed |
| `EntityReparentedEvent` | An entity was moved to a new parent |
| `EntityPathChangedEvent` | Derived path update (propagated from ancestor reparent) |
| `EntityStatusChangedEvent` | Status (and optional context) changed |
| `EntityRemovedEvent` | Entity was soft-deleted |

### Projection

The `entities_current` table — derived, rebuildable state computed by replaying all events in `event_id` order. It is the **only** projection table. Projections are disposable; authoritative state is always the event stream.

### Path

A colon-separated string identifying an entity's position in the hierarchy (e.g. `Garage:Toolbox:Drawer`). Represented in code as `entitypath.Path`. The `Base()` method returns the leaf segment; `Dir()` returns the parent path.

### CanonicalName

The normalized form of a name used for matching. Rules (from `CanonicalizeString`): lowercase, trim whitespace, collapse internal whitespace/`-`/`_` to single `_`, strip leading/trailing `_`. `CanonicalName` is the leaf segment only; `FullPathCanonical` is the full colon-separated canonical path.

### DisplayName vs FullPathDisplay

- `DisplayName` — the leaf name exactly as the user entered it (case/spacing preserved)
- `FullPathDisplay` — the full colon-separated path using display names (e.g. `"Garage:Toolbox"`)
- `CanonicalName` — the normalized leaf name only
- `FullPathCanonical` — the full colon-separated canonical path

**Do not confuse `CanonicalName` with `FullPathCanonical`.** Use `FullPathDisplay` when checking depth or path structure in `EntityResult`.

### Scry

The `scry` command searches for entities by name (fuzzy, using Levenshtein distance). It is a **search/find** command — not an inference engine for missing items. Results are `FindResult` values with `Entity` and `Distance` fields.

### Actor

The user attributed to an event. Defaults to the OS username. Stored as `actor_user_id` on every event. Attribution only — no access control.

---

## Terms to avoid

| Avoid | Use instead | Why |
|---|---|---|
| "item" / "location" (as separate types) | "entity" | The model is unified; there is no separate item or location type |
| "undo" | There is no undo | Corrections create new events |
| "delete" (for soft removal) | "remove" | `EntityRemovedEvent` is a status change, not a physical deletion |
| "tag" / "project" | — | Not implemented |
| "fuzzy match" | "Levenshtein search" | The exact algorithm is Levenshtein distance, used in `scry`/`FindEntities` |

---

## Hard Invariants

- Events are append-only. Never mutate or delete an event row.
- Replay order is strictly by `event_id ASC`. Timestamps do not determine order.
- Every `ORDER BY` that could tie must include `event_id ASC` as a tiebreaker.
- `entities_current` is the sole projection table. It must be rebuildable from the event stream.
- Entity canonical names are not globally unique across all types — but uniqueness within a parent is enforced at the application layer.
- Path propagation is recursive: reparenting an entity triggers `EntityPathChangedEvent` for all descendants.
- `EntityRemovedEvent` sets `status = "removed"`. Removed entities remain in `entities_current` (soft delete).
- No silent repair. No auto-retry beyond the `WithRetry` helper for SQLite locking.
- Timestamps stored as RFC3339 UTC with `Z` suffix.
- DB path must be absolute. SQLite is canonical storage, compatible with network mounts.
