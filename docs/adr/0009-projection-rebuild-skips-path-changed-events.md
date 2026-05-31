# Projection rebuild uses a projection-only apply path that skips `propagatePathChangesTx`

`TruncateAndReplay` rebuilds `entities_current` by iterating all events in `event_id ASC` order and applying each one to the projection inside a single write transaction. It cannot reuse `applyEventTx` directly, because `handleEntityReparented` calls `propagatePathChangesTx`, which **inserts new `EntityPathChangedEvent` rows** as a side effect. Replaying reparent events through that path would:

1. Grow the event log on every rebuild (duplicate `EntityPathChangedEvent` rows accumulate indefinitely).
2. Corrupt the projection if those freshly-inserted rows were then encountered later in the same event iteration and applied a second time.

## The fix: `applyEventProjectionOnlyTx`

A separate dispatch switch, `applyEventProjectionOnlyTx`, differs from `applyEventTx` in exactly one case:

- `EntityReparentedEvent` → `handleEntityReparentedProjectionOnlyTx` — recomputes the entity's own path via `ComputeEntityPathTx` and updates `entities_current`, but does **not** call `propagatePathChangesTx` and does **not** fetch descendants.
- `EntityPathChangedEvent` → `handleEntityPathChanged` normally — updates the descendant's path in `entities_current` from the payload already in the event log. No new events are written.

All other event types are identical between the two switches.

`TruncateAndReplay` uses `applyEventProjectionOnlyTx`. The net result: the event log is untouched by a rebuild, and the projection is fully correct because the `EntityPathChangedEvent` rows already in the store carry the authoritative path data for each descendant.

## Why not parametrize `applyEventTx`

A boolean `writesEvents bool` parameter on `applyEventTx` was considered and rejected. It would entangle normal dispatch (which always writes events) with rebuild semantics at every call site. A separate named function makes the invariant explicit: callers of `applyEventProjectionOnlyTx` get a documented guarantee that no events are written.

## Relationship to `ReplayEvent` (import)

`ReplayEvent` previously had the same event-log-growth defect — it routed `EntityReparentedEvent` through `applyEventTx`, which called `propagatePathChangesTx` and inserted duplicate `EntityPathChangedEvent` rows on every import run. This was fixed in #191: `ReplayEvent` now uses a closure that routes `EntityReparentedEvent` through `handleEntityReparentedComputePayloadsTx` (updates the projection, no event writes) and all other events through `applyEventProjectionOnlyTx`. The import path-changed validation no longer relies on `GetEventsAfter` side effects; see ADR-0005.
