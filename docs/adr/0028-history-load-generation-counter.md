# History pane uses a generation counter to discard stale loads

When the history pane is open and the user navigates with Up/Down, each keypress reinitialises `historyModel` and fires a new `loadCmd()` DB fetch. Because multiple fetches can be in-flight simultaneously, out-of-order `historyLoadedMsg` responses could briefly display history for the wrong entity.

We stamp each `historyModel` with a monotonically incrementing `gen int` counter. `historyLoadedMsg` carries the `gen` value active when the load was fired. `updateBrowse` silently discards any `historyLoadedMsg` whose `gen` does not match `m.history.gen`.

## Consequences

- Stale responses are dropped at the message-handling boundary; no UI flicker.
- No debounce timer or cancellation context required — the counter is zero-cost.
- `gen` is incremented in `newHistoryModel`, which is the sole construction path.

## Considered Options

**Debounce (fire load only after idle delay).** Rejected — adds latency perceptible to keyboard users and requires a timer message type with its own cancellation logic.

**Accept out-of-order (last writer wins).** Rejected — correctness should not depend on SQLite response ordering, even when local latency is typically sub-millisecond.
