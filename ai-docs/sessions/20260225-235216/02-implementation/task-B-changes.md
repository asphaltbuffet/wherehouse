# Task B Changes: Refactor cmd/config/init.go to use config.WriteDefault

## File Modified

`/home/grue/dev/wherehouse/cmd/config/init.go`

## Changes Made

### Import update
- Removed `"path/filepath"` (no longer needed)
- Added `"strings"` (for `strings.Contains` in error detection)

### Replaced block in `runInit`

**Removed** (~20 lines):
```go
// Check if file exists
exists, err := fileExists(cmdFS, expandedPath)
if err != nil {
    out.Error(fmt.Sprintf("checking config file: %v", err))
    return fmt.Errorf("checking config file: %w", err)
}

if exists && !force {
    out.Error(fmt.Sprintf("configuration file already exists: %s", expandedPath))
    out.Info("Use --force to overwrite")
    return fmt.Errorf("configuration file already exists: %s", expandedPath)
}

// Create parent directory if needed
dir := filepath.Dir(expandedPath)
err = ensureDir(cmdFS, dir)
if err != nil {
    out.Error(fmt.Sprintf("failed to create directory %q: %v", dir, err))
    return fmt.Errorf("failed to create directory %q: %w", dir, err)
}

// Generate default config
cfg := config.GetDefaults()

// Marshal to TOML with comments
data := marshalConfigWithComments(cfg)

// Write atomically (write to temp file, then rename)
err = atomicWrite(cmdFS, expandedPath, data, configFilePerms)
if err != nil {
    out.Error(fmt.Sprintf("failed to write configuration: %v", err))
    return fmt.Errorf("failed to write configuration: %w", err)
}
```

**Added** (~12 lines):
```go
// Write default config (handles exists check, dir creation, and write atomically)
err = config.WriteDefault(cmdFS, expandedPath, force)
if err != nil {
    if strings.Contains(err.Error(), "already exists") {
        out.Error(err.Error())
        out.Info("Use --force to overwrite")
    } else {
        out.Error(fmt.Sprintf("failed to write configuration: %v", err))
    }
    return err
}
```

## Test Compatibility

The existing tests in `init_test.go` remain compatible:
- `TestConfigInit_FailsWhenFileExists`: checks `result.Error()` contains "already exists" - WriteDefault returns `"configuration file already exists: <path>"` which satisfies this
- `TestConfigInit_OverwritesWithForce`: WriteDefault with force=true overwrites correctly
- `TestConfigInit_CreatesGlobalConfig`: WriteDefault creates the file with all defaults
- `TestConfigInit_CreatesParentDirectories`: WriteDefault calls `fs.MkdirAll` internally
