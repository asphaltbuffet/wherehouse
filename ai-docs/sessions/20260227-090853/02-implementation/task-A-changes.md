# Task A - Files Created/Modified

## New Files

### `/home/grue/dev/wherehouse/cmd/found/doc.go`
Package documentation for the `found` command.

### `/home/grue/dev/wherehouse/cmd/found/found.go`
Cobra command definition:
- `GetFoundCmd()` initializer pattern (singleton)
- `--in` / `-i` (required): found location
- `--return` / `-r` (bool): return item to home after recording found
- `--note` / `-n` (string): optional event note
- `cobra.MinimumNArgs(1)` for variadic item selectors

### `/home/grue/dev/wherehouse/cmd/found/helpers.go`
Thin wrappers over `internal/cli` package:
- `openDatabase()` -> `cli.OpenDatabase()`
- `resolveLocation()` -> `cli.ResolveLocation()`
- `resolveItemSelector()` -> `cli.ResolveItemSelector()` with command name "wherehouse found"

### `/home/grue/dev/wherehouse/cmd/found/item.go`
Core command implementation:
- `Result` struct with JSON tags (item_id, display_name, found_at, home_location, returned, found_event_id, return_event_id, warnings)
- `runFoundItem()`: cobra RunE handler
- `foundItem()`: single-item logic (warnings, home resolution, event firing)
- `validateNotSystemLocation()`: guard for --in system location
- `formatSuccessMessage()`: human-readable output helper

## Modified Files

### `/home/grue/dev/wherehouse/cmd/root.go`
Added import and registration:
- Import: `"github.com/asphaltbuffet/wherehouse/cmd/found"`
- Registration: `rootCmd.AddCommand(found.GetFoundCmd())` (between find and history)
