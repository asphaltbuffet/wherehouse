# Conventions

## Command constructor pattern (every cmd/xxx package)

- `cmd/xxx/db.go` — `xxxApp` interface (minimal methods only) + `//go:generate mockery`
- `cmd/xxx/xxx.go` — `NewXxxCmd(app xxxApp)` (testable), `NewDefaultXxxCmd()` (prod wiring via `cli.OpenDatabase`), `buildXxxCmd()` (cobra.Command shell), `runXxx()` (logic)
- Registered in `cmd/root.go` via `NewDefaultXxxCmd()`
- Never pass `*database.DB` directly to run function

## cmd/history as canonical template

```go
// db.go
type historyApp interface {
    GetHistory(ctx context.Context, req app.GetHistoryRequest) ([]app.HistoryResult, error)
}

// history.go
func NewDefaultHistoryCmd() *cobra.Command { /* opens DB, calls runHistory */ }
func NewHistoryCmd(a historyApp) *cobra.Command { /* wires runE, used in tests */ }
func buildHistoryCmd() *cobra.Command { /* pure cobra.Command definition */ }
func runHistory(cmd *cobra.Command, args []string, a historyApp) error { /* logic */ }
```

## web package isolation

`cmd/serve/` is a thin shell only — no `net/http`, `html/template`, `//go:embed`. All server logic in `internal/web`.

## Styles

All styles: private fields on `Styles` struct in `internal/styles/styles.go`, access via public accessors on singleton. Never inline `lipgloss.NewStyle()` in rendering.

## Error wrapping

`fmt.Errorf("context: %w", err)`

## DB operations

All via `ExecInTransaction` + `WithRetry` helpers.

## Timestamps

RFC3339 with UTC `Z` suffix.

## No type assertions / `any` casts — prefer typed interfaces.
