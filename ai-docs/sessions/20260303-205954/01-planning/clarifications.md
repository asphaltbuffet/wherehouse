# User Clarifications

## Quiet mode (-q) for find/scry
**Decision**: Results always print. Use `out.Println` so `-q` only suppresses info/warnings. Tabular results always appear regardless of quiet flag.

## History timestamps
**Decision**: Always use go-humanize (`cli.FormatRelativeTime`). Replace the entire 7-day threshold inline logic. Simpler code; humanize handles transitions naturally (e.g. "3 months ago").

## found/loan/lost domain extraction
**Decision**: Extract to `internal/cli/` NOW (like `cli.AddLocations`). More work but makes logic reusable from TUI/API layers immediately. Do not defer.

## history Writer() accessor
**Decision**: Add `Writer() io.Writer` accessor to OutputWriter. Small surface addition to `internal/cli/output.go`. Lets history's lipgloss-styled `formatEvent` funnel through the same writer OutputWriter controls.
