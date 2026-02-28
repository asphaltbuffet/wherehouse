# Code Review: `wherehouse initialize database` Feature

**Session**: 20260226-075406
**Reviewer**: code-reviewer agent
**Date**: 2026-02-26
**Linting**: PASSED (zero errors)

---

## Files Reviewed

| File | Type |
|------|------|
| `/home/grue/dev/wherehouse/internal/cli/database.go` | Modified |
| `/home/grue/dev/wherehouse/internal/cli/database_test.go` | Modified |
| `/home/grue/dev/wherehouse/cmd/initialize/doc.go` | New |
| `/home/grue/dev/wherehouse/cmd/initialize/initialize.go` | New |
| `/home/grue/dev/wherehouse/cmd/initialize/database.go` | New |
| `/home/grue/dev/wherehouse/cmd/initialize/database_test.go` | New |
| `/home/grue/dev/wherehouse/cmd/root.go` | Modified |

---

## Strengths

- **Clean separation of concerns**: The pre-flight check (`CheckDatabaseExists`) is a standalone exported function with a sentinel error, making it easy for callers to branch on `errors.Is(err, ErrDatabaseNotInitialized)`.
- **Proper use of `database.Open` directly**: The `initialize database` command correctly bypasses `cli.OpenDatabase` (which now gates on file existence), calling `database.Open` directly. This is well-documented in code comments.
- **Defensive filesystem handling**: `handleExistingDatabase` is cleanly extracted from `runInitializeDatabase`, making the flow easy to follow. The `os.MkdirAll` for the parent directory handles fresh XDG installs correctly.
- **Good test coverage**: 11 tests for the command, 3 for `CheckDatabaseExists`, plus existing `OpenDatabase` tests updated to pre-create the DB. Tests cover happy paths, error paths, collision, JSON output, and missing config.
- **Error messages are actionable**: The "database not found" message explicitly tells users to run `wherehouse initialize database`.
- **Deferred rollback pattern for backup failures**: Backup failure is non-fatal (warn-and-continue) but removal failure IS fatal. This is the correct priority ordering.
- **Package-level singleton reset in tests**: `resetForTesting()` properly clears the singleton command variables between test runs.

---

## Concerns

### CRITICAL (must fix before merge)

**1. `backupDatabase` can silently overwrite an existing backup when all 100 collision slots are exhausted**

File: `/home/grue/dev/wherehouse/cmd/initialize/database.go`, lines 140-148

```go
if _, err := os.Stat(candidate); err == nil {
    for i := 1; i <= 99; i++ {
        candidate = fmt.Sprintf("%s.backup.%s.%d", dbPath, dateSuffix, i)
        if _, statErr := os.Stat(candidate); errors.Is(statErr, os.ErrNotExist) {
            break
        }
    }
}
```

If `.backup.YYYYMMDD` and `.backup.YYYYMMDD.1` through `.backup.YYYYMMDD.99` all exist, the loop completes all 99 iterations without hitting the `break`. The `candidate` variable ends up as `.backup.YYYYMMDD.99` (which already exists), and `os.Rename` proceeds, silently overwriting that backup.

This violates the project's "explicit over implicit" philosophy. The function should return an error when all slots are exhausted rather than clobber an existing backup.

**Fix**: After the loop, check whether the final candidate already exists:
```go
// After the loop:
if _, err := os.Stat(candidate); err == nil {
    return "", fmt.Errorf("too many backups for %s on date %s", dbPath, dateSuffix)
}
```

Or track whether the loop found a free slot with a boolean flag.

---

### IMPORTANT (should fix before merge)

**2. `printInitResult` does not respect quiet mode (`-q` / `-qq`)**

File: `/home/grue/dev/wherehouse/cmd/initialize/database.go`, lines 157-180

Other commands in the codebase (e.g., `cmd/move/item.go`, `cmd/add/item.go`, `cmd/loan/item.go`) suppress human-readable output when quiet mode is active. The `printInitResult` function checks only for JSON mode but never checks `cfg.Output.Quiet` or `cfg.IsQuiet()`.

When a user passes `-q` or `-qq`, they expect no human-readable output, but this command will still print "Database initialized at ..." to stdout.

**Fix**: Before the human-readable output block, check quiet level:
```go
if cfg.IsQuiet() {
    return nil
}
```

**3. `printInitResult` uses `cfg.Output.DefaultFormat == "json"` instead of `cfg.IsJSON()`**

File: `/home/grue/dev/wherehouse/cmd/initialize/database.go`, line 158

```go
useJSON := cfg.Output.DefaultFormat == "json"
```

The `config.Config` type already has an `IsJSON()` method that encapsulates this check. Using the raw field comparison bypasses the abstraction and would break if the method's logic ever changes (e.g., case-insensitive comparison).

**Fix**: Replace with `cfg.IsJSON()`.

**4. No test for quiet mode output suppression**

File: `/home/grue/dev/wherehouse/cmd/initialize/database_test.go`

There is no test case verifying that quiet mode suppresses human-readable output. Once concern #2 is fixed, add a test:
- Set `cfg.Output.Quiet = 1`, run the command, assert stdout is empty.

---

### MINOR (optional improvements)

**5. `_ = db.Close()` discards potential close error**

File: `/home/grue/dev/wherehouse/cmd/initialize/database.go`, line 88

```go
_ = db.Close()
```

While SQLite close errors are rare and this is a short-lived initialization path, logging or returning the error would be more defensive. This is a very minor concern since the DB was just created and no writes beyond migrations occurred.

**6. Package-level singleton pattern introduces test coupling risk**

Files: `/home/grue/dev/wherehouse/cmd/initialize/initialize.go`, `/home/grue/dev/wherehouse/cmd/initialize/database.go`

Both `initializeCmd` and `databaseCmd` are package-level singletons with nil-guard initialization. The tests handle this correctly with `resetForTesting()`, but this pattern means tests cannot run in parallel (`t.Parallel()`) within this package. This is consistent with how other `cmd/` packages work in the codebase, so it is not a real issue -- just noting the tradeoff.

**7. Magic number 99 in backup collision loop**

File: `/home/grue/dev/wherehouse/cmd/initialize/database.go`, line 143

```go
for i := 1; i <= 99; i++ {
```

The limit of 99 is a magic number. Consider extracting it as a named constant (e.g., `maxBackupCollisions = 99`) for clarity. The `mnd` linter did not flag this (likely because it is in a loop bound), but a named constant improves readability.

**8. Missing test for `GetInitializeCmd` returns same instance on second call**

File: `/home/grue/dev/wherehouse/cmd/initialize/initialize.go`

The singleton pattern (`if initializeCmd != nil { return initializeCmd }`) is untested. A simple test calling `GetInitializeCmd()` twice and asserting pointer equality would cover this. Low priority since the pattern is standard across the codebase.

---

## Questions

1. **Is quiet mode intentionally omitted from `initialize database`?** The plan mentions "human-readable, JSON, quiet" output formatting, but the implementation only handles human and JSON. If quiet mode was intentionally omitted (since initialization is a one-time operation where confirming success is important), a code comment would clarify intent.

2. **Should `backupDatabase` handle WAL/SHM journal files?** SQLite in WAL mode creates `*.db-wal` and `*.db-shm` sidecar files. When backing up via `os.Rename`, the journal files for the old database are left behind. This is likely fine since `database.Open` creates fresh ones, but the orphaned WAL/SHM files could confuse users. Low priority since the user explicitly chose `--force`.

---

## Summary

**Assessment**: CHANGES NEEDED

**Priority Fixes**:
1. [CRITICAL] Fix `backupDatabase` silent overwrite when all 100 collision slots are exhausted
2. [IMPORTANT] Add quiet mode support to `printInitResult`
3. [IMPORTANT] Use `cfg.IsJSON()` instead of raw field comparison

**Issue Counts**: CRITICAL: 1 | IMPORTANT: 3 | MINOR: 4

**Estimated Risk**: Low (the critical bug requires 100 backups on the same date to trigger, but it is a correctness issue that should be fixed)

**Testability Score**: Good (well-structured helpers, injectable config via context, cobra test harness with buffer capture)
