# Architecture Plan: Configuration Refactoring

## Session
`20260225-235216`

## Problem Statement

The current configuration system mixes two distinct concerns:

1. **File I/O and TOML manipulation** - `cmd/config/` directly reads and writes TOML via `go-toml/v2`, parsing raw `map[string]any` structures, validating and writing back. This logic belongs in the library layer, not the CLI layer.

2. **Viper-backed config loading** - `internal/config/loader.go` already uses viper for loading, but then hands off a `*Config` struct. The `get`/`set`/`unset` commands bypass viper entirely and manipulate TOML files directly.

The refactoring goal is to make viper the single point of truth for all config reads and writes, while keeping `internal/config/` as the authoritative business-logic wrapper.

---

## Current State Analysis

### What Works Well (Preserve)
- `internal/config/loader.go`: Viper loading with correct precedence (env vars > local > global > defaults). This is the right approach and should remain essentially unchanged.
- `internal/config/config.go`: Clean `Config` struct with `mapstructure` tags. The `NewWithFS` entry point is well-designed.
- `internal/config/validation.go`: Business rule validation is properly separated.
- `internal/config/defaults.go`: Clean defaults logic.
- `cmd/config/helpers.go`: `atomicWrite`, `fileExists`, `ensureDir` - file utilities that belong in the cmd layer (or can be moved to internal).
- The `afero.Fs` injection pattern for testing is excellent and must be preserved throughout.

### What Needs to Change

#### Problem 1: Direct TOML manipulation in `cmd/config/set.go`
`updateConfigValue` reads the file as `map[string]any`, mutates it, marshals back to TOML, and writes. This:
- Strips TOML comments
- Bypasses viper
- Has its own type-coercion logic (`setValueInMap`) that duplicates validation
- Will silently lose comments and formatting

#### Problem 2: `marshalConfigWithComments` in `cmd/config/helpers.go`
`config init` uses a hand-crafted string builder to produce commented TOML. This is a template for what the config file should look like - it IS the golden file, just not captured as a static file. The golden file requirement for `config init` addresses this directly.

#### Problem 3: `getConfigValue` in `cmd/config/helpers.go`
`config get` uses a manual switch/case dispatch to read fields from `*Config`. After refactoring, the viper singleton (or a viper wrapper) should expose `Get(key string) (any, error)` so the CMD layer doesn't need field-level knowledge.

#### Problem 4: `loadConfigFile` in `cmd/config/helpers.go`
`config check` uses a local `loadConfigFile` that manually unmarshals TOML and validates. After refactoring, `internal/config` should expose a `LoadFile(fs, path)` function (or `NewWithFS` can be reused) so check doesn't need its own loading logic.

#### Problem 5: Missing `toml` struct tags
The existing `TestInitThenCheck` test documents a known bug: `Config` fields have `mapstructure` tags but not `toml` tags, causing `go-toml/v2` to fail on snake_case keys like `default_format`. **This must be fixed as part of the refactoring.**

---

## Target Architecture

### Layer Diagram

```
cmd/config/ (CLI layer)
    ├── init.go     - resolve path, call internal/config.WriteDefault(fs, path, force)
    ├── get.go      - call internal/config.Get(key) or internal/config.All()
    ├── set.go      - call internal/config.Set(fs, path, key, value)
    ├── unset.go    - call internal/config.Unset(fs, path, key)
    ├── check.go    - call internal/config.Check(fs, path)
    ├── edit.go     - unchanged (launches $EDITOR, then calls Check after)
    ├── path.go     - unchanged (just resolves and displays paths)
    └── helpers.go  - ONLY atomicWrite, fileExists, ensureDir (pure FS utilities)

internal/config/ (library layer)
    ├── config.go   - Config struct (ADD toml tags), NewWithFS, ExpandPath
    ├── loader.go   - loadConfig, loadDefaultConfigs, GetGlobalConfigPath, GetLocalConfigPath
    ├── defaults.go - applyDefaults, GetDefaults
    ├── validation.go - validate, Validate (exported)
    ├── database.go - DefaultDatabasePath, GetDatabasePath
    ├── log.go      - DefaultLogPath, GetLogPath
    ├── writer.go   - NEW: WriteDefault, Set, Unset, Check (file write operations)
    └── golden/     - NEW: testdata/config.golden (canonical TOML template)
```

### Key Design Decision: Viper for Reads, Writer for Writes

Viper's role is **read-only** from the perspective of the config system. Viper does not support writing back to config files (it has `WriteConfig` but it strips comments). The write path must remain a separate concern.

The architecture therefore splits into:
- **Read path**: viper singleton via `NewWithFS` (already correct)
- **Write path**: new `internal/config/writer.go` that uses the golden file template

### Component: `internal/config/writer.go`

This is the new file that centralizes all write operations, removing them from `cmd/config/`.

```go
// WriteDefault writes a new config file with default values using the golden template.
// Returns error if file exists and force is false.
func WriteDefault(fs afero.Fs, path string, force bool) error

// Set updates a single key in the given config file.
// Reads via viper (in-memory), validates the full config, writes back via template.
// Returns error if key is unknown or value fails validation.
func Set(fs afero.Fs, path string, key string, value string) error

// Unset removes a single key from the given config file.
// Returns (true, nil) if removed, (false, nil) if key not present.
func Unset(fs afero.Fs, path string, key string) (bool, error)

// Check validates a config file at path. Returns nil if valid.
func Check(fs afero.Fs, path string) error
```

### Golden File for `config init`

The golden file `internal/config/testdata/config.golden` is the canonical template for the initial config file. It contains:
- TOML structure with all keys
- Comments explaining each field
- Placeholder tokens (e.g., `{{.DatabasePath}}`) for values that require runtime computation (the database path default is platform-specific)

Two approaches for the golden file, with trade-offs:

**Option A: Static golden file with placeholder tokens (Recommended)**
```toml
# Wherehouse Configuration File
# See documentation for more details

[database]
# Path to SQLite database file
# Supports ~ for home directory and environment variables
path = "{{.DatabasePath}}"

[user]
# Default user identity for attribution
# Empty string means use OS username
default_identity = ""

# Map OS usernames to display names
# Example: os_username_map = { "jdoe" = "John Doe" }
os_username_map = {}

[output]
# Default output format (human or json)
default_format = "human"

# Enable quiet mode by default
quiet = false
```

`WriteDefault` uses `text/template` to render the golden file with actual default values. This keeps the format authoritative and comment-preserving.

**Option B: Pure static golden file (no tokens)**
Use a static file with hardcoded default strings. Simpler but the database path is platform-specific, making it impossible to embed as a literal.

Option A is recommended because `DefaultDatabasePath()` is platform-specific.

The golden file serves double duty:
1. Template for `config init` output
2. Reference for tests: `TestInitCreatesValidToml` can compare output against the rendered golden file to detect format drift

### Fix: Add `toml` struct tags to `Config`

The `TestInitThenCheck` regression documents that `go-toml/v2` cannot map snake_case TOML keys to CamelCase Go fields without explicit `toml` struct tags. The fix is straightforward:

```go
// Before
type OutputConfig struct {
    DefaultFormat string `mapstructure:"default_format"`
    Quiet         bool   `mapstructure:"quiet"`
}

// After
type OutputConfig struct {
    DefaultFormat string `mapstructure:"default_format" toml:"default_format"`
    Quiet         bool   `mapstructure:"quiet"          toml:"quiet"`
}
```

This must be applied to ALL fields in all config sub-structs. Viper uses `mapstructure` tags; direct `toml.Unmarshal` calls use `toml` tags.

### `cmd/config/set.go` After Refactoring

```go
func runSet(cmd *cobra.Command, args []string) error {
    key := args[0]
    value := args[1]
    local, _ := cmd.Flags().GetBool("local")
    // ... output writer setup ...

    targetPath := resolveTargetPath(local)
    expandedPath, err := config.ExpandPath(targetPath)
    // ... error handling ...

    if err := config.Set(cmdFS, expandedPath, key, value); err != nil {
        out.Error(err.Error())
        return err
    }

    out.Success("Configuration updated")
    out.KeyValue(key, value)
    out.KeyValue("File", expandedPath)
    return nil
}
```

The `updateConfigValue`, `setValueInMap`, `unsetValueInMap` functions are removed from `cmd/config/helpers.go` and replaced by the library-level `Set`/`Unset` functions.

### `cmd/config/get.go` After Refactoring

`config get` currently receives `*Config` from cobra context and does field-level dispatch. After refactoring, the approach depends on the `--sources` flag requirement:

- Without `--sources`: The existing behavior (get merged `*Config`, display fields) is fine. The viper singleton already handles merging. No change needed.
- With `--sources`: Requires viper to expose which source each key came from. Viper does support `v.IsSet()` and `v.GetString()` with source awareness, but there is no public API for "which file did this come from". This remains a TODO (as documented in the existing code).

The `getConfigValue` function in `helpers.go` (the switch/case dispatch) can be replaced by a library-level `Get(cfg *Config, key string) (any, error)` exported from `internal/config/`. This reduces duplication but is a minor refactoring. Alternatively, keeping `getConfigValue` in `helpers.go` is acceptable if the goal is to minimize scope.

**Recommendation**: Move `getConfigValue` to `internal/config/` as `GetValue(cfg *Config, key string) (any, error)`. This gives a single place to update when fields are added.

### `cmd/config/check.go` After Refactoring

```go
func runCheck(cmd *cobra.Command, _ []string) error {
    // ... output writer setup ...

    globalExists, _ := fileExists(cmdFS, expandedGlobal)
    if globalExists {
        if err := config.Check(cmdFS, expandedGlobal); err != nil {
            out.Error(...)
        } else {
            out.Success(...)
        }
    }
    // same for local
}
```

`loadConfigFile` in `helpers.go` is removed.

### `cmd/config/helpers.go` After Refactoring

Only pure filesystem utilities remain:
- `fileExists`
- `ensureDir`
- `atomicWrite`
- `cmdFS` and `SetFilesystem`
- Constants: `configFilePerms`, `keyValueParts`

`marshalConfigWithComments`, `getConfigValue`, `setValueInMap`, `unsetValueInMap`, `loadConfigFile`, `determineConfigPath` are all removed.

---

## Implementation Sequence

### Phase 1: Fix `toml` struct tags (no behavior change)
- Add `toml` tags to all `Config` sub-struct fields in `internal/config/config.go`
- This fixes the `TestInitThenCheck` regression
- Tests: `TestInitThenCheck` and `TestInitCreatesValidToml` should pass after this

### Phase 2: Create golden file and `writer.go`
- Create `internal/config/testdata/config.golden`
- Create `internal/config/writer.go` with `WriteDefault`, `Set`, `Unset`, `Check`
- Move `atomicWrite` to `internal/config/writer.go` (or leave a copy in helpers and call through)
- Recommendation: Keep `atomicWrite` in `cmd/config/helpers.go` and have `writer.go` accept a `writeFunc` parameter, OR move it to `internal/config/` and have `cmd/config/helpers.go` delegate to it. The simpler approach is to duplicate it (it is small) or move it entirely to `internal/config/`.

### Phase 3: Refactor `cmd/config/set.go` and `cmd/config/unset.go`
- Replace `updateConfigValue` with `config.Set`
- Replace `unsetFromFile` with `config.Unset`
- Remove `setValueInMap`, `unsetValueInMap` from `helpers.go`

### Phase 4: Refactor `cmd/config/init.go`
- Replace `marshalConfigWithComments` + `atomicWrite` with `config.WriteDefault`
- Remove `marshalConfigWithComments` from `helpers.go`

### Phase 5: Refactor `cmd/config/check.go` and `cmd/config/get.go`
- Replace `loadConfigFile` with `config.Check`
- Optionally move `getConfigValue` to `internal/config/GetValue`
- Remove `loadConfigFile`, `determineConfigPath` from `helpers.go`

### Phase 6: Update tests
- Add golden file comparison test in `cmd/config/init_test.go`
- Update `set_test.go` to test actual set behavior (currently very thin)
- Ensure `TestInitThenCheck` passes throughout

---

## Files to Create
- `/home/grue/dev/wherehouse/internal/config/writer.go`
- `/home/grue/dev/wherehouse/internal/config/testdata/config.golden`

## Files to Modify
- `/home/grue/dev/wherehouse/internal/config/config.go` - Add `toml` struct tags
- `/home/grue/dev/wherehouse/cmd/config/helpers.go` - Remove business logic functions
- `/home/grue/dev/wherehouse/cmd/config/init.go` - Use `config.WriteDefault`
- `/home/grue/dev/wherehouse/cmd/config/set.go` - Use `config.Set`
- `/home/grue/dev/wherehouse/cmd/config/unset.go` - Use `config.Unset`
- `/home/grue/dev/wherehouse/cmd/config/check.go` - Use `config.Check`
- `/home/grue/dev/wherehouse/cmd/config/get.go` - Optionally use `config.GetValue`
- `/home/grue/dev/wherehouse/cmd/config/init_test.go` - Add golden file test

---

## Trade-offs and Alternatives Considered

### Alternative: Use viper `WriteConfig` for writes
Viper's `WriteConfig` / `WriteConfigAs` does write back to file, but it strips comments and reformats the TOML. This violates the requirement that `config init` produces a well-commented, stable file format. Rejected.

### Alternative: Keep `setValueInMap` in `cmd/config/`
This avoids moving business logic to the library layer but leaves duplication between the type-coercion logic in `setValueInMap` and the validation in `internal/config/validation.go`. Rejected per requirement 5 (consolidate business logic).

### Alternative: Use `embed.FS` for golden file
The golden file could be embedded with `//go:embed testdata/config.golden` in `writer.go`. This ensures the template is always available at runtime without filesystem access. This is the recommended approach for `WriteDefault`.

### Decision: `atomicWrite` location
Moving `atomicWrite` to `internal/config/writer.go` is cleaner (all write operations in one place). `cmd/config/helpers.go` can then either re-export it or remove it entirely. Since `atomicWrite` is only used in `cmd/config/` via the new library calls, removing it from `helpers.go` is the right outcome.

---

## Risk Assessment

**Low risk:**
- Adding `toml` struct tags: purely additive, no behavior change for viper (which uses `mapstructure`)
- Golden file creation: new file, no existing code changes

**Medium risk:**
- `writer.go` Set/Unset implementation: must correctly handle the case where the config file has keys in it that were set outside of viper (e.g., user manually edited). The approach of "load via viper, modify, render via template" may lose user customizations. The safer approach is to use the `go-toml/v2` AST (document manipulation) to do surgical edits. This preserves comments and structure in user-edited files.

**Mitigation for Set/Unset**: Use `go-toml/v2`'s document API (`toml.Tree` or equivalent) for surgical key updates rather than round-tripping through viper. This preserves user edits and is consistent with the current behavior.

### Note on `go-toml/v2` AST vs. viper round-trip for Set/Unset

The current `updateConfigValue` already does a round-trip through `toml.Marshal`/`toml.Unmarshal` which strips comments. Moving this to `internal/config/writer.go` doesn't worsen the situation but also doesn't improve it.

If comment preservation on `set`/`unset` is a hard requirement, we need `go-toml/v2`'s `toml.Tree` (the v1 API) or another approach. In `go-toml/v2` there is no public AST manipulation API - only the v1 `toml` package had `Tree`. This is a significant constraint.

**Decision**: Accept comment loss on `set`/`unset` for now (same as current behavior). The golden file only applies to `config init`. Comments on user-edited configs will be lost on next `set`/`unset`, which is the existing behavior.
