# Code Review: Configuration Refactoring

**Session**: 20260225-235216
**Reviewer**: code-reviewer agent
**Date**: 2026-02-26
**Assessment**: CHANGES_NEEDED

---

## Pre-Review Linting

Linting via `mise run lint` reports **1 issue**:

```
cmd/config/set.go:36:25: Magic number: 2, in <argument> detected (mnd)
    Args: cobra.ExactArgs(2),
```

This is a pre-existing lint violation (not introduced by this refactoring) but must be addressed.

---

## Strengths

1. **Clean separation of concerns**: All config write/validation logic properly centralized in `internal/config/writer.go`. The cmd layer is now pure orchestration -- flag parsing, user messaging, and delegation to the library layer. This is a significant improvement.

2. **Comprehensive test coverage in writer_test.go**: 18 test functions covering round-trip writes, sequential updates, unknown keys, invalid values, file-not-found, empty files, and multi-update scenarios. Good use of table-driven tests.

3. **Proper validation pipeline in `Set`**: Validates key+value via `parseConfigValue`, reads existing config, sets override, unmarshals to struct, applies defaults, validates full config, then writes. This ensures no invalid state can be persisted.

4. **`Check` correctly uses direct TOML unmarshal**: Bypasses viper to catch raw TOML syntax errors that viper might silently ignore. Good design decision.

5. **`atomicWrite` correctly removed**: The plan mentioned keeping it as an internal helper, but the implementation correctly removed it since `viper.WriteConfigAs` handles file writing directly.

6. **Clean helpers.go**: Reduced to exactly three concerns (`cmdFS`/`SetFilesystem`, `fileExists`, `ensureDir`). No dead code from the old implementation remains.

7. **`bindFlagsToConfig` uses `Changed` guard**: Prevents flag zero-values from clobbering config file values. Correct approach.

8. **`toml` struct tags added alongside `mapstructure` tags**: Enables direct `toml.Unmarshal` in `Check` while preserving viper compatibility.

---

## Concerns

### CRITICAL (must fix before merge)

None.

### IMPORTANT (should fix before merge)

**I1. `cmd/config/get.go:58` -- Context key mismatch (pre-existing bug, now blocking)**

```go
// get.go line 58:
cfg := cmd.Context().Value("config")  // string key "config"

// root.go line 118:
ctx := context.WithValue(cmd.Context(), config.ConfigKey, globalConfig)  // typed key
```

`config.ConfigKey` is a `configKeyType` (int), not a string. These keys will never match. `config get` will always return "configuration not loaded". This is a pre-existing bug, but since this refactoring touches `get.go` (replacing `getConfigValue` with `config.GetValue`), it should be fixed in scope.

**Fix**: Change `get.go:58` to:
```go
cfg := cmd.Context().Value(config.ConfigKey)
```

**I2. `cmd/config/check.go:49-50` -- Silently ignoring ExpandPath errors**

```go
expandedGlobal, _ := config.ExpandPath(globalPath)
expandedLocal, _ := config.ExpandPath(localPath)
```

If `ExpandPath` fails (e.g., `$HOME` not set), the path will be empty or incorrect, and the subsequent `fileExists` check will silently report "not found" rather than surfacing the real error. Every other command in this refactoring properly checks `ExpandPath` errors.

**Fix**: Check and report the error, or at minimum log it.

**I3. `cmd/config/check.go:55,67` -- Silently ignoring fileExists errors**

```go
globalExists, _ := fileExists(cmdFS, expandedGlobal)
localExists, _ := fileExists(cmdFS, expandedLocal)
```

A permission-denied error on the config file would be silently treated as "file does not exist". The `fileExists` function returns `(false, err)` for non-NotExist errors for exactly this reason.

**Fix**: Check the error return.

### MEDIUM (consider fixing)

**M1. `bindFlagsToConfig` -- `--json` flag not checked for actual value**

```go
if cmd.Flags().Changed("json") {
    cfg.Output.DefaultFormat = "json"
}
```

If a user passes `--json=false` (unusual but valid), `Changed` is true and the format is still set to `"json"`. Should read the actual flag value:

```go
if cmd.Flags().Changed("json") {
    if jsonMode, _ := cmd.Flags().GetBool("json"); jsonMode {
        cfg.Output.DefaultFormat = "json"
    }
}
```

Same applies to `--quiet`.

**M2. `writer_test.go` -- `unmarshalTOML` helper does not use `toml.Unmarshal`**

```go
func unmarshalTOML(data []byte, cfg *Config) error {
    // Import toml at the top of the test file and use it here
    // For now, we'll use viper to do the unmarshaling
    v := viper.New()
    v.SetConfigType("toml")
    if err := v.ReadConfig(bytes.NewReader(data)); err != nil {
        return err
    }
    return v.Unmarshal(cfg)
}
```

The comment says "for now" and the function name implies direct TOML unmarshaling, but it actually goes through viper. This means `TestWriteDefault_OutputIsParseable` does not actually test what `Check` does (direct `toml.Unmarshal`). The test should either use `toml.Unmarshal` directly (matching `Check`'s code path) or the helper should be renamed to reflect what it actually does.

**M3. `ensureDir` in helpers.go is unused**

`ensureDir` has no callers outside test files after this refactoring (directory creation moved into `WriteDefault`). Grep confirms only `helpers.go` and `helpers_test.go` reference it. Consider removing to keep the codebase clean, or mark with a comment explaining future intent.

**M4. Lint violation in `set.go:36` (pre-existing)**

```go
Args: cobra.ExactArgs(2),
```

The `mnd` linter flags the magic number `2`. Extract to a named constant:
```go
const setArgCount = 2
// ...
Args: cobra.ExactArgs(setArgCount),
```

### LOW (optional improvements)

**L1. `parseConfigValue` allocates a map on every call for level validation**

```go
valid := map[string]bool{"debug": true, "info": true, "warn": true, "warning": true, "error": true}
```

This is a micro-optimization concern and negligible for a CLI tool. A `switch` statement would be marginally cleaner and avoid allocation, but this is not worth changing unless other refactoring touches this code.

**L2. `WriteDefault` registers defaults one-by-one instead of using struct**

The function manually lists all 9 default keys with `v.SetDefault(...)`. If a new config key is added to the struct, both `GetDefaults()` and `WriteDefault` must be updated. Consider iterating over the struct fields or adding a comment noting the coupling.

---

## Questions

**Q1.** Was the removal of `config unset` communicated to users? The changes-made.md notes it but there is no migration guide or deprecation notice in the CLI help text.

**Q2.** The `Check` function calls `applyDefaults(&cfg)` before `validate(&cfg)`. This means an empty config file passes validation (empty file -> all zero values -> defaults applied -> validation passes). Is this intentional? It means `config check` on a file with `[database]\npath = ""` would pass because `applyDefaults` fills in the default path. If the intent is to validate what the user actually wrote, defaults should not be applied before validation.

---

## Summary

**Assessment**: CHANGES_NEEDED

**Priority Fixes**:
1. **I1**: Fix context key mismatch in `get.go` (pre-existing bug, breaks `config get`)
2. **I2/I3**: Handle `ExpandPath` and `fileExists` errors in `check.go`
3. **M1**: Check actual `--json` flag value in `bindFlagsToConfig`
4. **M4**: Fix lint violation in `set.go`

**Estimated Risk**: Low -- the core refactoring (`writer.go`, helpers cleanup, cmd delegation) is solid. Issues are in the cmd layer error handling and a pre-existing context bug.

**Testability Score**: Good -- writer_test.go is thorough. The M2 concern about `unmarshalTOML` is minor.

**Files reviewed**:
- `/home/grue/dev/wherehouse/internal/config/writer.go` (NEW)
- `/home/grue/dev/wherehouse/internal/config/config.go` (MODIFIED)
- `/home/grue/dev/wherehouse/internal/config/writer_test.go` (NEW)
- `/home/grue/dev/wherehouse/internal/config/validation.go` (reference)
- `/home/grue/dev/wherehouse/cmd/config/init.go` (MODIFIED)
- `/home/grue/dev/wherehouse/cmd/config/set.go` (MODIFIED)
- `/home/grue/dev/wherehouse/cmd/config/check.go` (MODIFIED)
- `/home/grue/dev/wherehouse/cmd/config/get.go` (MODIFIED)
- `/home/grue/dev/wherehouse/cmd/config/helpers.go` (MODIFIED)
- `/home/grue/dev/wherehouse/cmd/config/config.go` (MODIFIED)
- `/home/grue/dev/wherehouse/cmd/config/edit.go` (reference)
- `/home/grue/dev/wherehouse/cmd/root.go` (MODIFIED)
- `/home/grue/dev/wherehouse/cmd/config/init_test.go` (reference)
