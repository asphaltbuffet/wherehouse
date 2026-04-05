# Final Implementation Plan: `wherehouse initialize database` + Pre-flight DB Check

**Session**: 20260226-075406
**Date**: 2026-02-26
**Status**: Approved for implementation

---

## Problem Statement

When a user runs any command before the database exists, the SQLite driver (modernc) emits an
opaque low-level error ("not enough memory" or similar). The fix has two independent parts:

1. **Pre-flight check** - Intercept the missing-database case in `cli.OpenDatabase` before the
   driver is invoked, and surface a clear, actionable error.
2. **`initialize database` subcommand** - Provide an explicit initialization step that creates the
   database file. Fails if the file already exists unless `--force` is passed.

---

## Clarification Changes from Initial Plan

| Item | Initial Plan | Final Plan |
|------|-------------|------------|
| Database path override | `--database` flag on `initialize` | Removed; root `--db` flag is the only path override |
| Command structure | `wherehouse initialize [--database <path>]` | `wherehouse initialize` (parent, help only); `wherehouse initialize database` (action) |
| Backup suffix | `.backup` (fixed) | `.backup.<YYYYMMDD>` (date-stamped) |
| Already-exists behavior | Silently pass if identical | Error: not idempotent. Must use `--force` |
| Command name | `init` / `initcmd` | `initialize` / `initialize` |

---

## Command Structure

```
wherehouse initialize                   # Shows help; no action. Parent command only.
wherehouse initialize database          # Creates DB. Fails if already present.
wherehouse initialize database --force  # Renames existing to <path>.backup.<YYYYMMDD>, then creates fresh.
```

The root `--db` persistent flag controls the database path for all commands including
`initialize database`. There is no initialize-local `--database` flag.

---

## Part 1: Pre-flight Database Existence Check

### File: `/home/grue/dev/wherehouse/internal/cli/database.go`

**Current state**: `OpenDatabase(ctx)` resolves path from config and calls `database.Open` directly.

**Changes**:

Add sentinel error and `CheckDatabaseExists` helper. Update `OpenDatabase` to call the check
before `database.Open`.

```go
// ErrDatabaseNotInitialized is returned when the database file does not exist on disk.
// Callers can use errors.Is to detect this case programmatically.
var ErrDatabaseNotInitialized = errors.New("database not initialized")

// CheckDatabaseExists returns ErrDatabaseNotInitialized if the file at dbPath
// does not exist. Returns nil if the file is present. Returns a wrapped os error
// for any other stat failure (permissions, etc.).
func CheckDatabaseExists(dbPath string) error {
    _, err := os.Stat(dbPath)
    if errors.Is(err, os.ErrNotExist) {
        return fmt.Errorf("database not found at %q: run `wherehouse initialize database` to create it: %w",
            dbPath, ErrDatabaseNotInitialized)
    }
    return err // nil or unexpected OS error
}
```

Update `OpenDatabase`:

```go
func OpenDatabase(ctx context.Context) (*database.Database, error) {
    cfg, ok := ctx.Value(config.ConfigKey).(*config.Config)
    if !ok || cfg == nil {
        return nil, errors.New("configuration not found in context")
    }

    dbPath, err := cfg.GetDatabasePath()
    if err != nil {
        return nil, fmt.Errorf("failed to resolve database path: %w", err)
    }

    // Pre-flight: fail fast with a human-readable message if the DB file is absent.
    if err := CheckDatabaseExists(dbPath); err != nil {
        return nil, err
    }

    dbConfig := database.Config{
        Path:        dbPath,
        BusyTimeout: database.DefaultBusyTimeout,
        AutoMigrate: true,
    }

    return database.Open(dbConfig)
}
```

**Import additions**: `"os"` (already in stdlib; verify it is added to the import block).

---

## Part 2: `initialize` Command Package

### Directory structure

```
/home/grue/dev/wherehouse/cmd/initialize/
  doc.go            - package doc comment
  initialize.go     - GetInitializeCmd() parent cobra command; no RunE
  database.go       - GetDatabaseCmd() subcommand; runInitializeDatabase() implementation
  database_test.go  - table-driven tests
```

### File: `cmd/initialize/doc.go`

```go
// Package initialize provides the `wherehouse initialize` command group for
// first-time setup of wherehouse resources.
package initialize
```

All files in `cmd/initialize/` use `package initialize`.

### File: `cmd/initialize/initialize.go`

```go
package initialize

import "github.com/spf13/cobra"

var initializeCmd *cobra.Command

// GetInitializeCmd returns the initialize command group. The parent command shows help only;
// action is delegated to subcommands (e.g., `initialize database`).
func GetInitializeCmd() *cobra.Command {
    if initializeCmd != nil {
        return initializeCmd
    }

    initializeCmd = &cobra.Command{
        Use:   "initialize",
        Short: "Initialize wherehouse resources",
        Long: `Initialize wherehouse resources for first-time setup.

Examples:
  wherehouse initialize database           Create the database
  wherehouse initialize database --force   Reinitialize (backs up existing database)`,
        // No RunE: displays help when called without a subcommand.
    }

    initializeCmd.AddCommand(GetDatabaseCmd())

    return initializeCmd
}
```

### File: `cmd/initialize/database.go`

#### Imports

```go
package initialize

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "time"

    "github.com/spf13/cobra"

    "github.com/asphaltbuffet/wherehouse/internal/config"
    "github.com/asphaltbuffet/wherehouse/internal/database"
)
```

#### Command factory

```go
var databaseCmd *cobra.Command

// GetDatabaseCmd returns the `initialize database` subcommand.
func GetDatabaseCmd() *cobra.Command {
    if databaseCmd != nil {
        return databaseCmd
    }

    databaseCmd = &cobra.Command{
        Use:   "database",
        Short: "Create the wherehouse database",
        Long: `Create the SQLite database and apply all migrations.

Fails if the database already exists. Use --force to reinitialize.
The --force flag renames the existing database to <path>.backup.<YYYYMMDD>
before creating a fresh one.

The database path is controlled by the root --db flag or the database.path
config value. Default: $XDG_DATA_HOME/wherehouse/wherehouse.db`,
        RunE: runInitializeDatabase,
    }

    databaseCmd.Flags().Bool("force", false, "reinitialize: back up existing DB then create fresh")

    return databaseCmd
}
```

#### Implementation

```go
// initResult is the structured output for JSON mode.
type initResult struct {
    Status     string `json:"status"`
    Path       string `json:"path"`
    BackupPath string `json:"backup_path,omitempty"`
}

func runInitializeDatabase(cmd *cobra.Command, _ []string) error {
    ctx := cmd.Context()

    cfg, ok := ctx.Value(config.ConfigKey).(*config.Config)
    if !ok || cfg == nil {
        return errors.New("configuration not found in context")
    }

    dbPath, err := cfg.GetDatabasePath()
    if err != nil {
        return fmt.Errorf("failed to resolve database path: %w", err)
    }

    force, _ := cmd.Flags().GetBool("force")

    var backupPath string

    // Check if file already exists.
    if _, statErr := os.Stat(dbPath); statErr == nil {
        // File exists.
        if !force {
            return fmt.Errorf("database already exists at %q: use --force to reinitialize", dbPath)
        }
        // --force: attempt timestamped backup.
        backupPath, err = backupDatabase(dbPath)
        if err != nil {
            // Backup failed: warn and continue (best-effort).
            fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not back up existing database: %v\n", err)
            backupPath = ""
            // Remove the existing file so Open can create a fresh one.
            if removeErr := os.Remove(dbPath); removeErr != nil {
                return fmt.Errorf("could not remove existing database for reinitialization: %w", removeErr)
            }
        }
    } else if !errors.Is(statErr, os.ErrNotExist) {
        // Unexpected stat error (permissions, etc.).
        return fmt.Errorf("could not check database path: %w", statErr)
    }

    // Ensure parent directory exists (XDG data dir may not exist on fresh install).
    if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
        return fmt.Errorf("could not create database directory: %w", err)
    }

    // Open (creates file) with migrations.
    dbConfig := database.Config{
        Path:        dbPath,
        BusyTimeout: database.DefaultBusyTimeout,
        AutoMigrate: true,
    }
    db, err := database.Open(dbConfig)
    if err != nil {
        return fmt.Errorf("database initialization failed: %w", err)
    }
    db.Close()

    // Output.
    return printInitResult(cmd, cfg, dbPath, backupPath)
}
```

#### Backup helper

```go
// backupDatabase renames dbPath to dbPath.backup.<YYYYMMDD>.
// If that target already exists, it appends a counter suffix (.1, .2, ...).
// Returns the backup path on success, or an error.
func backupDatabase(dbPath string) (string, error) {
    dateSuffix := time.Now().Format("20060102")
    candidate := dbPath + ".backup." + dateSuffix

    // Resolve collision with counter suffix.
    if _, err := os.Stat(candidate); err == nil {
        for i := 1; i <= 99; i++ {
            candidate = fmt.Sprintf("%s.backup.%s.%d", dbPath, dateSuffix, i)
            if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
                break
            }
        }
    }

    if err := os.Rename(dbPath, candidate); err != nil {
        return "", err
    }
    return candidate, nil
}
```

#### Output helper

```go
func printInitResult(cmd *cobra.Command, cfg *config.Config, dbPath, backupPath string) error {
    useJSON := cfg.Output.DefaultFormat == "json"

    if useJSON {
        result := initResult{
            Status:     "initialized",
            Path:       dbPath,
            BackupPath: backupPath,
        }
        enc := json.NewEncoder(cmd.OutOrStdout())
        enc.SetIndent("", "  ")
        return enc.Encode(result)
    }

    // Human-readable.
    if backupPath != "" {
        fmt.Fprintf(cmd.OutOrStdout(), "Backed up existing database to %s\n", backupPath)
    }
    fmt.Fprintf(cmd.OutOrStdout(), "Database initialized at %s\n", dbPath)
    return nil
}
```

---

## Part 3: Wire into Root

### File: `/home/grue/dev/wherehouse/cmd/root.go`

Add import:

```go
"github.com/asphaltbuffet/wherehouse/cmd/initialize"
```

Add to `GetRootCmd()` alongside existing `AddCommand` calls:

```go
rootCmd.AddCommand(initialize.GetInitializeCmd())
```

The `initialize` parent command uses `PersistentPreRunE` inherited from root (`initConfig`), so
configuration is loaded before `RunE` executes. The `initialize database` subcommand inherits this
automatically - no override needed.

**Important**: `initialize database` calls `database.Open` directly (bypassing `cli.OpenDatabase`)
because `cli.OpenDatabase` now contains the "file must exist" pre-flight that would block
legitimate database creation. This is the only command with this exception.

---

## File Change Summary

| File | Change Type | Description |
|------|-------------|-------------|
| `/home/grue/dev/wherehouse/internal/cli/database.go` | Modify | Add `ErrDatabaseNotInitialized`, `CheckDatabaseExists`; update `OpenDatabase` |
| `/home/grue/dev/wherehouse/internal/cli/database_test.go` | Modify | Add tests for `CheckDatabaseExists` (3 cases) |
| `/home/grue/dev/wherehouse/cmd/initialize/doc.go` | New | Package doc for `initialize` |
| `/home/grue/dev/wherehouse/cmd/initialize/initialize.go` | New | `GetInitializeCmd()` parent command |
| `/home/grue/dev/wherehouse/cmd/initialize/database.go` | New | `GetDatabaseCmd()`, `runInitializeDatabase()`, `backupDatabase()`, `printInitResult()` |
| `/home/grue/dev/wherehouse/cmd/initialize/database_test.go` | New | Table-driven tests for `initialize database` |
| `/home/grue/dev/wherehouse/cmd/root.go` | Modify | Import `initialize`; add `rootCmd.AddCommand(initialize.GetInitializeCmd())` |

No schema changes. No migration changes. No projection changes.

---

## Algorithm Detail: `runInitializeDatabase`

```
1. Extract *config.Config from context (error if absent)
2. Resolve dbPath via cfg.GetDatabasePath() (error if unresolvable)
3. Read --force flag
4. os.Stat(dbPath):
     a. File exists AND !force  → return error (not idempotent)
     b. File exists AND force   → backupDatabase(dbPath)
          - On backup success: record backupPath for output
          - On backup failure: warn to stderr; os.Remove(dbPath)
              - If remove also fails: return error (cannot proceed)
     c. File absent (ErrNotExist) → continue normally
     d. Other stat error → return wrapped error
5. os.MkdirAll(filepath.Dir(dbPath), 0o700)
6. database.Open(Config{Path, BusyTimeout, AutoMigrate:true})
7. db.Close()
8. printInitResult (human or JSON based on cfg.Output.DefaultFormat)
```

---

## Algorithm Detail: `backupDatabase`

```
1. Compute dateSuffix = time.Now().Format("20060102")  // e.g., "20260226"
2. candidate = dbPath + ".backup." + dateSuffix
3. If candidate exists:
     for i := 1..99:
         candidate = dbPath + ".backup." + dateSuffix + "." + strconv.Itoa(i)
         if candidate does not exist: break
4. os.Rename(dbPath, candidate)
5. Return (candidate, nil) or ("", err)
```

---

## Output Specification

**Fresh install (no existing file):**
```
Database initialized at /home/user/.local/share/wherehouse/wherehouse.db
```

**`--force` with successful backup:**
```
Backed up existing database to /home/user/.local/share/wherehouse/wherehouse.db.backup.20260226
Database initialized at /home/user/.local/share/wherehouse/wherehouse.db
```

**`--force` with failed backup (backup warn, remove succeeded):**
```
warning: could not back up existing database: <reason>
Database initialized at /home/user/.local/share/wherehouse/wherehouse.db
```

**Already exists, no `--force`:**
```
Error: database already exists at "/home/user/.local/share/wherehouse/wherehouse.db": use --force to reinitialize
```

**DB missing, other command runs:**
```
Error: database not found at "/home/user/.local/share/wherehouse/wherehouse.db": run `wherehouse initialize database` to create it
```

**JSON output (`--json`):**
```json
{
  "status": "initialized",
  "path": "/home/user/.local/share/wherehouse/wherehouse.db",
  "backup_path": "/home/user/.local/share/wherehouse/wherehouse.db.backup.20260226"
}
```
`backup_path` is omitted (omitempty) when no backup was created.

---

## Testing Plan

### `internal/cli/database_test.go` additions

Table-driven tests for `CheckDatabaseExists`:

| Case | Setup | Expected |
|------|-------|----------|
| File present | Create temp file | nil |
| File absent, dir present | Create temp dir only | error wrapping `ErrDatabaseNotInitialized` |
| Dir absent | No setup | error wrapping `ErrDatabaseNotInitialized` |

Verify `errors.Is(err, ErrDatabaseNotInitialized)` for the two error cases.

### `cmd/initialize/database_test.go` (new)

Table-driven tests for `runInitializeDatabase`:

| Case | Setup | Force | Expected outcome |
|------|-------|-------|-----------------|
| Fresh install | No dir, no file | false | DB created; output has path |
| Fresh install | Dir exists, no file | false | DB created |
| Already exists | File present | false | Error returned; file unchanged |
| Already exists | File present | true | Backup created; new DB created; backup path in output |
| Already exists, backup collision | File + `.backup.YYYYMMDD` present | true | `.backup.YYYYMMDD.1` created |
| Backup rename fails | Read-only dir | true | Warning printed; file removed; new DB created |
| Remove fails after backup failure | Immovable + unremovable file | true | Error returned |

Use `testing.T` temp dirs (`t.TempDir()`) for filesystem isolation. Inject config via context.

---

## Error Handling Strategy

- `CheckDatabaseExists` returns a wrapped `ErrDatabaseNotInitialized` sentinel; callers can
  `errors.Is` on it. The message is self-contained for fang error display.
- `runInitializeDatabase` returns errors up the cobra `RunE` stack; fang handles display formatting.
- Backup failure is non-fatal (warn to `cmd.ErrOrStderr()`); remove failure after backup
  failure IS fatal (cannot create a clean slate).
- All error strings avoid colons before dynamic content to be safe with fang output.

---

## PersistentPreRunE Interaction

Root `PersistentPreRunE` (`initConfig`) runs for all subcommands including `initialize database`.
This loads config and sets up logging before `RunE`. The `initialize database` `RunE` can safely
read `config.ConfigKey` from context.

The pre-flight DB check is NOT in `PersistentPreRunE`. It lives in `cli.OpenDatabase`, which
is only called by commands that need the database. `config`, `initialize`, and any future
help-only commands are unaffected.

---

## Alternatives Considered

**Alt: Keep `--database` flag on initialize** - Rejected per user clarification. The root `--db` flag
is the canonical path override; duplicating it would create confusion and inconsistency.

**Alt: Make `initialize` idempotent** - Rejected per user clarification. Explicit `--force` requirement
prevents accidental data loss and aligns with the project's "explicit over implicit" philosophy.

**Alt: Fixed `.backup` suffix** - Rejected per user clarification. Date-stamped suffix avoids
silent clobber of a previous backup and is more informative to users.

**Alt: Warn and continue on remove-after-backup-failure** - Rejected because proceeding without
a known clean file state risks corrupting a partially-initialized DB or creating confusing state.
Fail fast with a clear message is safer.
