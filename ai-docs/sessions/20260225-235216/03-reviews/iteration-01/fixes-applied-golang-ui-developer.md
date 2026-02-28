# Fixes Applied - golang-ui-developer

**Session**: 20260225-235216
**Date**: 2026-02-26
**Status**: ALL_FIXED

---

## Fix 1 - cmd/config/get.go: Context key mismatch

**File**: `/home/grue/dev/wherehouse/cmd/config/get.go`
**Line**: 58

**Before**:
```go
cfg := cmd.Context().Value("config")
```

**After**:
```go
cfg := cmd.Context().Value(config.ConfigKey)
```

`cmd/root.go` stores the config using the typed key `config.ConfigKey` (a `configKeyType` const).
Using a plain string `"config"` never matches a typed key in Go's context, so the lookup always
returned `nil`. Now the correct exported key is used and the lookup works.

---

## Fix 2 - cmd/config/check.go: ExpandPath errors discarded

**File**: `/home/grue/dev/wherehouse/cmd/config/check.go`
**Lines**: 49-50 (original)

**Before**:
```go
expandedGlobal, _ := config.ExpandPath(globalPath)
expandedLocal, _ := config.ExpandPath(localPath)
```

**After**:
```go
expandedGlobal, err := config.ExpandPath(globalPath)
if err != nil {
    out.Error(fmt.Sprintf("failed to expand global config path %q: %v", globalPath, err))
    return fmt.Errorf("failed to expand global config path: %w", err)
}

expandedLocal, err := config.ExpandPath(localPath)
if err != nil {
    out.Error(fmt.Sprintf("failed to expand local config path %q: %v", localPath, err))
    return fmt.Errorf("failed to expand local config path: %w", err)
}
```

Errors are now checked immediately. If `$HOME` is unset or path expansion fails, the user gets
a clear message identifying which path could not be expanded and why, rather than silently
proceeding with an empty path.

---

## Fix 3 - cmd/config/check.go: fileExists errors discarded

**File**: `/home/grue/dev/wherehouse/cmd/config/check.go`
**Lines**: 55, 67 (original)

**Before**:
```go
globalExists, _ := fileExists(cmdFS, expandedGlobal)
// ...
localExists, _ := fileExists(cmdFS, expandedLocal)
```

**After**:
```go
globalExists, err := fileExists(cmdFS, expandedGlobal)
if err != nil {
    out.Error(fmt.Sprintf("cannot access global config %s: %v", expandedGlobal, err))
    return fmt.Errorf("cannot access global config: %w", err)
}
// ...
localExists, err := fileExists(cmdFS, expandedLocal)
if err != nil {
    out.Error(fmt.Sprintf("cannot access local config %s: %v", expandedLocal, err))
    return fmt.Errorf("cannot access local config: %w", err)
}
```

`fileExists` returns `(false, err)` for non-NotExist errors (e.g. permission denied, I/O error).
Previously the error was discarded, so a permission-denied file was treated as "not found" and
silently skipped. Now the error is surfaced to the user with the path that was inaccessible.

---

## Verification

```
go build ./cmd/config/...   OK
go test -count=1 ./cmd/config/...   ok  (0.004s)
```

All 3 issues fixed. Build clean, tests pass.
