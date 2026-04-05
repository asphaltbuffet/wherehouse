# Task H Notes

## Caller Analysis Before Removal

Before removing each function, callers in `cmd/config/` (excluding helpers.go and test files) were verified:

| Function | Production Callers | Action |
|---|---|---|
| `atomicWrite` | none | Removed |
| `marshalConfigWithComments` | none | Removed |
| `getConfigValue` | none | Removed |
| `setValueInMap` | none | Removed |
| `unsetValueInMap` | none | Removed |
| `loadConfigFile` | `edit.go:116` | Updated `edit.go` to call `config.Check` instead |
| `determineConfigPath` | none | Removed |
| `configFilePerms` | `atomicWrite` only (being removed) | Removed |
| `keyValueParts` | `set.go:36` | Updated `set.go` to use literal `2` |

## Unexpected Discovery

`config_test.go` contained references to `GetUnsetCmd` and `unsetCmd` that were not cleaned up when the unset command was previously deleted. These were removed as part of this task since the plan states the `config unset` command was eliminated.

## Import Cleanup

After removing the functions, the following imports became unused in `helpers.go` and were removed:
- `errors` (used only by `determineConfigPath`)
- `fmt` (used only by `atomicWrite`, `marshalConfigWithComments`)
- `strings` (used only by `getConfigValue`, `setValueInMap`, `unsetValueInMap`)
- `github.com/pelletier/go-toml/v2` (used only by `loadConfigFile`)
- `github.com/asphaltbuffet/wherehouse/internal/config` (used by multiple removed functions)
