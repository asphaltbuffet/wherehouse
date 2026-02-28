# Task A Notes and Decisions

## Key Decisions

### Existing tests required updating

All existing `OpenDatabase` success tests passed a path to a non-existent file (relying on `database.Open` to create it). After adding `CheckDatabaseExists` to `OpenDatabase`, those tests would fail with `ErrDatabaseNotInitialized`. Updated each to pre-create the database via `database.Open` directly (which is how the `initialize database` command will work in production).

This is semantically correct: `OpenDatabase` is now "open an existing database", not "create or open". The `initialize database` subcommand is the only command that creates the file.

### govet shadow fix

The linter flagged `if err := CheckDatabaseExists(dbPath); err != nil` as shadowing the outer `err` from `cfg.GetDatabasePath()`. Fixed by using `err =` to reuse the existing variable. This is the standard pattern noted in CLAUDE.md linting gotchas.

### `TestOpenDatabase_AutoMigration` repurposed

This test originally verified that `OpenDatabase` creates a new database file. Since `OpenDatabase` no longer creates files, the test was repurposed to verify that it succeeds on a pre-existing file and that the file remains after the call. The auto-migration behavior (applied to pre-existing files) is still exercised.

### `wantSentinel` field comment removed

`goimports` flagged the struct field alignment when a trailing comment was present on `wantSentinel`. Removed the comment to satisfy the formatter. The field names are self-documenting.

### `require.ErrorIs` for sentinel check

`testifylint` prefers `require.ErrorIs` (stops test on failure) over `assert.True(t, errors.Is(...))` or `assert.ErrorIs`. Used `require.ErrorIs` to match the `require-error` rule.

## No Deviations from Plan

The implementation follows the plan exactly:
- `ErrDatabaseNotInitialized = errors.New("database not initialized")`
- `CheckDatabaseExists` uses `os.Stat`, wraps `ErrDatabaseNotInitialized` on `os.ErrNotExist`
- `OpenDatabase` calls `CheckDatabaseExists` before `database.Open`
- Three test cases for `CheckDatabaseExists` as specified
