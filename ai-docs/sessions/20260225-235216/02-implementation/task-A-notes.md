# Task A Notes and Deviations

## Deviations from Plan

### 1. `atomicWrite` and `configFilePerms` omitted

The plan specified keeping `atomicWrite` and `configFilePerms` as "internal helpers retained in case needed." However, since `WriteDefault` uses `viper.WriteConfigAs` directly (as designed), neither `atomicWrite` nor `configFilePerms` are called anywhere in writer.go. The `unused` linter enforced in this project (golangci-lint) rejects unused functions and constants as errors. Both were removed to achieve zero linter errors.

**Impact**: None. Future implementers who need atomic writes can add them back with a concrete caller.

### 2. Shadow variable warnings fixed by unique variable names

The plan used `if err := ...` chained error variables in WriteDefault, Set, and Check. The project linter rejects shadowed `err` declarations with `govet`. Fixed by using distinct names: `existsErr`, `mkdirErr`, `parseErr`, `readErr`, `unmarshalErr`, `validateErr`. This is a style difference from the plan but semantically identical.

### 3. `GetValue` - single-case switch rewritten as if-statement

The `database` section in `GetValue` only had one field (`path`). The linter (gocritic `singleCaseSwitch`) requires an `if` statement in that case. Replaced with `if field == "path" { return cfg.Database.Path, nil }`.

### 4. Magic number 2 in `GetValue` extracted to named constant

`strings.SplitN(key, ".", 2)` triggered `mnd` (magic number detector). Extracted to `const keyParts = 2`. The length check `len(parts) != 2` uses the same constant.

### 5. `Check` calls `applyDefaults` before `validate`

The plan's Check implementation called `validate` directly after `toml.Unmarshal`. However, `validateOutput` checks `DefaultFormat` is non-empty ("human" or "json"), and a config file that omits `output.default_format` would fail validation even though it's a valid partial config (defaults fill in the missing value). Added `applyDefaults(&cfg)` before `validate` to match the behavior of `NewWithFS`. This is consistent with how all other callers of `validate` work in this package.

## Implementation Decisions

### viper.WriteConfigAs with afero

Confirmed viper v1.21.0 uses `v.fs.OpenFile(...)` internally when writing, so `afero.MemMapFs` works correctly in tests. No workarounds needed.

### `filepath.Dir` usage

`WriteDefault` calls `fs.MkdirAll(filepath.Dir(path), 0o755)` to create parent directories. This uses the standard library `filepath.Dir` (not afero-specific) since path manipulation is filesystem-agnostic.
