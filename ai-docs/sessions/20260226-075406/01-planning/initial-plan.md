# Architecture Plan: Init Command + Database Pre-flight Check

## Problem Statement

When a user runs any command (e.g., `add item`) before the database exists, the SQLite driver
returns an opaque error like "not enough memory" rather than a helpful message. The solution has
two parts:

1. A pre-flight check that intercepts the missing-database case before any driver-level error
   occurs, and surfaces a clear, actionable message.
2. A new `init` command that creates (or resets) the database as an explicit first-time setup step.

---

## Root Cause Analysis

`cli.OpenDatabase` calls `database.Open`, which calls `sql.Open("sqlite", path)` followed by
PRAGMAs and a ping. When the directory exists but the file does not, the SQLite driver (modernc)
creates an empty file. When the directory does not exist (the common fresh-install case), the
driver fails with a low-level OS error that surfaces as "not enough memory."

The fix must happen before `database.Open` is called - a simple `os.Stat` check on the resolved
path is sufficient and avoids touching the driver at all.

---

## Component Design

### 1. Pre-flight Database Existence Check

**Location:** `internal/cli/database.go` (extend `OpenDatabase`)

Add a helper `CheckDatabaseExists(dbPath string) error` that performs an `os.Stat` check before
opening the connection. If the file is absent, return a typed sentinel error with a human-friendly
message.

```go
// ErrDatabaseNotInitialized is returned when the database file does not exist.
// Commands should catch this and surface a helpful message pointing to `wherehouse init`.
var ErrDatabaseNotInitialized = errors.New("database not initialized")
```

The `OpenDatabase` function resolves the path, calls `CheckDatabaseExists`, and only proceeds to
`database.Open` if the file is present. The error message surfaced to the user is:

```
Database not found at <path>.
Run `wherehouse init` to create it.
```

**Design decision:** The check goes in `internal/cli/database.go` rather than in each command's
`RunE`, because `cli.OpenDatabase` is the single call-site shared by all commands. This guarantees
the check fires consistently without touching every command package.

**Design decision:** Use a sentinel error (`ErrDatabaseNotInitialized`) rather than a formatted
string so callers can `errors.Is` on it if needed in tests or future code.

### 2. `cmd/init` Package - New Init Command

**Location:** `cmd/init/` (new directory, following existing command package layout)

```
cmd/init/
  doc.go      - package doc comment
  init.go     - GetInitCmd() cobra.Command factory
  helpers.go  - openDatabase (delegating to cli.OpenDatabase or direct database.Open)
```

**Command signature:**

```
wherehouse init [--database <path>] [--force]
```

**Flags:**

| Flag         | Type   | Description                                                       |
|--------------|--------|-------------------------------------------------------------------|
| `--database` | string | Path where the database should be created. Overrides config/env. |
| `--force`    | bool   | Overwrite existing database (backup first).                       |

Note: `--database` on `init` has the same semantics as the root `--db` flag, but is local to the
`init` command to make it self-contained for first-time users who may not know global flags.
Both `--db` (root) and `--database` (init-local) resolve the same path. The implementation
should prefer the init-local `--database` if set, fall back to root `--db`, fall back to config.

**Algorithm:**

```
1. Resolve effective database path (init --database > root --db > config > default)
2. Check if file exists at resolved path
3. If exists AND --force not set:
     Return error: "Database already exists at <path>. Use --force to overwrite."
4. If exists AND --force set:
     Attempt os.Rename(<path>, <path>+".backup")
     If rename fails: print warning to stderr (do NOT fail)
     Attempt os.Remove(<path>) if rename failed (to allow overwrite)
5. Ensure parent directory exists (os.MkdirAll)
6. Open database (database.Open with AutoMigrate=true)
   This runs all migrations and seeds system locations.
7. Print success: "Database initialized at <path>"
```

**Key decisions:**

- `init` calls `database.Open` directly rather than `cli.OpenDatabase`, because `cli.OpenDatabase`
  will soon contain the "file must exist" pre-flight that would block `init` from creating the DB.
  The `init` command is the one case where the DB legitimately does not exist yet.
- `init` does NOT check `CheckDatabaseExists` - it is the exception to the rule.
- Backup is best-effort: warn and continue on failure (as specified in requirements).
- `os.MkdirAll` ensures the XDG data directory exists (fresh install scenario).
- The command uses `PersistentPreRunE` from root (i.e., `initConfig`) so configuration is loaded
  before `RunE` executes - no special parent-chain override needed.

**Output (human-readable):**

```
Database initialized at /home/user/.local/share/wherehouse/wherehouse.db
```

With `--force` and successful backup:
```
Backed up existing database to /home/user/.local/share/wherehouse/wherehouse.db.backup
Database initialized at /home/user/.local/share/wherehouse/wherehouse.db
```

With `--force` and failed backup (warn only):
```
Warning: could not back up existing database: <reason>
Database initialized at /home/user/.local/share/wherehouse/wherehouse.db
```

**JSON output** (when `--json` flag is set):
```json
{
  "status": "initialized",
  "path": "/home/user/.local/share/wherehouse/wherehouse.db",
  "backup_path": "/home/user/.local/share/wherehouse/wherehouse.db.backup"
}
```

### 3. Wire `init` into root command

**Location:** `cmd/root.go`

Add import and `rootCmd.AddCommand(initcmd.GetInitCmd())` alongside existing commands.

---

## File Change Summary

| File | Change |
|------|--------|
| `internal/cli/database.go` | Add `ErrDatabaseNotInitialized`, `CheckDatabaseExists`, update `OpenDatabase` |
| `cmd/init/doc.go` | New - package documentation |
| `cmd/init/init.go` | New - `GetInitCmd()`, `runInit()` |
| `cmd/root.go` | Add `initcmd` import and `AddCommand(initcmd.GetInitCmd())` |

No database schema changes. No migration changes. No projection changes.

---

## Error Handling Strategy

The pre-flight check in `OpenDatabase` returns a wrapped `ErrDatabaseNotInitialized`. The cobra
`RunE` functions return this error up the stack. `fang.Execute` in `cmd/root.go` handles error
display. The error message should be self-contained and not rely on cobra's error prefix.

The error returned from `OpenDatabase` when the file is missing should be:

```go
fmt.Errorf("database not found at %q: run `wherehouse init` to create it: %w",
    dbPath, ErrDatabaseNotInitialized)
```

This satisfies both human-readable output and `errors.Is` testability.

---

## PersistentPreRunE Interaction

The root `PersistentPreRunE` (`initConfig`) runs before every subcommand's `RunE`. It loads
config and sets up logging. The `init` command inherits this without issue. The pre-flight DB
check is NOT part of `PersistentPreRunE` - it lives in `cli.OpenDatabase`, which is only called
by commands that actually need the database. This means config commands (e.g., `config set`) and
the new `init` command are not affected by the pre-flight check.

---

## Testing Approach

- `internal/cli/database_test.go`: Add tests for `CheckDatabaseExists` with temp dirs (file
  present, file absent, directory absent).
- `cmd/init/init_test.go`: Table-driven tests covering fresh install, already-exists without
  force (error), already-exists with force (backup success), already-exists with force (backup
  failure - warn and continue).
- Existing tests for `add`, `move`, etc. remain unchanged; the pre-flight check is transparent
  when the DB file exists.

---

## Alternatives Considered

**Alternative A: PersistentPreRunE database check in root**
Check DB existence in `initConfig` (root's `PersistentPreRunE`). Rejected because it would fire
for commands that do not need the database (e.g., `config`, `init` itself, future help-only
commands), requiring an allowlist of exempt commands.

**Alternative B: Per-command PreRunE hooks**
Add `PreRunE` to each command that needs the DB. Rejected as duplicative - every new command
would need to remember to add it. Centralizing in `cli.OpenDatabase` is safer and DRY.

**Alternative C: Let SQLite create the file**
When the file does not exist, `database.Open` with modernc sqlite creates an empty file
automatically (in the happy path where the directory exists). We could run migrations on it.
Rejected because this silently creates the DB without user intent, which contradicts the project's
philosophy of explicit user action. The `init` command should be the explicit initialization step.
