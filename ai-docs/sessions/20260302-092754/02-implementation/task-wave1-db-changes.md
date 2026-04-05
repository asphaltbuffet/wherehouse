# Wave 1 Database Changes

## Files Modified

### Subtask A - System Location ID Constants
- `/home/grue/dev/wherehouse/internal/database/schema_metadata.go`
  - Changed `missingID` from `"00000000-0000-0000-0000-000000000001"` to `"sys0000001"`
  - Changed `borrowedID` from `"00000000-0000-0000-0000-000000000002"` to `"sys0000002"`
  - Changed `loanedID` from `"00000000-0000-0000-0000-000000000003"` to `"sys0000003"`
  - Updated comment from "Deterministic UUIDs" to "Deterministic IDs"

### Subtask B - Migration Files Created
- `/home/grue/dev/wherehouse/internal/database/migrations/000003_nanoid_migration.up.sql` (new)
  - Comment-only no-op migration: `SELECT 1; -- no-op version marker`
  - Explains that data transformation is done by `wherehouse migrate database` command
- `/home/grue/dev/wherehouse/internal/database/migrations/000003_nanoid_migration.down.sql` (new)
  - Comment-only no-op down migration: `SELECT 1; -- no-op`
  - Directs user to restore from backup for data rollback

### Subtask C - New Database Methods
- `/home/grue/dev/wherehouse/internal/database/location.go`
  - Added `GetAllLocations(ctx context.Context) ([]*Location, error)` - queries all rows from `locations_current` ordered by depth then display_name
- `/home/grue/dev/wherehouse/internal/database/item.go`
  - Added `GetAllItems(ctx context.Context) ([]*Item, error)` - queries all rows from `items_current` ordered by display_name
- Note: `ExecInTransaction` already existed in `database.go` - no change needed

### Test Files Updated (to accommodate new migration version 3)
- `/home/grue/dev/wherehouse/internal/database/migrations_test.go`
  - Updated version tracking assertion: `2` -> `3`
  - Updated dirty state detection to use version `3` instead of `2`
  - Updated rollback tests: 2 rollback calls -> 3 rollback calls (one per migration)
- `/home/grue/dev/wherehouse/internal/database/integration_test.go`
  - Updated migration version assertion: `uint(2)` -> `uint(3)`

## Build and Test Results
- `go build ./internal/database/...`: SUCCESS
- `go test ./internal/database/...`: ok (all tests pass)
