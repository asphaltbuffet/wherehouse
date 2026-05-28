# Import uses a new `ReplayEvent` bus method, not `Dispatch`

The `import` command restores inventory state by replaying events from an NDJSON export. It cannot use the existing `eventbus.Dispatch` method because `Dispatch` stamps `time.Now()` as `timestamp_utc`, destroying the original event timestamp from the export. Timestamps are informational (replay order is by `event_id ASC`), but losing them would make the restored event log differ from the source in a way that is confusing and unnecessary.

We added a `Bus.ReplayEvent(ctx, *inventory.Event) (int64, error)` method that accepts a fully-populated event (including original `TimestampUTC`), inserts it with that timestamp, and then calls the same `applyEventTx` handler as `Dispatch` to maintain projections. The new `event_id` assigned by SQLite replaces the exported one (autoincrement surrogates are not preserved across databases).

`ReplayEvent` is the only sanctioned write path for import. Direct store insertion was rejected because it would bypass projection maintenance entirely, requiring a separate rebuild pass that does not exist.
