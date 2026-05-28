# Conventions

## Command pattern
- Each `cmd/xxx/` exposes `NewXxxCmd(db xxxDB)` (test injection) + `NewDefaultXxxCmd()` (prod wiring via `cli.OpenDatabase`)
- Per-command interface in `cmd/xxx/db.go`; `//go:generate mockery` directive there
- Never pass `*store.Store` or `*app.App` directly — always via local interface
- `cmd/serve/` is shell only — no net/http, html/template, //go:embed there; all lives in `internal/web`

## Error handling
- Wrap: `fmt.Errorf("context: %w", err)`
- DB ops via `ExecInTransaction` + `WithRetry`
- No type assertions / `any` casts — typed interfaces only

## Testing
- `testify/assert` for non-fatal, `testify/require` for preconditions
- No mocking the DB in integration tests (burned by mock/prod divergence before)

## Ordering invariant
- Every `ORDER BY` that could tie MUST include `event_id ASC` as tiebreaker

## UI conventions
- Silence is success; verbose output with `-v`/`--verbose` only
- `--json` on all commands
- All styles via `internal/styles` singleton; no inline `lipgloss.NewStyle()` in rendering

## Timestamps
- RFC3339 UTC with `Z` suffix
- DB path must be absolute

## Adding a new EventType
1. Add to iota + line-comment string + `eventTypeByName` map in `internal/store/eventTypes.go` (note: CLAUDE.md says `internal/database` but actual path is `internal/store`)
2. `go generate ./...` to regen `eventtype_string.go`
3. Add case to `applyEventTx` in `internal/eventbus/bus.go`
4. Add handler method on `Bus` in `internal/eventbus/handlers.go`
