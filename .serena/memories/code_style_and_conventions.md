# Code Style and Conventions

## General
- Go 1.25; no CGo
- ASCII-only files by default (no non-ASCII unless justified)
- Succinct comments only where code is not self-explanatory
- No magic numbers — use stdlib/codebase constants (e.g. `math.MaxInt64`)
- No type assertions or `interface{}`/`any` casts — prefer proper types

## Naming and Architecture Patterns

### Command Constructor Pattern
Every command package exposes two constructors:
- `NewXxxCmd(db xxxDB)` — for tests (inject interface)
- `NewDefaultXxxCmd()` — for production wiring
Registered in `cmd/root.go` via `NewDefaultXxxCmd()`.

### Per-Command DB Interface
Each `cmd/xxx/db.go` defines a minimal `xxxDB` interface covering only what
that command needs, with a `//go:generate mockery` directive. Never pass
`*database.DB` directly to a command's run function.

### Event Type Registration
New event types:
1. Add to `EventType` iota + line-comment string + `eventTypeByName` map in `internal/database/eventTypes.go`
2. Regenerate `eventtype_string.go` with `go generate`
3. Add a case to `processEventInTx` in `eventHandler.go`
Never switch on bare integers representing enums — use typed iota constants.

### Styles
- All styles live as private fields on the `Styles` struct in `styles.go`
- Access via public accessor methods on the `appStyles` singleton (e.g. `appStyles.Item()`)
- Never inline `lipgloss.NewStyle()` in rendering functions
- Colorblind-safe: Wong palette with `lipgloss.AdaptiveColor{Light, Dark}`

## Error Handling
- Return `error` types; wrap with `fmt.Errorf("...: %w", err)`
- All DB operations use transactions
- Retry logic for SQL BUSY/LOCKED errors
- No broad catches or silent defaults — propagate explicitly
- No early-return on invalid input without logging/notification

## Testing
- `testify/assert` for assertions, `testify/require` for preconditions
- No bare `t.Fatal`/`t.Error`
- Prefer mocking over complex test setup
- `t.Skip()` only for platform-specific tests
- Every error path needs at least one test
- TDD for bug fixes: write failing test first, then fix root cause

## Database / SQL
- Every `ORDER BY` that could tie MUST include `event_id ASC` tiebreaker
- Timestamps use RFC3339 (1-second resolution) — never rely on `timestamp_utc` alone for ordering
- Foreign key constraints enabled via PRAGMA
- Use `ExecInTransaction` and `WithRetry` helpers

## UI/UX
- Silence is success (no empty-state placeholders or success confirmations by default)
- Verbose output only with `-v`/`--verbose`
- Actionable error messages: failure + likely cause + concrete remediation step
- JSON output available via `--json` flag on all commands
