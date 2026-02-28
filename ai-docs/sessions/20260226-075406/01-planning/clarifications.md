# User Clarifications

## Q1: Flag design for `--database` on init
**Answer**: Instead of a flag, make it a subcommand. To create a new database, the user types:
```
wherehouse init database
```
So `init` has a subcommand called `database`. The `--force` flag applies to this subcommand.
- `wherehouse init database` — creates database (fails if already present)
- `wherehouse init database --force` — renames existing to `.backup.<timestamp>` then creates fresh

## Q2: Backup collision behavior
**Answer**: Use timestamp suffix. When `.backup` already exists, create `.backup.20260226` style name (date-stamped).

## Q3: Idempotency
**Answer**: Error if already initialized. User must explicitly use `--force` to reinitialize. Not idempotent.

## Additional Structural Change
The command structure is now:
```
wherehouse initialize           # shows help (no action by itself)
wherehouse initialize database  # creates the database
wherehouse initialize database --force  # force-reinitializes with backup
```

The root `--db` persistent flag still controls the database path for all commands including `initialize database`.

## Q4: Command and Package Naming
**Answer**: The command is named `initialize` (not `init`). The Go package is also `initialize` (not `initcmd` or `init`).
- CLI command: `wherehouse initialize`
- Subcommand: `wherehouse initialize database`
- Go package: `initialize` under `cmd/initialize/`
- Files: `cmd/initialize/doc.go`, `cmd/initialize/initialize.go`, `cmd/initialize/database.go`, `cmd/initialize/database_test.go`
