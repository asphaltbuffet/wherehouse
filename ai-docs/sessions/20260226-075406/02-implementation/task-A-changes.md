# Task A Changes

## Files Modified

### `/home/grue/dev/wherehouse/internal/cli/database.go`

- Added `"os"` to imports
- Added sentinel error `ErrDatabaseNotInitialized = errors.New("database not initialized")`
- Added `CheckDatabaseExists(dbPath string) error` function using `os.Stat`
- Updated `OpenDatabase` to call `CheckDatabaseExists` before `database.Open`
- Fixed govet shadow: changed `if err := CheckDatabaseExists(...)` to `if err = CheckDatabaseExists(...)` to reuse the outer `err` variable

### `/home/grue/dev/wherehouse/internal/cli/database_test.go`

- Added `"github.com/asphaltbuffet/wherehouse/internal/database"` import (needed to pre-create DB files)
- Updated existing `OpenDatabase` success tests to pre-create the database file via `database.Open` before calling `OpenDatabase` (required because `OpenDatabase` now enforces file existence)
  - `success with valid config`
  - `success with empty path uses default`
  - `success with nested directory creation`
  - `TestOpenDatabase_AutoMigration`
  - `TestOpenDatabase_ContextPropagation`
  - `TestOpenDatabase_MultipleCallsSeparate`
  - `TestOpenDatabase_ExistingDatabase` (was using `OpenDatabase` to create, now uses `database.Open` directly)
- Added new test case in `TestOpenDatabase`: `error when database file does not exist`
- Added new `TestCheckDatabaseExists` table-driven test with 3 cases:
  - File present returns nil
  - File absent, dir present returns `ErrDatabaseNotInitialized`
  - Dir absent returns `ErrDatabaseNotInitialized`
