# Final Implementation Plan: Configuration Refactoring (Revised)

## Session
`20260225-235216`

## Status
Incorporates user feedback from `user-feedback.md`: viper-native write operations, viper-backed config struct, flag binding strategy, and removal of `config unset` command. Supersedes previous version.

---

## Summary of Changes from Previous Plan

1. **Eliminated `marshalConfigWithComments`** - viper writes TOML natively via `pelletier/go-toml/v2`. No custom TOML serialization code.
2. **`WriteDefault` uses viper `SetDefault` + `WriteConfigAs`** - produces clean TOML with all sections/keys, no comments.
3. **`Set` uses viper `Set` + `WriteConfigAs`** - loads file, updates key, rewrites.
4. **`config unset` command REMOVED** - viper has no key-delete API; the command is deleted entirely. Users use `config set` with the default value instead.
5. **Flag binding strategy documented** - which persistent flags bind to viper keys.
6. **`toml` struct tags still needed** - `config check` uses `toml.Unmarshal` directly.
7. **No golden file, no `text/template`** - viper output is the canonical format.

---

## Architecture After Refactoring

```
cmd/config/ (CLI layer - orchestration only)
    ├── init.go     - call config.WriteDefault(fs, path, force)
    ├── get.go      - call config.GetValue(cfg, key) or marshal cfg
    ├── set.go      - call config.Set(fs, path, key, value)
    ├── check.go    - call config.Check(fs, path)
    ├── edit.go     - unchanged
    ├── path.go     - unchanged
    └── helpers.go  - fileExists, ensureDir, cmdFS, SetFilesystem, constants

internal/config/ (library layer - all business logic)
    ├── config.go      - Config struct (ADD toml tags), NewWithFS, ExpandPath
    ├── loader.go      - loadConfig, GetGlobalConfigPath, GetLocalConfigPath
    ├── defaults.go    - applyDefaults, GetDefaults
    ├── validation.go  - Validate (exported), validate (internal)
    ├── database.go    - DefaultDatabasePath, GetDatabasePath
    ├── log.go         - DefaultLogPath, GetLogPath
    └── writer.go      - NEW: WriteDefault, Set, Check, atomicWrite, GetValue
```

Note: `Unset` is NOT in writer.go. `atomicWrite` is kept in writer.go for `WriteDefault` if needed as an internal implementation detail, but is not required for `Set` (which uses `viper.WriteConfigAs` directly).

---

## Viper Behavior Research (Codebase-Verified)

Before implementing, these facts were verified against viper v1.21.0 source:

**`viper.WriteConfig` / `viper.WriteConfigAs`:**
- Uses `v.fs` (afero) for the file open call (`v.fs.OpenFile(...)`). Compatible with `afero.MemMapFs` for testing.
- Determines config type from file extension (`.toml` → TOML). Must use `.toml` extension.
- Uses `v.AllSettings()` as the data source, which merges: aliases, overrides, pflags, env, config file, kvstore, **and defaults**. This means `SetDefault` values ARE written by `WriteConfigAs`.
- Encodes via `pelletier/go-toml/v2` (`toml.Marshal(map[string]any)`). No comments in output.
- Writes with `O_CREATE | O_TRUNC | O_WRONLY` flags.

**`viper.AllKeys()` includes:**
- Keys set via `SetDefault`
- Keys set via `Set`
- Keys from loaded config files
- Keys from environment variables and pflags

**`viper.Set` behavior:**
- Sets a key in `v.override` map (highest priority after aliases).
- `AllSettings()` will include this value.
- After `v.ReadInConfig()` + `v.Set(key, val)` + `v.WriteConfigAs(path)`, the file contains the merged result: all read keys plus the override.

**No viper key-delete API:**
- `viper.Unset` does not exist in viper v1.21.0.
- This is the reason `config unset` is removed entirely. No clean implementation exists without raw TOML map manipulation.

---

## Phase-by-Phase Implementation

---

### Phase 1: Add `toml` struct tags in `internal/config/config.go`

**Goal**: Fix `TestInitThenCheck` and `TestInitCreatesValidToml` regression bugs. Also required for `config check` which calls `toml.Unmarshal(data, &cfg)` directly (in `cmd/config/helpers.go:loadConfigFile`, moved to `internal/config/writer.go:Check`).

**File**: `/home/grue/dev/wherehouse/internal/config/config.go`

**Exact changes** - add `toml` tags alongside existing `mapstructure` tags:

```go
type Config struct {
    Database DatabaseConfig `mapstructure:"database" toml:"database"`
    Logging  LoggingConfig  `mapstructure:"logging"  toml:"logging"`
    User     UserConfig     `mapstructure:"user"     toml:"user"`
    Output   OutputConfig   `mapstructure:"output"   toml:"output"`
}

type LoggingConfig struct {
    FilePath   string `mapstructure:"file_path"    toml:"file_path"`
    Level      string `mapstructure:"level"        toml:"level"`
    MaxSizeMB  int    `mapstructure:"max_size_mb"  toml:"max_size_mb"`
    MaxBackups int    `mapstructure:"max_backups"  toml:"max_backups"`
}

type DatabaseConfig struct {
    Path string `mapstructure:"path" toml:"path"`
}

type UserConfig struct {
    DefaultIdentity string            `mapstructure:"default_identity" toml:"default_identity"`
    OSUsernameMap   map[string]string `mapstructure:"os_username_map"  toml:"os_username_map"`
}

type OutputConfig struct {
    DefaultFormat string `mapstructure:"default_format" toml:"default_format"`
    Quiet         bool   `mapstructure:"quiet"          toml:"quiet"`
}
```

**Why `toml` tags are still needed despite switching to viper write:**
- `config.Check(fs, path)` in `writer.go` must validate an arbitrary config file by parsing it directly with `toml.Unmarshal(data, &cfg)`. Without `toml` tags, snake_case keys like `default_format` do not map to `DefaultFormat`.
- The existing `config check` test `TestInitThenCheck` documents this exact regression.

**Tests that become green after this phase**: `TestInitThenCheck`, `TestInitCreatesValidToml`

---

### Phase 2: Create `internal/config/writer.go`

**Goal**: Centralize all write operations using viper-native write where possible. `atomicWrite` is kept as an internal helper in case `WriteDefault` needs it; it is not used by `Set`.

**File to create**: `/home/grue/dev/wherehouse/internal/config/writer.go`

**Complete imports and function signatures**:

```go
package config

import (
    "fmt"
    "strings"

    "github.com/pelletier/go-toml/v2"
    "github.com/spf13/afero"
    "github.com/spf13/viper"
)

const (
    configFilePerms = 0o644
)
```

Note: `toml` import is needed for `Check` (uses `toml.Unmarshal`). It is NOT needed for `Unset` (removed). If `atomicWrite` is not used by `WriteDefault`, `toml` may be the only non-viper write dependency.

#### `atomicWrite` (moved from `cmd/config/helpers.go`)

```go
// atomicWrite writes data to path atomically via a temp file + rename.
// Prevents partial writes if the process is interrupted.
// Retained as an internal helper; used if WriteDefault cannot use viper.WriteConfigAs directly.
func atomicWrite(fs afero.Fs, path string, data []byte) error {
    tempPath := path + ".tmp"
    if err := afero.WriteFile(fs, tempPath, data, configFilePerms); err != nil {
        return fmt.Errorf("writing temp file: %w", err)
    }
    if err := fs.Rename(tempPath, path); err != nil {
        _ = fs.Remove(tempPath)
        return fmt.Errorf("renaming temp file: %w", err)
    }
    return nil
}
```

Note: The existing `atomicWrite` in `cmd/config/helpers.go` takes an `os.FileMode` parameter that is unused (`_ os.FileMode`). The new version drops this parameter - it always uses `configFilePerms`.

#### `newViperForFile` (private helper)

```go
// newViperForFile creates a configured viper instance for a single file.
// Sets the afero filesystem and config file path. Does not read the file.
func newViperForFile(fs afero.Fs, path string) *viper.Viper {
    v := viper.New()
    v.SetFs(fs)
    v.SetConfigFile(path)
    v.SetConfigType("toml")
    return v
}
```

#### `WriteDefault`

```go
// WriteDefault writes a new config file at path with all default values.
// Uses viper to produce TOML output - no comments, but all keys present.
// Returns error if file already exists and force is false.
// Creates parent directories as needed.
//
// Output format: clean TOML with four sections [database], [logging], [user], [output].
// All keys are written with their default values. Comments are not included.
// Users who want comments should run 'config edit' to annotate manually.
func WriteDefault(fs afero.Fs, path string, force bool) error {
    exists, err := afero.Exists(fs, path)
    if err != nil {
        return fmt.Errorf("checking config file: %w", err)
    }
    if exists && !force {
        return fmt.Errorf("configuration file already exists: %s", path)
    }

    if err := fs.MkdirAll(filepath.Dir(path), 0755); err != nil {
        return fmt.Errorf("creating directory: %w", err)
    }

    v := newViperForFile(fs, path)
    defaults := GetDefaults()

    // Register all defaults with viper so AllSettings() includes them
    v.SetDefault("database.path", defaults.Database.Path)
    v.SetDefault("logging.file_path", defaults.Logging.FilePath)
    v.SetDefault("logging.level", defaults.Logging.Level)
    v.SetDefault("logging.max_size_mb", defaults.Logging.MaxSizeMB)
    v.SetDefault("logging.max_backups", defaults.Logging.MaxBackups)
    v.SetDefault("user.default_identity", defaults.User.DefaultIdentity)
    v.SetDefault("user.os_username_map", defaults.User.OSUsernameMap)
    v.SetDefault("output.default_format", defaults.Output.DefaultFormat)
    v.SetDefault("output.quiet", defaults.Output.Quiet)

    // WriteConfigAs writes AllSettings() as TOML using pelletier/go-toml/v2
    // Uses v.fs (the afero filesystem we set above)
    return v.WriteConfigAs(path)
}
```

Note on output format: viper writes TOML via `toml.Marshal(map[string]any)`. The section order is determined by `go-toml/v2` marshaling of a `map[string]any`, which sorts keys alphabetically. Expected output order: `[database]`, `[logging]`, `[output]`, `[user]` (alphabetical). This differs from the current `marshalConfigWithComments` order (`[database]`, `[user]`, `[output]`). This is acceptable per user feedback.

#### `Set`

```go
// Set updates a single key-value pair in the config file at path.
// Reads the existing config via viper, sets the override, validates the full
// merged config, then rewrites via viper.WriteConfigAs.
//
// Supported keys:
//   database.path
//   logging.file_path
//   logging.level           (must be: debug, info, warn, warning, error)
//   logging.max_size_mb     (non-negative integer)
//   logging.max_backups     (non-negative integer)
//   user.default_identity
//   output.default_format   (must be: human, json)
//   output.quiet            (must be: true, false)
//
// Note: user.os_username_map is a map type and is NOT settable via this function.
// Use 'config edit' to modify map values.
//
// Returns error if key is unknown, value fails type conversion, file cannot be
// read/written, or the resulting config fails validation.
func Set(fs afero.Fs, path string, key string, value string) error {
    // Validate key is known and parse/type-check value
    parsedValue, err := parseConfigValue(key, value)
    if err != nil {
        return err
    }

    v := newViperForFile(fs, path)
    if err := v.ReadInConfig(); err != nil {
        return fmt.Errorf("reading config file: %w", err)
    }

    v.Set(key, parsedValue)

    // Validate the full merged config before writing
    var cfg Config
    if err := v.Unmarshal(&cfg); err != nil {
        return fmt.Errorf("invalid configuration: %w", err)
    }
    applyDefaults(&cfg)
    if err := validate(&cfg); err != nil {
        return fmt.Errorf("configuration validation failed: %w", err)
    }

    return v.WriteConfigAs(path)
}
```

#### `Check`

```go
// Check validates the config file at path.
// Reads the file directly (not via viper) to test the raw TOML parse path.
// Returns nil if the file is valid TOML and passes all validation constraints.
func Check(fs afero.Fs, path string) error {
    data, err := afero.ReadFile(fs, path)
    if err != nil {
        return fmt.Errorf("reading config file: %w", err)
    }

    var cfg Config
    if err := toml.Unmarshal(data, &cfg); err != nil {
        return fmt.Errorf("parsing config file: %w", err)
    }

    if err := validate(&cfg); err != nil {
        return fmt.Errorf("validating config: %w", err)
    }

    return nil
}
```

Note: `Check` uses `toml.Unmarshal` directly (not viper) to catch raw TOML syntax errors that viper might silently ignore. This is why Phase 1 `toml` struct tags are required.

#### `GetValue`

```go
// GetValue returns the value of a dot-separated config key from cfg.
// Supports all config keys including logging.* and user.os_username_map.
// Returns (value, nil) on success or (nil, error) for unknown keys.
func GetValue(cfg *Config, key string) (any, error) {
    parts := strings.SplitN(key, ".", 2)
    if len(parts) != 2 {
        return nil, fmt.Errorf("invalid key format %q (expected section.key)", key)
    }

    section, field := parts[0], parts[1]

    switch section {
    case "database":
        switch field {
        case "path":
            return cfg.Database.Path, nil
        }
    case "logging":
        switch field {
        case "file_path":
            return cfg.Logging.FilePath, nil
        case "level":
            return cfg.Logging.Level, nil
        case "max_size_mb":
            return cfg.Logging.MaxSizeMB, nil
        case "max_backups":
            return cfg.Logging.MaxBackups, nil
        }
    case "user":
        switch field {
        case "default_identity":
            return cfg.User.DefaultIdentity, nil
        case "os_username_map":
            return cfg.User.OSUsernameMap, nil
        }
    case "output":
        switch field {
        case "default_format":
            return cfg.Output.DefaultFormat, nil
        case "quiet":
            return cfg.Output.Quiet, nil
        }
    }

    return nil, fmt.Errorf("unknown configuration key %q", key)
}
```

#### `parseConfigValue` (private helper)

```go
// parseConfigValue validates and parses a string value for the given config key.
// Returns the typed value ready for viper.Set() or an error if invalid.
// This is the single source of truth for type coercion and per-key validation.
func parseConfigValue(key, value string) (any, error) {
    switch key {
    case "database.path":
        return value, nil
    case "logging.file_path":
        return value, nil
    case "logging.level":
        normalized := strings.ToLower(value)
        valid := map[string]bool{"debug": true, "info": true, "warn": true, "warning": true, "error": true}
        if !valid[normalized] {
            return nil, fmt.Errorf("logging.level must be one of [debug, info, warn, warning, error], got %q", value)
        }
        return normalized, nil
    case "logging.max_size_mb":
        n, err := strconv.Atoi(value)
        if err != nil || n < 0 {
            return nil, fmt.Errorf("logging.max_size_mb must be a non-negative integer, got %q", value)
        }
        return n, nil
    case "logging.max_backups":
        n, err := strconv.Atoi(value)
        if err != nil || n < 0 {
            return nil, fmt.Errorf("logging.max_backups must be a non-negative integer, got %q", value)
        }
        return n, nil
    case "user.default_identity":
        return value, nil
    case "output.default_format":
        if value != "human" && value != "json" {
            return nil, fmt.Errorf("output.default_format must be 'human' or 'json', got %q", value)
        }
        return value, nil
    case "output.quiet":
        b, err := strconv.ParseBool(value)
        if err != nil {
            return nil, fmt.Errorf("output.quiet must be 'true' or 'false', got %q", value)
        }
        return b, nil
    default:
        return nil, fmt.Errorf("unknown configuration key %q", key)
    }
}
```

Note: `user.os_username_map` is intentionally not in this switch. Callers of `Set(key, value)` will get "unknown configuration key" for map-type keys. The `GetValue` function does support reading `user.os_username_map`.

---

### Phase 3: Refactor `cmd/config/init.go`

**File**: `/home/grue/dev/wherehouse/cmd/config/init.go`

**Change**: Replace `fileExists` check + `ensureDir` + `marshalConfigWithComments` + `atomicWrite` block with a single `config.WriteDefault` call.

The "already exists" check moves inside `config.WriteDefault`. The `out.Error` + `out.Info` user messages for the "already exists" case must remain in `cmd/config/init.go` since they are CLI concerns. We detect the "already exists" error by checking `os.IsExist` or string matching.

**Before** (lines 75-107):
```go
exists, err := fileExists(cmdFS, expandedPath)
if err != nil { ... }
if exists && !force {
    out.Error(fmt.Sprintf("configuration file already exists: %s", expandedPath))
    out.Info("Use --force to overwrite")
    return fmt.Errorf("configuration file already exists: %s", expandedPath)
}

dir := filepath.Dir(expandedPath)
err = ensureDir(cmdFS, dir)
if err != nil { ... }

cfg := config.GetDefaults()
data := marshalConfigWithComments(cfg)
err = atomicWrite(cmdFS, expandedPath, data, configFilePerms)
if err != nil { ... }
```

**After**:
```go
if err := config.WriteDefault(cmdFS, expandedPath, force); err != nil {
    // Distinguish "already exists" for user-friendly messaging
    if !force && strings.Contains(err.Error(), "already exists") {
        out.Error(fmt.Sprintf("configuration file already exists: %s", expandedPath))
        out.Info("Use --force to overwrite")
    } else {
        out.Error(fmt.Sprintf("failed to write configuration: %v", err))
    }
    return err
}
```

**Remove imports no longer needed**: `"path/filepath"` (MkdirAll is now inside WriteDefault).

---

### Phase 4: Refactor `cmd/config/set.go`

**File**: `/home/grue/dev/wherehouse/cmd/config/set.go`

**Change**: Replace `updateConfigValue` with `config.Set`. Remove the entire `updateConfigValue` function.

**`runSet` after refactoring**:
```go
func runSet(cmd *cobra.Command, args []string) error {
    key := args[0]
    value := args[1]
    local, _ := cmd.Flags().GetBool("local")

    jsonMode, _ := cmd.Flags().GetBool("json")
    quietMode, _ := cmd.Flags().GetBool("quiet")
    out := cli.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), jsonMode, quietMode)

    var targetPath string
    if local {
        targetPath = config.GetLocalConfigPath()
    } else {
        targetPath = config.GetGlobalConfigPath()
    }

    expandedPath, err := config.ExpandPath(targetPath)
    if err != nil {
        out.Error(fmt.Sprintf("invalid path %q: %v", targetPath, err))
        return fmt.Errorf("invalid path %q: %w", targetPath, err)
    }

    exists, err := fileExists(cmdFS, expandedPath)
    if err != nil {
        out.Error(fmt.Sprintf("checking config file: %v", err))
        return fmt.Errorf("checking config file: %w", err)
    }
    if !exists {
        if local {
            out.Error("no local configuration file found")
            out.Info("Run 'wherehouse config init --local' to create one")
            return errors.New("no local configuration file found")
        }
        out.Error("no global configuration file found")
        out.Info("Run 'wherehouse config init' to create one")
        out.Info("Or use 'wherehouse config set --local ...' for project-specific config")
        return errors.New("no global configuration file found")
    }

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

**Remove from `set.go`**: The entire `updateConfigValue` function (lines 48-95).

**Remove imports no longer needed**: `"github.com/pelletier/go-toml/v2"`, `"github.com/spf13/afero"`.

---

### Phase 5: DELETE `cmd/config/unset.go` and `cmd/config/unset_test.go`

**Files to DELETE**:
- `/home/grue/dev/wherehouse/cmd/config/unset.go`
- `/home/grue/dev/wherehouse/cmd/config/unset_test.go`

**Rationale**: viper v1.21.0 has no key-delete API. A clean `Unset` implementation requires raw TOML map manipulation (`deleteFromMap` + `atomicWrite`). The user has decided this complexity is not worth it. Users who want to "unset" a value can use `config set <key> <default>`.

**Also remove** the `unset` subcommand registration from the config command. This is typically in `cmd/config/config.go` or `cmd/root.go`:

```go
// REMOVE this line (or equivalent):
configCmd.AddCommand(newUnsetCmd())
```

**No replacement needed.** The `Unset` function is NOT added to `writer.go`. The `deleteFromMap` helper is NOT added to `writer.go`.

---

### Phase 6: Refactor `cmd/config/check.go`

**File**: `/home/grue/dev/wherehouse/cmd/config/check.go`

**Change**: Replace `loadConfigFile` calls with `config.Check` calls. No other logic changes.

**Before**:
```go
if err := loadConfigFile(cmdFS, expandedGlobal); err != nil {
```

**After**:
```go
if err := config.Check(cmdFS, expandedGlobal); err != nil {
```

Same for local. `loadConfigFile` is then deleted from `helpers.go`.

No import changes needed (`config` package already imported).

---

### Phase 7: Refactor `cmd/config/get.go`

**File**: `/home/grue/dev/wherehouse/cmd/config/get.go`

**Change**: Replace `getConfigValue(globalConfig, key)` with `config.GetValue(globalConfig, key)`.

**Before**:
```go
value, err := getConfigValue(globalConfig, key)
```

**After**:
```go
value, err := config.GetValue(globalConfig, key)
```

`getConfigValue` is then deleted from `helpers.go`.

The "show all" path (`toml.Marshal(globalConfig)`) remains unchanged - `get.go` keeps its `go-toml/v2` import for this purpose.

---

### Phase 8: Clean up `cmd/config/helpers.go`

**File**: `/home/grue/dev/wherehouse/cmd/config/helpers.go`

**Remove** the following functions (moved to `internal/config/writer.go` or eliminated):
- `atomicWrite` - moved to `internal/config/writer.go`
- `marshalConfigWithComments` - eliminated (viper WriteConfigAs replaces it)
- `getConfigValue` - moved as `GetValue` to `internal/config/writer.go`
- `setValueInMap` - moved as `parseConfigValue` (private) to `internal/config/writer.go`
- `unsetValueInMap` - eliminated (no longer needed; `config unset` command removed)
- `loadConfigFile` - moved as `Check` to `internal/config/writer.go`
- `determineConfigPath` - confirm unused with grep, then delete

**Remove constants no longer needed**:
- `configFilePerms` - moved to `internal/config/writer.go`
- `keyValueParts` - replaced by `strings.SplitN(key, ".", 2)` approach

**Keep**:
- `cmdFS` variable and `SetFilesystem` function
- `fileExists` function
- `ensureDir` function (kept for potential future use; verify no callers after Phase 8)

**After cleanup**, `helpers.go` contains:
```go
package config

import (
    "os"
    "github.com/spf13/afero"
)

// cmdFS is the filesystem abstraction used by all config commands.
var cmdFS afero.Fs = afero.NewOsFs()

// SetFilesystem allows injecting a filesystem implementation for testing.
func SetFilesystem(fs afero.Fs) {
    cmdFS = fs
}

// fileExists checks if a file exists and is accessible.
func fileExists(fs afero.Fs, path string) (bool, error) {
    _, err := fs.Stat(path)
    if err == nil {
        return true, nil
    }
    if os.IsNotExist(err) {
        return false, nil
    }
    return false, err
}

// ensureDir creates a directory and all parent directories if they don't exist.
func ensureDir(fs afero.Fs, path string) error {
    return fs.MkdirAll(path, 0755)
}
```

---

### Phase 9: Add programmatic tests for `writer.go`

**File**: `/home/grue/dev/wherehouse/internal/config/writer_test.go` (new file)

Tests use `afero.NewMemMapFs()` throughout. No golden files. No static fixtures.

**Test list**:

1. `TestWriteDefault_AllDefaultsRoundTrip` - write via `WriteDefault`, read back via viper, verify all keys match `GetDefaults()` values
2. `TestWriteDefault_CreatesFile` - verify file is created in memfs
3. `TestWriteDefault_FailsIfExists` - returns error when `force=false` and file exists
4. `TestWriteDefault_ForceOverwrites` - `force=true` overwrites existing file
5. `TestWriteDefault_CreatesParentDirs` - creates missing parent directories
6. `TestWriteDefault_OutputIsParseable` - viper output is valid TOML with expected sections
7. `TestSet_UpdatesValue` - parameterized table test, all settable keys (verify via viper re-read)
8. `TestSet_UnknownKey` - returns error for unknown key
9. `TestSet_InvalidValue` - returns error for bad value (e.g., `logging.level = "verbose"`, `output.quiet = "maybe"`)
10. `TestSet_FileNotFound` - returns error when file does not exist
11. `TestCheck_ValidFile` - returns nil for valid TOML
12. `TestCheck_InvalidToml` - returns error for malformed TOML
13. `TestCheck_FailsValidation` - returns error for TOML with invalid values (e.g., `output.default_format = "xml"`)
14. `TestGetValue_AllKeys` - parameterized test for all supported keys including `logging.*` and `user.os_username_map`
15. `TestGetValue_UnknownKey` - returns error

Note: Tests for `Unset` are removed since the function no longer exists.

**Round-trip test pattern** (`TestWriteDefault_AllDefaultsRoundTrip`):
```go
func TestWriteDefault_AllDefaultsRoundTrip(t *testing.T) {
    fs := afero.NewMemMapFs()
    path := "/tmp/test-config.toml"

    err := WriteDefault(fs, path, false)
    require.NoError(t, err)

    // Read back through viper to verify all keys/values
    v := viper.New()
    v.SetFs(fs)
    v.SetConfigFile(path)
    v.SetConfigType("toml")
    require.NoError(t, v.ReadInConfig())

    defaults := GetDefaults()
    assert.Equal(t, defaults.Database.Path, v.GetString("database.path"))
    assert.Equal(t, defaults.Logging.Level, v.GetString("logging.level"))
    assert.Equal(t, defaults.Logging.FilePath, v.GetString("logging.file_path"))
    assert.Equal(t, defaults.Logging.MaxSizeMB, v.GetInt("logging.max_size_mb"))
    assert.Equal(t, defaults.Logging.MaxBackups, v.GetInt("logging.max_backups"))
    assert.Equal(t, defaults.User.DefaultIdentity, v.GetString("user.default_identity"))
    assert.Equal(t, defaults.Output.DefaultFormat, v.GetString("output.default_format"))
    assert.Equal(t, defaults.Output.Quiet, v.GetBool("output.quiet"))
}
```

**Set round-trip test pattern** (`TestSet_UpdatesValue`):
```go
func TestSet_UpdatesValue(t *testing.T) {
    cases := []struct {
        key    string
        value  string
        verify func(t *testing.T, v *viper.Viper)
    }{
        {"database.path", "/custom/db.sqlite", func(t *testing.T, v *viper.Viper) {
            assert.Equal(t, "/custom/db.sqlite", v.GetString("database.path"))
        }},
        {"logging.level", "debug", func(t *testing.T, v *viper.Viper) {
            assert.Equal(t, "debug", v.GetString("logging.level"))
        }},
        {"logging.max_size_mb", "100", func(t *testing.T, v *viper.Viper) {
            assert.Equal(t, 100, v.GetInt("logging.max_size_mb"))
        }},
        {"output.quiet", "true", func(t *testing.T, v *viper.Viper) {
            assert.True(t, v.GetBool("output.quiet"))
        }},
        {"output.default_format", "json", func(t *testing.T, v *viper.Viper) {
            assert.Equal(t, "json", v.GetString("output.default_format"))
        }},
        // ... all other settable keys
    }

    for _, tc := range cases {
        t.Run(tc.key, func(t *testing.T) {
            fs := afero.NewMemMapFs()
            path := "/tmp/test-config.toml"
            require.NoError(t, WriteDefault(fs, path, false))
            require.NoError(t, Set(fs, path, tc.key, tc.value))

            v := viper.New()
            v.SetFs(fs)
            v.SetConfigFile(path)
            v.SetConfigType("toml")
            require.NoError(t, v.ReadInConfig())
            tc.verify(t, v)
        })
    }
}
```

---

## Flag Binding Strategy

The user feedback requests binding persistent flags to viper so config file values serve as defaults and flag values override them.

**Current persistent flags in `cmd/root.go`**:
```
--config    string   config file path         (transient: excluded from binding)
--no-config bool     skip config files        (transient: excluded from binding)
--db        string   database file path       (bind → "database.path")
--as        string   override user identity   (bind → "user.default_identity")
--json      bool     JSON output              (bind → "output.default_format" indirectly)
--quiet     count    quiet mode               (bind → "output.quiet")
```

**Binding implementation** in `initConfig` in `cmd/root.go`:

```go
// After loading config and before using it, apply flag overrides.
// Flag values override config file values which override defaults.
func bindFlagsToConfig(cmd *cobra.Command, cfg *config.Config) {
    if cmd.Flags().Changed("db") {
        if val, _ := cmd.Flags().GetString("db"); val != "" {
            cfg.Database.Path = val
        }
    }
    if cmd.Flags().Changed("as") {
        if val, _ := cmd.Flags().GetString("as"); val != "" {
            cfg.User.DefaultIdentity = val
        }
    }
    if cmd.Flags().Changed("json") {
        cfg.Output.DefaultFormat = "json"
    }
    // --quiet is a count flag; apply to config if changed
    if cmd.Flags().Changed("quiet") {
        cfg.Output.Quiet = true
    }
}
```

This approach applies flag overrides directly to the `*Config` struct after loading (simpler than viper `BindPFlag` for this use case, since we already use `cfg.*` struct access throughout the app, not `viper.Get()`).

**Alternative: `v.BindPFlag`** would require threading the viper instance out of `loadConfig` to `initConfig` in `root.go`. Since `loadConfig` creates a local viper instance and discards it after `v.Unmarshal(&cfg)`, binding pflags to it would require restructuring `loader.go` to return the viper instance or accept pflags as parameters. This is significant scope creep.

**Recommendation**: Implement `bindFlagsToConfig` as a post-load override on the `*Config` struct. This achieves the user's goal (flags override config file values) without restructuring the loader.

**Flags excluded from binding** (transient/per-invocation):
- `--config`: path to load config from; not a config key
- `--no-config`: operational flag; not a config key
- `--to`, `--from`, `--id`: command-specific flags (not on root)

---

## Complete File Change Summary

### Files to CREATE
| File | Purpose |
|------|---------|
| `/home/grue/dev/wherehouse/internal/config/writer.go` | WriteDefault, Set, Check, GetValue, atomicWrite (all write ops; no Unset) |
| `/home/grue/dev/wherehouse/internal/config/writer_test.go` | Programmatic tests for all writer.go functions |

### Files to MODIFY
| File | Changes |
|------|---------|
| `/home/grue/dev/wherehouse/internal/config/config.go` | Add `toml` struct tags to all fields in all sub-structs |
| `/home/grue/dev/wherehouse/cmd/config/helpers.go` | Remove: atomicWrite, marshalConfigWithComments, getConfigValue, setValueInMap, unsetValueInMap, loadConfigFile, determineConfigPath, configFilePerms, keyValueParts constants |
| `/home/grue/dev/wherehouse/cmd/config/init.go` | Replace write block with `config.WriteDefault`; remove filepath import |
| `/home/grue/dev/wherehouse/cmd/config/set.go` | Remove `updateConfigValue`; replace with `config.Set` call |
| `/home/grue/dev/wherehouse/cmd/config/check.go` | Replace `loadConfigFile` calls with `config.Check` calls |
| `/home/grue/dev/wherehouse/cmd/config/get.go` | Replace `getConfigValue` with `config.GetValue` |
| `/home/grue/dev/wherehouse/cmd/root.go` | Add `bindFlagsToConfig` call in `initConfig` |
| `/home/grue/dev/wherehouse/cmd/config/config.go` (or wherever subcommands are registered) | Remove `unset` subcommand registration |

### Files to DELETE
| File | Reason |
|------|--------|
| `/home/grue/dev/wherehouse/cmd/config/unset.go` | `config unset` command removed entirely |
| `/home/grue/dev/wherehouse/cmd/config/unset_test.go` | Tests for removed command |

### Files UNCHANGED
- `internal/config/loader.go`
- `internal/config/defaults.go`
- `internal/config/validation.go`
- `internal/config/database.go`
- `internal/config/log.go`
- `cmd/config/edit.go`
- `cmd/config/path.go`
- `cmd/config/init_test.go` (existing tests pass after Phase 1)

---

## Implementation Order and Dependencies

```
Phase 1 (toml tags)     ─┐
                          ├─> Phases 3-9 (all depend on toml tags for Check)
Phase 2 (writer.go)    ──┘

Phase 3 (init.go)      ─ depends on Phase 2 (WriteDefault)
Phase 4 (set.go)       ─ depends on Phase 2 (Set)
Phase 5 (DELETE unset) ─ independent (delete files, remove registration)
Phase 6 (check.go)     ─ depends on Phase 2 (Check)
Phase 7 (get.go)       ─ depends on Phase 2 (GetValue)
Phase 8 (helpers.go)   ─ depends on Phases 3-7 (cleanup after all callers updated)
Phase 9 (tests)        ─ depends on Phase 2 (writer.go exists)
Phase 10 (root.go)     ─ independent (flag binding, can be done any time)
```

**Recommended execution order**:
1. Phases 1 + 2 (sequential: tags first, then writer.go)
2. Phases 3-7 in parallel (all cmd refactors are independent once writer.go exists; Phase 5 is a deletion)
3. Phase 8 (cleanup - after 3-7)
4. Phases 9 + 10 in parallel (tests and flag binding are independent)

---

## Scope Boundaries (What This Does NOT Change)

- `internal/config/loader.go` - viper loading logic is correct, no changes
- `internal/config/defaults.go` - no changes
- `internal/config/validation.go` - no changes
- `cmd/config/edit.go` - unchanged (launches $EDITOR)
- `cmd/config/path.go` - unchanged (displays paths only)
- The `--sources` flag in `get.go` remains a TODO comment
- `user.os_username_map` is NOT settable via `config set` (map type)
- Config file format loses comments on `config init` (viper does not write comments). This is a deliberate trade-off per user feedback.
- `config unset` command does not exist in the refactored CLI. Users use `config set <key> <default-value>` to restore defaults.

---

## Risk Assessment

**Low risk (additive)**:
- Phase 1 (toml tags): adds tags, no behavior change for viper
- Phase 10 (flag binding): post-load override, no structural change

**Low risk (deletion)**:
- Phase 5 (delete unset): removes files and registration. No other code depends on `unset.go`.

**Low-medium risk (straightforward moves)**:
- Phases 3-7: call sites simplified, easy to verify with existing tests

**Medium risk (behavioral change)**:
- `WriteDefault` output: no longer has comments. Tests `TestInitCreatesValidToml` and `TestInitThenCheck` must be updated to not assert comment presence. Existing tests only check for file existence and non-empty content, so this is low impact in practice.
- `Set` via `viper.WriteConfigAs`: writes ALL settings (not just the changed key), which means after `config set database.path /foo`, the file will contain all default values for other keys that were not previously present. This is a behavior change from the current implementation which only modifies the targeted key. **This is acceptable** since viper reads all defaults.

**Items requiring careful testing**:
- `WriteDefault` via viper: verify `afero.MemMapFs` works with `v.fs.OpenFile()` (viper uses `O_CREATE|O_TRUNC|O_WRONLY` flags - must be supported by afero memory fs). This is a standard afero operation and should work.
- `Set` after `WriteDefault`: the second viper instance in `Set` reads the file written by the first. File must be fully flushed before read. `WriteConfigAs` calls `f.Sync()` which ensures this.
- Integer keys (`logging.max_size_mb`, `logging.max_backups`): after `Set`, viper will store as `int`. On subsequent `WriteConfigAs`, verify TOML writes as integer (not float). `go-toml/v2` preserves Go int type in `map[string]any`.
