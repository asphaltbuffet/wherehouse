# Code Review - Iteration 02 (Re-review)

**Session**: 20260225-235216
**Date**: 2026-02-26
**Reviewer**: code-reviewer agent
**Scope**: Verify 3 action items from iteration-01 were fixed; final check of all changed files

---

## Verification of Iteration-01 Action Items

### 1. FIXED - Context key mismatch in `cmd/config/get.go:58`

**Previous issue**: `cmd.Context().Value("config")` used a string key, but `cmd/root.go` stores config with typed key `config.ConfigKey`.

**Current code** (line 58):
```go
cfg := cmd.Context().Value(config.ConfigKey)
```

**Verification**: `cmd/root.go:118` sets the context with the same typed key:
```go
ctx := context.WithValue(cmd.Context(), config.ConfigKey, globalConfig)
```

Both sides now use `config.ConfigKey` (type `configKeyType`, defined in `internal/config/config.go:16`). **Correctly fixed.**

### 2. FIXED - ExpandPath errors discarded in `cmd/config/check.go`

**Previous issue**: Lines 49-50 used `_, _ :=` discarding errors from `config.ExpandPath`.

**Current code** (lines 49-58):
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

Errors are now checked and reported with user-visible messages and wrapped returns. **Correctly fixed.**

### 3. FIXED - fileExists errors discarded in `cmd/config/check.go`

**Previous issue**: Lines 55,67 used `_, _ :=` discarding errors from `fileExists`.

**Current code** (lines 64-68, 80-84):
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

Permission-denied and I/O errors are now surfaced to the user. **Correctly fixed.**

---

## Automated Linting Results

`golangci-lint` reports 3 issues:

### Issue L1: Variable shadowing in `cmd/config/check.go` (govet)

**Lines 70 and 86**: `err` declared in short-variable `:=` inside `if` blocks shadows the outer `err` from line 49/55.

```go
// Line 70 - shadows outer err
if err := config.Check(cmdFS, expandedGlobal); err != nil {
// Line 86 - shadows outer err
if err := config.Check(cmdFS, expandedLocal); err != nil {
```

This is a lint error, not a runtime bug in this case (the outer `err` is not used after these blocks). However, lint must pass. The fix is to use a different variable name or assign without `:=`.

**Priority**: HIGH (linter must pass for CI)

### Issue L2: Magic number in `cmd/config/set.go:36` (mnd)

```go
Args: cobra.ExactArgs(2),
```

The linter flags `2` as a magic number. This is a common cobra pattern but the linter configuration requires it to be a named constant.

**Priority**: HIGH (linter must pass for CI)

---

## Final Review of All Changed Files

### Strengths

- All three action items from iteration-01 are correctly and thoroughly fixed
- Consistent error handling pattern across all config subcommands: user-visible message via `out.Error()` plus wrapped `fmt.Errorf` return
- Clean separation of concerns: `internal/config/writer.go` owns file I/O and validation, `cmd/config/*.go` handles CLI presentation
- `WriteDefault` properly checks existence, creates directories, and writes atomically via viper
- `Set` validates the full merged config before writing, preventing invalid states
- `Check` reads raw TOML (not via viper) to test the actual parse path users will hit
- `parseConfigValue` is a single source of truth for type coercion with clear error messages
- `cmd/root.go:bindFlagsToConfig` only applies flags that were explicitly changed (`.Changed()` check), avoiding zero-value clobbering
- `loadConfigOrDefaults` correctly distinguishes explicit path failures (hard error) from default path failures (fall back to defaults)
- `fileExists` helper properly distinguishes "not found" from "permission denied" errors
- `cmd/config/edit.go` validates config after editing and warns without failing (correct UX choice)

### Concerns

**HIGH** (linter failures - must fix for CI):

1. **`cmd/config/check.go:70,86`** - Variable `err` shadowed by `:=` inside `if` blocks. The `govet` shadow checker flags this. While not a runtime bug here, CI will fail.

2. **`cmd/config/set.go:36`** - Magic number `2` in `cobra.ExactArgs(2)`. The `mnd` linter flags this. Define a constant or add a nolint directive if this is an accepted pattern in the codebase.

**MEDIUM** (non-blocking observations):

3. **`cmd/config/init_test.go:35,49,73,153`** - Tests discard errors from `config.ExpandPath` with `_`. While acceptable in tests, these could mask test environment issues (e.g., `$HOME` not set in CI). Consider using `require.NoError` for robustness.

4. **`cmd/config/edit.go:104`** - Uses `exec.CommandContext` with unsanitized `$EDITOR` value. This is standard Unix convention and not a real vulnerability (the user controls their own `$EDITOR`), but worth noting for completeness.

### Questions

None. The implementation is clear and well-structured.

---

## Summary

**Assessment**: APPROVED (pending lint fixes)

**All 3 iteration-01 action items**: Correctly fixed

**New issues found**: 2 lint errors (variable shadowing, magic number) that must be resolved for CI to pass. No logic, security, or event-sourcing concerns.

**Priority Fixes**:
1. Fix `err` shadowing in `cmd/config/check.go` lines 70 and 86
2. Address magic number `2` in `cmd/config/set.go` line 36

**Estimated Risk**: Low
**Testability Score**: Good
