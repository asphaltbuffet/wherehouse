# Action Items: Configuration Refactoring Review

**Session**: 20260225-235216
**Generated**: 2026-02-26

Only CRITICAL and IMPORTANT issues listed. No CRITICAL issues found.

---

## IMPORTANT (must fix before merge)

### 1. Fix context key mismatch in `cmd/config/get.go:58`

**Issue**: `cmd.Context().Value("config")` uses a string key, but `cmd/root.go` stores config with typed key `config.ConfigKey` (a `configKeyType` int). These never match, so `config get` always fails.

**Fix**: Change line 58 in `cmd/config/get.go`:
```go
// Before:
cfg := cmd.Context().Value("config")
// After:
cfg := cmd.Context().Value(config.ConfigKey)
```

**Files**: `/home/grue/dev/wherehouse/cmd/config/get.go`

---

### 2. Handle ExpandPath errors in `cmd/config/check.go:49-50`

**Issue**: Errors from `config.ExpandPath` are discarded with `_`. If `$HOME` is unset, paths become empty and `fileExists` silently reports "not found" instead of the real error.

**Fix**: Check and report the error, consistent with every other command in this refactoring.

**Files**: `/home/grue/dev/wherehouse/cmd/config/check.go`

---

### 3. Handle fileExists errors in `cmd/config/check.go:55,67`

**Issue**: Errors from `fileExists` are discarded with `_`. Permission-denied errors are silently treated as "file does not exist". The `fileExists` function returns `(false, err)` for non-NotExist errors specifically so callers can distinguish these cases.

**Fix**: Check the error return and report permission or I/O errors to the user.

**Files**: `/home/grue/dev/wherehouse/cmd/config/check.go`
