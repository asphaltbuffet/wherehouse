# Task B: Decisions and Deviations

## Decisions

### nestif Lint Issue
The original plan's `runInitializeDatabase` embedded all existing-file handling inline inside a
nested `if` block (complexity 5, flagged by `nestif`). Fixed by extracting
`handleExistingDatabase(cmd, dbPath, force)` which flattens nesting using early returns:
1. Not exists → return ("", nil)
2. Stat error (not ErrNotExist) → return error
3. Exists, no force → return error
4. Exists, force, backup ok → return (backupPath, nil)
5. Exists, force, backup fails → warn, remove, return ("", nil)

This matches the spec behavior exactly while satisfying the linter.

### testifylint: require for file-existence assertions
Two `assert.NoError(t, statErr, ...)` calls checking `os.Stat` results were upgraded to
`require.NoError` per `testifylint` rules. The assertions gate subsequent file-read operations
so `require` is semantically correct anyway.

### Variable Shadowing (err reuse)
In `runInitializeDatabase`, `dbPath, err := cfg.GetDatabasePath()` followed by
`backupPath, err := handleExistingDatabase(...)` would shadow. Used `:=` for `backupPath`
since `err` was already declared; Go allows redeclaration in short var decls when at least one
variable is new. No `govet shadow` issue arises here.

## Deviations from Plan

None. Implementation follows the final plan exactly:
- Parent command: `GetInitializeCmd()` in `initialize.go`, no RunE
- Subcommand: `GetDatabaseCmd()` in `database.go`, --force flag only
- Error messages match spec verbatim
- Backup naming: `<path>.backup.<YYYYMMDD>` with `.1`, `.2`... on collision
- Output: human-readable default, JSON when `cfg.Output.DefaultFormat == "json"`
- `database.Open` called directly (not `cli.OpenDatabase`) to bypass the existence pre-flight
