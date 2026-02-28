# Task B: Files Created/Modified

## New Files

### cmd/initialize/doc.go
Package doc comment for the `initialize` package.

### cmd/initialize/initialize.go
`GetInitializeCmd()` — parent cobra command with no RunE (shows help only). Adds `GetDatabaseCmd()` as subcommand.

### cmd/initialize/database.go
`GetDatabaseCmd()` — `initialize database` subcommand with `--force` flag.

Implementation functions:
- `runInitializeDatabase` — main RunE: resolves config/path, delegates to `handleExistingDatabase`, creates directory, calls `database.Open`, prints result
- `handleExistingDatabase` — extracted helper (reduces nestif complexity): checks for existing file, returns error if not forced, attempts backup rename, warns+removes on backup failure
- `backupDatabase` — renames to `<path>.backup.<YYYYMMDD>` with counter suffix on collision
- `printInitResult` — human-readable or JSON output

### cmd/initialize/database_test.go
11 table-driven tests covering:
- Fresh install (no parent dir, dir exists but no file)
- Already exists without --force (error, file untouched)
- --force with successful backup (file renamed, new DB created)
- --force with backup collision (counter suffix `.1` used)
- JSON output (fresh and with backup)
- No config in context (error)
- `backupDatabase` unit tests (no collision, collision, source missing)

## Modified Files

### cmd/root.go
- Added import: `"github.com/asphaltbuffet/wherehouse/cmd/initialize"`
- Added: `rootCmd.AddCommand(initialize.GetInitializeCmd())`
