# Import skips `EntityPathChangedEvent` records and uses them for validation

`EntityPathChangedEvent` is a derived event: `propagatePathChangesTx` generates and inserts these records automatically as a side effect of handling `EntityReparentedEvent`. The export contains both the triggering reparent event and its derived path-changed events.

Replaying path-changed events via `ReplayEvent` during import would corrupt the projection: `handleEntityReparented` already regenerates them, so each descendant's path would be updated twice — once by the reparent handler and once by the explicit path-changed replay.

Instead, import skips `EntityPathChangedEvent` records during replay. The bus recomputes the correct descendant paths when each `EntityReparentedEvent` is replayed (via `handleEntityReparentedComputePayloadsTx`) and returns the expected `EntityPathChangedPayload` for each affected descendant directly in the `ReplayEvent` return value. The skipped records from the export are not discarded — they are buffered and compared field-by-field against the bus-returned payloads to validate export integrity. Count mismatches or payload mismatches are surfaced as warnings on the `ImportResult` and logged via the structured logger. This validation is non-fatal: a warning indicates possible export corruption or schema divergence but does not abort the import.

No new `EntityPathChangedEvent` rows are written to the event log during import. The event-log-growth defect that existed when `ReplayEvent` used `applyEventTx` (which called `propagatePathChangesTx`) was fixed in #191.

## Warning surface

Warnings are exposed on `ImportResult` as two fields kept in lockstep:

- `WarningCount int` — number of warnings detected (was previously `Warnings int`).
- `Warnings []error` — one descriptive error per warning, in detection order. `len(Warnings) == WarningCount` as an invariant.

The CLI summary prints the count on its single summary line and one indented `warning: <message>` line per entry in `Warnings`, suppressed by `-q`/`--quiet`. Splitting the count from the diagnostic surface lets the CLI render actionable detail while programmatic consumers (tests, future automation) can still treat the count as a numeric outcome.
