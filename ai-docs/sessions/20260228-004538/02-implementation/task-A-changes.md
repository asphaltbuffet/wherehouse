# Task A Changes

## Files Modified

### `/home/grue/dev/wherehouse/internal/cli/selectors.go`

Added `LocationItemQuerier` interface before `ResolveLocation` function definition.

Updated the following function signatures from `db *database.Database` to `db LocationItemQuerier`:
- `ResolveLocation`
- `ResolveItemSelector`
- `resolveLocationItemSelector` (internal helper)
- `resolveItemByCanonicalName` (internal helper)
- `buildAmbiguousItemError` (internal helper)

## Files NOT Modified

- `internal/cli/selectors_test.go` — passes `*database.Database` which satisfies the interface implicitly
- `internal/database/` — no changes needed; existing methods already match the interface
