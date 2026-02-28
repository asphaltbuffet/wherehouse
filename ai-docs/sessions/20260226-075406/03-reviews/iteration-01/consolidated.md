# Consolidated Code Review: `wherehouse initialize database`

**Session**: 20260226-075406 | **Date**: 2026-02-26 | **Linting**: PASSED

---

## CRITICAL (1 issue)

### 1. `backupDatabase` silently overwrites existing backup when all collision slots exhausted

**File**: `/home/grue/dev/wherehouse/cmd/initialize/database.go`, lines 140-148

When `.backup.YYYYMMDD` and `.backup.YYYYMMDD.1` through `.backup.YYYYMMDD.99` all exist, the loop finishes without finding a free slot. `candidate` ends up as `.backup.YYYYMMDD.99` (already exists), and `os.Rename` overwrites it silently. Violates "explicit over implicit" design philosophy.

**Fix**: After the loop, verify the final candidate does not exist. Return an error if all slots are taken.

---

## IMPORTANT (3 issues)

### 2. `printInitResult` does not respect quiet mode (`-q` / `-qq`)

**File**: `/home/grue/dev/wherehouse/cmd/initialize/database.go`, lines 157-180

Other commands suppress human-readable output in quiet mode. This command only checks for JSON mode, never `cfg.IsQuiet()`. Users passing `-q` still get "Database initialized at ..." on stdout.

**Fix**: Add `cfg.IsQuiet()` check before the human-readable output block.

### 3. `printInitResult` uses raw field comparison instead of `cfg.IsJSON()`

**File**: `/home/grue/dev/wherehouse/cmd/initialize/database.go`, line 158

```go
useJSON := cfg.Output.DefaultFormat == "json"  // bypasses abstraction
```

**Fix**: Replace with `cfg.IsJSON()`.

### 4. No test for quiet mode output suppression

**File**: `/home/grue/dev/wherehouse/cmd/initialize/database_test.go`

Once issue #2 is fixed, a test is needed: set `cfg.Output.Quiet = 1`, run the command, assert stdout is empty.

---

## MINOR (4 issues)

### 5. `_ = db.Close()` discards potential close error

**File**: `/home/grue/dev/wherehouse/cmd/initialize/database.go`, line 88

SQLite close errors are rare on a freshly created DB, but logging would be more defensive.

### 6. Package-level singleton pattern prevents parallel tests

**Files**: `/home/grue/dev/wherehouse/cmd/initialize/initialize.go`, `database.go`

Consistent with other `cmd/` packages. Tests use `resetForTesting()` correctly. Just a tradeoff note.

### 7. Magic number 99 in backup collision loop

**File**: `/home/grue/dev/wherehouse/cmd/initialize/database.go`, line 143

Consider extracting as `maxBackupCollisions = 99` for readability.

### 8. Missing singleton identity test for `GetInitializeCmd`

**File**: `/home/grue/dev/wherehouse/cmd/initialize/initialize.go`

Low priority; pattern is standard across the codebase.

---

## Open Questions

1. Is quiet mode intentionally omitted from `initialize database` (one-time operation where confirming success matters)?
2. Should `backupDatabase` handle orphaned WAL/SHM journal files after rename?

---

## Strengths

- Clean separation of concerns (sentinel error for pre-flight check)
- Proper bypass of `cli.OpenDatabase` for initialization path
- Defensive filesystem handling with extracted `handleExistingDatabase`
- Good test coverage (11 command tests, 3 pre-flight tests)
- Actionable error messages guiding users to `wherehouse initialize database`
- Correct priority ordering for backup/removal failure handling

---

## Assessment

| Metric | Value |
|--------|-------|
| Verdict | CHANGES NEEDED |
| Critical | 1 |
| Important | 3 |
| Minor | 4 |
| Risk | Low |
| Testability | Good |
