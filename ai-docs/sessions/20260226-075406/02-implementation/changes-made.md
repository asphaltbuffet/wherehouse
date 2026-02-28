# Implementation Changes

## Batch 1 — Task A (golang-developer)

### internal/cli/database.go
- Added `ErrDatabaseNotInitialized` sentinel error
- Added `CheckDatabaseExists(dbPath string) error` using `os.Stat`
- Updated `OpenDatabase` to call `CheckDatabaseExists` before opening (fail-fast on missing DB)

### internal/cli/database_test.go
- Updated existing OpenDatabase success tests to pre-create DB via `database.Open`
- Added new error test case: "error when database file does not exist"
- Added `TestCheckDatabaseExists` table-driven test (3 cases)

## Batch 2 — Task B (golang-ui-developer)

### cmd/initialize/doc.go (new)
- Package doc comment

### cmd/initialize/initialize.go (new)
- `GetInitializeCmd()` — parent command, help-only (no RunE)

### cmd/initialize/database.go (new)
- `GetDatabaseCmd()` — `initialize database` subcommand with `--force` flag
- `runInitializeDatabase` — main RunE
- `handleExistingDatabase` — helper for force/backup logic
- `backupDatabase` — date-stamped backup with collision counter
- `printInitResult` — human-readable/JSON output

### cmd/initialize/database_test.go (new)
- 11 table-driven tests covering fresh install, existing DB, --force, backup collision, JSON output, error cases

### cmd/root.go (modified)
- Import `cmd/initialize`
- `rootCmd.AddCommand(initialize.GetInitializeCmd())`
