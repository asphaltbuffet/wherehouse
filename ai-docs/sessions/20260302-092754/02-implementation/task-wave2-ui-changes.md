# Wave 2 UI Changes — Implementation Notes

**Date:** 2026-03-02
**Agent:** golang-ui-developer
**Status:** SUCCESS

## Subtask A — cmd/migrate/ package

### Files Created

**`/home/grue/dev/wherehouse/cmd/migrate/doc.go`**
Package declaration for the migrate package.

**`/home/grue/dev/wherehouse/cmd/migrate/migrate.go`**
- `GetMigrateCmd()` returns a `*cobra.Command` with `Use="migrate"`
- Registers the `database` subcommand via `GetDatabaseCmd()`
- Has non-empty Short and Long help text

**`/home/grue/dev/wherehouse/cmd/migrate/database.go`**
- `GetDatabaseCmd()` returns a `*cobra.Command` with `Use="database"`
- Short text: "migrate database IDs from UUID to nanoid format"
- Long text contains "UUID" (satisfies TestGetDatabaseCmd_LongHelp)
- `--dry-run` boolean flag, default false
- RunE calls `cli.OpenDatabase` then `cli.MigrateDatabase(cmd, db, dryRun)`
- Follows thin-wrapper pattern from cmd/initialize/database.go

**`/home/grue/dev/wherehouse/internal/cli/migrate.go`** (stub)
- Provides `MigrateDatabase(*cobra.Command, *database.Database, bool) error`
- Stub returns nil — full implementation is a golang-developer task
- Required for compilation of cmd/migrate/database.go

### cmd/root.go Registration
Added import `"github.com/asphaltbuffet/wherehouse/cmd/migrate"` and
`rootCmd.AddCommand(migrate.GetMigrateCmd())` after existing AddCommand calls.

## Subtask B — cmd/add/ nanoid migration

### cmd/add/item.go
- Removed import `"github.com/google/uuid"`
- Added import `"github.com/asphaltbuffet/wherehouse/internal/nanoid"`
- Replaced `uuid.NewV7()` block with `nanoid.New()`

### cmd/add/location.go
- Removed import `"github.com/google/uuid"`
- Added import `"github.com/asphaltbuffet/wherehouse/internal/nanoid"`
- Replaced `uuid.NewV7()` block with `nanoid.New()`

## Test Results

```
go test ./cmd/migrate/...   -> ok  (0.003s)
go test ./cmd/add/...       -> ok  (0.003s)
go build ./cmd/...          -> success (no output)
```

All 7 tests in cmd/migrate pass:
- TestGetDatabaseCmd_RegisteredUnderMigrateCmd
- TestGetDatabaseCmd_HasDryRunFlag
- TestGetDatabaseCmd_DryRunDefaultFalse
- TestGetDatabaseCmd_ShortHelp
- TestGetDatabaseCmd_LongHelp
- TestGetMigrateCmd_HasExpectedFields
