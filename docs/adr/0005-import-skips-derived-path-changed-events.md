# Import skips `EntityPathChangedEvent` records and uses them for validation

`EntityPathChangedEvent` is a derived event: `propagatePathChangesTx` generates and inserts these records automatically as a side effect of handling `EntityReparentedEvent`. The export contains both the triggering reparent event and its derived path-changed events.

Replaying path-changed events via `ReplayEvent` during import would corrupt the projection: `handleEntityReparented` already regenerates them, so each descendant's path would be updated twice — once by the reparent handler and once by the explicit path-changed replay.

Instead, import skips `EntityPathChangedEvent` records during replay. The bus regenerates them correctly when each `EntityReparentedEvent` is replayed. The skipped records from the export are not discarded — they are compared against the freshly-generated records (via `store.GetEventsAfter`) to validate export integrity. Count mismatches or payload mismatches are surfaced as warnings on the `ImportResult` and logged via the structured logger. This validation is non-fatal: a warning indicates possible export corruption or schema divergence but does not abort the import.

## Warning surface

Warnings are exposed on `ImportResult` as two fields kept in lockstep:

- `WarningCount int` — number of warnings detected (was previously `Warnings int`).
- `Warnings []error` — one descriptive error per warning, in detection order. `len(Warnings) == WarningCount` as an invariant.

The CLI summary prints the count on its single summary line and one indented `warning: <message>` line per entry in `Warnings`, suppressed by `-q`/`--quiet`. Splitting the count from the diagnostic surface lets the CLI render actionable detail while programmatic consumers (tests, future automation) can still treat the count as a numeric outcome.
