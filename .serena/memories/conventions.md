# Conventions

## Command pattern
- `NewXxxCmd(db xxxDB)` for tests (inject interface); `NewDefaultXxxCmd()` for prod wiring
- Each `cmd/xxx/db.go` defines a minimal `xxxDB` interface — these are real seams, not boilerplate
- Hand-rolled fakes in `*_test.go`; mockery only for external deps

## Store layer
- All SQL in `internal/store`; never in cmd or app layers
- Transactional methods take `(ctx, tx Tx)` — `Tx` is the store's abstraction over `*sql.Tx`
- `ExecInTransaction` wraps begin/commit/rollback; `WithRetry` for SQLite busy errors
- Projection tables (e.g. `entities_current`) are fully rebuildable from `events`

## Event sourcing
- No undo — corrections create compensating events
- `events` schema: `event_id PK, event_type, timestamp_utc, actor_user_id, payload JSON, note`
- Adding a new event type: update `EventType` iota + `eventTypeByName` map → `go generate ./...` → add case to `processEventInTx`

## Web layer (`internal/web`)
- `cmd/serve/` is thin shell only — no net/http, html/template, or //go:embed there
- Handler split by cluster: `handlers_browse.go`, `handlers_entity.go`, `handlers_add.go`, `handlers_util.go`
- `EntityResult.CanonicalName` = leaf name only; `FullPathDisplay` = full colon-separated path

## Style
- Errors: `fmt.Errorf("context: %w", err)`
- Timestamps: RFC3339 UTC `Z` suffix
- No type assertions / `any` casts; no magic numbers
- UI: silence is success; verbose only with `-v`/`--verbose`; `--json` on all commands
- Styles: lipgloss via `internal/styles` singleton — never inline `lipgloss.NewStyle()`
- Wong palette with `lipgloss.AdaptiveColor{Light, Dark}` for colorblind safety
