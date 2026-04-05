# Task D Changes: Delete `config unset` Command

## Files Deleted

- `/home/grue/dev/wherehouse/cmd/config/unset.go`
- `/home/grue/dev/wherehouse/cmd/config/unset_test.go`

## Files Modified

### `/home/grue/dev/wherehouse/cmd/config/config.go`

**Removed** subcommand registration:
```go
// Removed from GetConfigCmd():
configCmd.AddCommand(GetUnsetCmd())

// Removed from ResetForTesting():
unsetCmd = nil
```

**Result**: `GetConfigCmd()` now registers 6 subcommands (init, get, set, path, check, edit) instead of 7.

## Rationale

viper v1.21.0 does not expose a key-delete API. The `config unset` command cannot be implemented correctly. Users can restore defaults with `config set <key> <default-value>`.
