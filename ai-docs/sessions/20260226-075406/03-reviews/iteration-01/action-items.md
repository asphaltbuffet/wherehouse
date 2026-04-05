# Action Items: `initialize database` Review (Iteration 01)

Critical and important fixes only. Items ordered by priority.

---

## 1. [CRITICAL] Fix `backupDatabase` silent overwrite on slot exhaustion

**File**: `/home/grue/dev/wherehouse/cmd/initialize/database.go`, lines 140-148
**Problem**: When all 100 backup name slots are taken, loop ends on `.backup.YYYYMMDD.99` which already exists and `os.Rename` overwrites it.
**Fix**: After the collision loop, check if the final `candidate` already exists. If so, return an error: `fmt.Errorf("too many backups for %s on date %s", dbPath, dateSuffix)`. Alternatively, track with a `found` boolean and error when `!found`.

## 2. [IMPORTANT] Add quiet mode support to `printInitResult`

**File**: `/home/grue/dev/wherehouse/cmd/initialize/database.go`, lines 157-180
**Problem**: Human-readable output prints regardless of `-q`/`-qq` flags.
**Fix**: Add early return before human-readable block:
```go
if cfg.IsQuiet() {
    return nil
}
```

## 3. [IMPORTANT] Use `cfg.IsJSON()` instead of raw field comparison

**File**: `/home/grue/dev/wherehouse/cmd/initialize/database.go`, line 158
**Problem**: `cfg.Output.DefaultFormat == "json"` bypasses the `IsJSON()` abstraction.
**Fix**: Replace with `cfg.IsJSON()`.

## 4. [IMPORTANT] Add test for quiet mode output suppression

**File**: `/home/grue/dev/wherehouse/cmd/initialize/database_test.go`
**Problem**: No test verifies quiet mode suppresses output.
**Fix**: Add test case: set `cfg.Output.Quiet = 1`, execute command, assert stdout buffer is empty.
