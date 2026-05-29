# Conventions

## Command pattern
- Each `cmd/xxx/` exposes `NewXxxCmd(db xxxDB)` (test injection) + `NewDefaultXxxCmd()` (production).
- `cmd/xxx/db.go` defines minimal `xxxDB` interface for that command only — never pass `*database.DB` directly.
- These interfaces are real seams with hand-rolled fakes in `*_test.go`. Do not treat as boilerplate.

## Fakes vs mocks
- Hand-rolled fakes for all internal interfaces (struct with controllable return values).
- mockery-generated mocks ONLY for external/third-party interfaces.
- Never generate mocks for interfaces defined within this codebase.

## Error handling
- Wrap: `fmt.Errorf("context: %w", err)`
- All DB ops use `ExecInTransaction` + `WithRetry` helpers.

## Styles
- All styles on `Styles` struct in `internal/styles/styles.go`, accessed via public accessor methods on `appStyles` singleton.
- Never inline `lipgloss.NewStyle()` in rendering functions.
- Wong palette with `lipgloss.AdaptiveColor{Light, Dark}` for colorblind safety.

## Tests
- `testify/assert` for non-fatal, `testify/require` for preconditions.
- App-layer tests use `openTestApp(t)` (returns `*app.App`); if store access needed, use `openTestAppWithStore(t)`.
- Both helpers are defined in the `internal/app` test package.

## Misc
- Timestamps: RFC3339 UTC `Z` suffix.
- No type assertions / `any` casts — prefer typed interfaces.
- No magic numbers — use stdlib constants.
- UI: silence is success; verbose only with `-v`/`--verbose`; `--json` on all commands.
- No comments unless WHY is non-obvious.
- web package: `cmd/serve/` is shell only — no `net/http`, `html/template`, `//go:embed` there.
