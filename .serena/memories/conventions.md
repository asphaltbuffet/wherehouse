# Code Conventions

- Go 1.25, no CGo
- `testify/assert` non-fatal, `testify/require` for preconditions
- Errors: `fmt.Errorf("context: %w", err)`
- All DB ops: `ExecInTransaction` + `WithRetry` helpers
- Timestamps: RFC3339 UTC `Z` suffix
- No type assertions / `any` casts — typed interfaces
- UI: silence is success; verbose with `-v`; `--json` on all commands

## Command constructor pattern
Every cmd package: `NewXxxCmd(db xxxDB)` (tests) + `NewDefaultXxxCmd()` (prod).
Per-command `cmd/xxx/db.go` defines minimal interface — hand-rolled fakes in tests, NOT mockery.
Reserve mockery for external/third-party interfaces only.

## Styles
All styles as private fields on `Styles` struct in `internal/styles/styles.go`.
Use Wong palette with `lipgloss.AdaptiveColor{Light, Dark}`. Never inline `lipgloss.NewStyle()`.

## Adding a new EventType
1. Add to `EventType` iota + line-comment string + `eventTypeByName` map in `internal/database/eventTypes.go`
2. `go generate ./...` to regenerate `eventtype_string.go`
3. Add case to `processEventInTx` in `eventHandler.go`

## applyEventTx vs applyEventProjectionOnlyTx
`applyEventTx` — normal dispatch, calls `propagatePathChangesTx` on reparent (writes new events).
`applyEventProjectionOnlyTx` — rebuild/replay dispatch, never writes events. Use for `TruncateAndReplay`. See ADR-0009.
