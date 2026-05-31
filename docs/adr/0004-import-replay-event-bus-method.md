# Import uses a new `ReplayEvent` bus method, not `Dispatch`

The `import` command restores inventory state by replaying events from an NDJSON export. It cannot use the existing `eventbus.Dispatch` method because `Dispatch` stamps `time.Now()` as `timestamp_utc`, destroying the original event timestamp from the export. Timestamps are informational (replay order is by `event_id ASC`), but losing them would make the restored event log differ from the source in a way that is confusing and unnecessary.

We added a `Bus.ReplayEvent(ctx, *inventory.Event) (int64, error)` method that accepts a fully-populated event (including original `TimestampUTC`), inserts it with that timestamp, and then calls the same `applyEventTx` handler as `Dispatch` to maintain projections. The new `event_id` assigned by SQLite replaces the exported one (autoincrement surrogates are not preserved across databases).

`ReplayEvent` is the only sanctioned write path for import. Direct store insertion was rejected because it would bypass projection maintenance entirely, requiring a separate rebuild pass that does not exist.

## Implementation note: shared writeEvent helper

Both `Dispatch` and `ReplayEvent` delegate to a private `Bus.writeEvent(ctx, *inventory.Event, applyFn) (int64, error)` helper that owns the INSERT + `LastInsertId` + apply-function sequence. The public separation between `Dispatch` and `ReplayEvent` remains because the two paths differ in how they source `TimestampUTC` and `EntityID`, and in which apply function they use:

- `Dispatch` stamps `time.Now()`, parses `entity_id` from the payload, and calls `applyEventTx` (which writes derived `EntityPathChangedEvent` rows as a side effect).
- `ReplayEvent` uses the values already on the supplied `*inventory.Event` and passes a closure as the apply function: `EntityReparentedEvent` is routed to `handleEntityReparentedComputePayloadsTx` (updates projection, no event writes, returns computed payloads); all other events go through `applyEventProjectionOnlyTx`.

## ReplayEvent return signature

`ReplayEvent` returns `(int64, []EntityPathChangedPayload, error)`. The payload slice is non-nil only when the replayed event is an `EntityReparentedEvent`; it carries the expected `EntityPathChangedPayload` for each affected descendant, computed from the post-reparent projection state. The import layer uses these to validate the path-changed records buffered from the export stream without relying on `GetEventsAfter` side effects. See ADR-0005.
