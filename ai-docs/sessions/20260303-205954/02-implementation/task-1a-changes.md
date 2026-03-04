# Task 1a Changes: cli.AddLocations

## Files Created

### `/home/grue/dev/wherehouse/internal/cli/locations.go`

New file containing:
- `AddLocationResult` struct with `LocationID`, `DisplayName`, `FullPathDisplay` fields
- `addLocationsDB` interface (embeds `LocationItemQuerier`; adds `ValidateLocationExists`, `ValidateUniqueLocationName`, `AppendEvent`)
- `AddLocations(ctx, names, parentName)` — public entry point, opens DB via `OpenDatabase`, delegates to `addLocations`
- `addLocations(ctx, db, names, parentName)` — injectable implementation for testability

Logic mirrors the original `cmd/add/location.go` inline body exactly:
1. Resolve optional parent via `ResolveLocation` + `ValidateLocationExists`
2. Per-name: validate no colon, canonicalize, check uniqueness, generate nanoid, `AppendEvent`
3. Best-effort `GetLocation` for `FullPathDisplay` after creation (non-fatal on failure)
4. Fail-fast on first error across all names

## Files Modified

### `/home/grue/dev/wherehouse/cmd/add/location.go`

Replaced `runAddLocation` body (~86 lines of inline logic) with a thin wrapper (~15 lines):
1. Reads `--in` flag
2. Calls `cli.AddLocations(ctx, args, parentInput)`
3. Iterates results, calls `out.Success(...)` with path or ID depending on whether `FullPathDisplay` is set
4. Removed inline domain logic
5. Removed unused imports: `database`, `nanoid`

## Lint Notes

Two lint issues exist post-change:
1. `cmd/add/helpers.go`: `openDatabase` and `resolveLocation` are now unused (they were only called by `location.go`). These will be deleted in Step 2 by `golang-ui-developer`. This is expected and correct.
2. `internal/cli/output.go:137`: godoclint issue ("io.Writer" should be "[io.Writer]") — pre-existing, not caused by this task.
