# Task H Changes: cmd/config/helpers.go Cleanup

## Files Modified

### `/home/grue/dev/wherehouse/cmd/config/helpers.go`
**Removed:**
- `configFilePerms` constant (moved to `internal/config/writer.go`)
- `keyValueParts` constant (was used only in helpers and set.go; set.go updated to literal `2`)
- `atomicWrite` function (moved to `internal/config/writer.go`)
- `marshalConfigWithComments` function (eliminated; viper WriteConfigAs replaces it)
- `getConfigValue` function (moved as `GetValue` to `internal/config/writer.go`)
- `setValueInMap` function (moved as `parseConfigValue` to `internal/config/writer.go`)
- `unsetValueInMap` function (eliminated; config unset command deleted)
- `loadConfigFile` function (moved as `Check` to `internal/config/writer.go`)
- `determineConfigPath` function (no remaining non-test callers)
- Imports: `errors`, `fmt`, `strings`, `github.com/pelletier/go-toml/v2`, `github.com/asphaltbuffet/wherehouse/internal/config`

**Kept:**
- `cmdFS` variable
- `SetFilesystem` function
- `fileExists` function
- `ensureDir` function

### `/home/grue/dev/wherehouse/cmd/config/edit.go`
- Line 116: Changed `loadConfigFile(cmdFS, expandedPath)` to `config.Check(cmdFS, expandedPath)`

### `/home/grue/dev/wherehouse/cmd/config/set.go`
- Line 36: Changed `cobra.ExactArgs(keyValueParts)` to `cobra.ExactArgs(2)`

### `/home/grue/dev/wherehouse/cmd/config/helpers_test.go`
- Removed all tests for deleted functions:
  - `TestAtomicWrite_*` (4 tests)
  - `TestMarshalConfigWithComments_*` (2 tests)
  - `TestGetConfigValue_*` (6 tests)
  - `TestSetValueInMap_*` (8 tests)
  - `TestUnsetValueInMap_*` (3 tests)
  - `TestLoadConfigFile_*` (2 tests)
  - `TestDetermineConfigPath_*` (4 tests)
- Kept tests for: `SetFilesystem`, `fileExists`, `ensureDir`

### `/home/grue/dev/wherehouse/cmd/config/config_test.go`
- Removed `TestGetUnsetCmd_Returns` and `TestGetUnsetCmd_Singleton` tests
- Updated `TestGetConfigCmd_HasSubcommands` count from 7 to 6
- Updated `TestGetConfigCmd_SubcommandNames` to remove `unset` check
- Updated `TestResetForTesting_ResetsAll` to remove `unsetCmd` references
- Updated `TestAllSubcommands_HaveRunE` to remove `GetUnsetCmd` entry

## Verification
- `go build ./cmd/config/...`: PASS
- `go test ./cmd/config/...`: PASS (all tests pass)
- `go build ./...`: PASS (no regressions)
