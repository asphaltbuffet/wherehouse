# Wherehouse — Domain Context

Wherehouse answers one question: **"Where did I put my 10mm socket wrench?"**

It is a CLI-first, event-sourced inventory tracking system. Small, deterministic, explicit, auditable.

---

## Glossary

### Entity

The single unified domain type. Everything in the inventory — a storage place, a container, a physical item — is an `Entity`. Entities have no type classification; they are distinguished only by their position in the hierarchy and their mutable attributes (`locked`, `discrete`). This is **not** split into separate "item" and "location" types.

### locked

A mutable boolean attribute on an entity. When `true`, the entity cannot be directly reparented by the user, and cannot be set to `EntityStatusMissing` or `EntityStatusLoaned` (its location is fixed by assertion). Conversely, an entity can only be locked while its status is `ok` or `borrowed` and it is not displaced, so a locked entity is never `missing`, `loaned`, or displaced. Cascade path updates from an ancestor reparent still propagate normally — a locked entity moves with its parent. Defaults to `false` at creation. Toggled via `EntityLockedEvent` / `EntityUnlockedEvent`. Locking restricts location only; name, tags, other attributes, and removal are unaffected. It says nothing about physical security (a padlocked box is not a locked entity). _Avoid_: anchored, pinned, fixed.

### discrete

A mutable boolean attribute on an entity. When `true`, the entity may not have children added to it (via `add` or `move`). Acts as a guard rail against accidental nesting — for example, a "Box of Nails" that should be tracked as a single unit. Can only be set on an entity with no live (non-`removed`) children, so a discrete entity never has live children. Defaults to `false` at creation. Can be cleared when sub-tracking is needed (e.g. pulling individual items out of a box). Toggled via `EntityDiscreteSetEvent` / `EntityDiscreteClearedEvent`.

### Eligible parent

An entity that may receive a new child (via `add`, `borrow`, or `move`): any entity that is neither `removed` nor `discrete`. Status is otherwise irrelevant — a `borrowed`, `loaned`, or `missing` entity is still an eligible parent (e.g. borrowed bits placed inside a borrowed drill case). The root (no parent) is always eligible.

### Move

Permanently relocating an entity: changing its parent (recorded as `EntityReparentedEvent`) and setting its **home** to the new parent. Move changes location only — never status. Only an entity that is not `locked` and whose status is `ok` or `borrowed` may be moved; a `missing` entity must first be found, and a `loaned` entity must first be returned. The destination must be an eligible parent (including the root) and may not be the entity itself, any of its descendants, or its current parent (a move must change location). There is one exception: a displaced entity may be moved to its current parent, which makes that parent its new home. A displaced entity may not be moved to its home, because that is a **return**.

### Home

An entity's designated location: the parent it belongs in. Every entity has a home, which is an entity reference rather than a path, so it stays valid when an ancestor is moved. A **move** sets both the parent and the home. A **use** changes only the parent.

A home is always a valid place to return to. The home of a non-removed entity is never `removed`, never `discrete`, and never the entity itself or one of its descendants. While an entity is the home of any non-removed displaced entity, removing it or setting it `discrete` is rejected. Any move or use that would place a home inside the subtree of an entity it is home to is also rejected.

### Displaced

The condition of an entity whose current parent is not its home. Displaced is derived, not stored, and it is **not** an `EntityStatus`: it is independent of status, so a `borrowed` or `missing` entity can also be displaced. A descendant that travels with a displaced ancestor is not itself displaced, because its parent has not changed.

### Use

Temporarily relocating an entity: changing its parent without changing its home, which leaves it **displaced**. Use has the same preconditions as **move**, and in addition the destination may not be the entity's home (that is a **return**). An entity that is already displaced may be used again somewhere else, and its home stays the same.

### Return

Restoring an entity to how it should be. For a displaced `ok` entity, it moves the entity home. For a `loaned` entity, it sets the status back to `ok` and, if the entity is displaced, also moves it home. For a `borrowed` entity, it removes the entity without relocating it. `return` is rejected when there is nothing to restore (an `ok` entity already at home) and for a `missing` entity, which must be found first. Finding an entity does not move it home. See [ADR 0039](docs/adr/0039-home-is-a-persistent-reference-and-displaced-is-derived.md).

### EntityStatus

The lifecycle state of an entity. Valid values:

| Value | String | Meaning |
|---|---|---|
| `EntityStatusOk` | `"ok"` | Normal, at its location |
| `EntityStatusMissing` | `"missing"` | Location unknown |
| `EntityStatusBorrowed` | `"borrowed"` | An external item brought into the inventory temporarily (a new entity is created to represent it). `borrowed` is a terminal status — the only valid transition is `return`, which sets the entity to `removed`, and only when it has no non-removed children ([ADR 0032](docs/adr/0032-return-of-borrowed-entity-requires-no-live-children.md)). All other status changes (including `ok`, `missing`, `loaned`) and direct `remove` are blocked. |
| `EntityStatusLoaned` | `"loaned"` | An existing inventory entity given out to someone else (the entity already exists; no new entity is created) |
| `EntityStatusRemoved` | `"removed"` | Soft-deleted from the inventory. A removed entity never has non-removed children: `remove`, and `return` of a `borrowed` entity, are rejected while any remain ([ADR 0032](docs/adr/0032-return-of-borrowed-entity-requires-no-live-children.md)). |

Status changes may carry an optional `StatusContext` (free-text, e.g. borrower name). For `loaned`, the recipient (`loan --to`) is **required**, mirroring `borrow --from`.

Each status is reached by exactly one intent-driven command (`lost`, `found`, `loan`, `return`, `borrow`), and each command rejects illegal source statuses rather than performing a no-op transition. The full legal transition table is recorded in [ADR 0024](docs/adr/0024-status-transition-commands-enforce-source-preconditions.md).

### Event

An immutable, append-only record of something that happened. Events are the **source of truth**. The `event_id` (integer, autoincrement) defines replay order — timestamps are informational only.

### Note

Optional free-text annotation on an `Event`. Canonically human-typed and used infrequently — for example, a one-line reason on a corrective rename. **Not** intended to hold log output, attachments, file contents, or other large blobs. Import enforces a per-record size ceiling (16 MiB) so that garbage files (e.g. binary blobs masquerading as NDJSON) fail loudly at the parser rather than at the storage layer.

### EventType

The event types currently implemented:

| Constant | Meaning |
|---|---|
| `EntityCreatedEvent` | A new entity was added |
| `EntityRenamedEvent` | An entity's display name changed |
| `EntityReparentedEvent` | An entity was moved to a new parent |
| `EntityPathChangedEvent` | Derived path update (propagated from ancestor reparent) |
| `EntityStatusChangedEvent` | Status (and optional context) changed |
| `EntityRemovedEvent` | Entity was soft-deleted |
| `EntityTagAddedEvent` | A tag was applied to an entity |
| `EntityTagRemovedEvent` | A tag was removed from an entity |
| `EntityLockedEvent` | Entity's `locked` flag set to `true` |
| `EntityUnlockedEvent` | Entity's `locked` flag set to `false` |
| `EntityDiscreteSetEvent` | Entity's `discrete` flag set to `true` |
| `EntityDiscreteClearedEvent` | Entity's `discrete` flag set to `false` |
| `EntityBorrowedEvent` | A new entity was created directly in `borrowed` status (atomic; no preceding `EntityCreatedEvent`) |
| `EntityUsedEvent` | An entity was moved to a new parent temporarily; its home did not change (a **use**) |
| `EntityReturnedHomeEvent` | A displaced entity was moved back to its home (part of a **return**) |
| `EntityRehomedEvent` | An entity's home was changed to its current parent, without a reparent |

`EntityCreatedEvent` and `EntityBorrowedEvent` set the home to the initial parent. `EntityReparentedEvent` sets both the parent and the home.

### Projection

Derived, rebuildable state computed by replaying all events in `event_id` order. Projections are disposable; authoritative state is always the event stream. Current projection tables:

- `entities_current` — current entity state (places, containers, items) and hierarchy
- `entity_tags` — current tag set per entity; populated by `EntityTagAddedEvent` / `EntityTagRemovedEvent`

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

### PathResolution

The act of translating a user-supplied colon-separated path string into an `entity_id` by canonical-name lookup. PathResolution is a CLI/UI concern — the app layer's `LookupEntityByPath` method is the single authoritative entry point. Internal mutations (create, borrow, rename, reparent, remove, status change, tag, history) accept only `entity_id` — for create and borrow, the parent's; CLI commands that receive a path from the user perform PathResolution before constructing a request. Web callers that already hold an `entity_id` bypass PathResolution entirely.

### Scry

The `scry` command searches for entities by name (fuzzy, using Levenshtein distance). It is a **search/find** command — not an inference engine for missing items. Results are `FindResult` values with `Entity` and `Distance` fields.

### Tag

A short, user-defined classification label applied to an entity. Tags are bare strings (no key-value pairs). They are stored in their canonical form only (via `CanonicalizeString`: lowercase, whitespace/`-`/`_` collapsed to `_`, leading/trailing `_` stripped). Any entity type (`place`, `container`, `leaf`) may have zero or more tags. Tags are used to group or filter entities across the hierarchy regardless of location or status.

Tag mutations are recorded as events (`EntityTagAddedEvent`, `EntityTagRemovedEvent`). Current tags are maintained in the `entity_tags` projection table (`entity_id`, `tag`), which is rebuildable from the event stream. Tags are retained when an entity is removed (soft-delete consistency).

Adding a tag that already exists, or removing a tag that is not present, is a silent no-op (warning logged). If the same tag appears in both `--add` and `--remove` in a single invocation, they cancel each other out (warning shown to user).

### Actor

The user attributed to an event. Stored as `actor_user_id` on every event. Attribution only — no access control.

- **CLI:** the configured/`--as` identity, else the (optionally mapped) OS username.
- **Web UI:** the identity asserted by a trusted reverse proxy, recorded verbatim (e.g. `alice@example.com`); `webui` when no such identity is present. The web UI never lets the visitor choose their own actor. The same person may therefore appear under different actor strings from CLI and web.

### DoctorIssue

A single structural problem found by a doctor validation method. Fields: `Kind DoctorKind`, `EventID *int64` (nil when not tied to a specific event), `Description string`.

### DoctorKind

A typed iota enum classifying which layer a `DoctorIssue` belongs to: `config`, `event_log`, or `projection`. Renders via `String()` (generated by `stringer -linecomment`).

### Healthy

The top-level verdict of a `doctor` run. `true` when no `DoctorIssue`s were found across all checks; `false` otherwise. Exposed as `healthy` in `--json` output. Determines exit code regardless of output mode.

### WherehouseName

A user-assigned display label for a database file, stored in `schema_metadata` under the key `"display_name"`. Independent of the filename. Set via `wherehouse db name <display name>`; cleared via `wherehouse db name --clear`. Never parsed as a path or entity name. When unset, displays as `(unnamed)`. Max 255 characters; newlines are rejected.

---

## Terms to avoid

| Avoid | Use instead | Why |
|---|---|---|
| "item" / "location" (as separate types) | "entity" | The model is unified; there is no separate item or location type |
| "type" (for entity classification) | "locked", "discrete" | `EntityType` was removed; entities have no type, only mutable attributes |
| "place", "container", "leaf" (as entity types) | "entity" with `locked`/`discrete` attributes | These type values no longer exist |
| "undo" | There is no undo | Corrections create new events |
| "delete" (for soft removal) | "remove" | `EntityRemovedEvent` is a status change, not a physical deletion |
| "project" | — | Not implemented |
| "fuzzy match" | "Levenshtein search" | The exact algorithm is Levenshtein distance, used in `scry`/`FindEntities` |

---

## Hard Invariants

- Events are append-only. Never mutate or delete an event row.
- Replay order is strictly by `event_id ASC`. Timestamps do not determine order.
- Every `ORDER BY` that could tie must include `event_id ASC` as a tiebreaker.
- All projection tables (`entities_current`, `entity_tags`) must be fully rebuildable by replaying the event stream. `TruncateAndReplay` truncates all projection tables before replaying.
- Entity canonical names are not globally unique and uniqueness within a parent is not enforced. Multiple entities may share a name (e.g. three identical "10mm socket" entities in the same drawer) — each is distinct by `entity_id` and has its own history and status. Disambiguation between same-name entities is a UI concern, not a data model concern.
- Every non-removed entity's home is non-removed, non-discrete, and not the entity itself or one of its descendants.
- Path propagation is recursive: reparenting an entity triggers `EntityPathChangedEvent` for all descendants.
- `EntityRemovedEvent` sets `status = "removed"`. Removed entities remain in `entities_current` (soft delete).
- No silent repair. No application-level auto-retry: SQLite lock contention is absorbed only by the connection's busy timeout, after which the operation fails.
- Timestamps stored as RFC3339 UTC with `Z` suffix.
- DB path must be absolute. SQLite is canonical storage.
- Single host: every process that opens a database must run on the same host. The file may live on a network mount only if no other host opens it. Users on other machines share an inventory through `serve`, not by opening the file.
