# Task B: Decisions and Deviations

## Decisions

### 1. JSON type name deduplication
The plan specified `ListItemJSON`, `ListLocationJSON`, `ListOutputJSON`. The `revive` linter flags these as stuttering (`list.ListItemJSON` → use `list.ItemJSON`). Renamed to `ItemJSON`, `LocationJSON`, `OutputJSON`. JSON field names (`json:"..."` tags) are unchanged so the wire format is identical.

### 2. `ResolveLocation` returns UUID string, not `*database.Location`
The plan noted this ambiguity. `cli.ResolveLocation` returns `(string, error)`. In `buildNodes`, after resolving the ID string, we call `db.GetLocation(ctx, locationID)` to get the full `*database.Location`. If that second call fails (extremely unlikely after a successful resolve), we render as not-found rather than error out.

### 3. `buildLocationNodeRecursive` - Items/Children may be nil for empty locations
The DB's `scanItems`/`scanLocations` returns `nil` slice (not empty slice) when no rows match. The `populateTree` function and `nodeToJSON` handle nil slices correctly (range over nil is a no-op). The test `TestBuildLocationNodeRecursive_ItemsNotNil` was renamed to `TestBuildLocationNodeRecursive_EmptyLocation` to test the actual behavior (zero length, may be nil).

### 4. `populateTree` — recursive mode detection
The plan used `child.Items != nil || child.Children != nil` to detect recursive mode. This is preserved exactly. In flat mode both are nil so the condition is false and we use `ChildItemCount`/`ChildLocationCount`. In recursive mode at least one of Items/Children is non-nil (even if empty, as the slice is assigned from a DB call result).

### 5. No `--quiet` flag
As specified: the list command has only `--recurse`/`-r` and the global `--json` flag. No quiet mode.

### 6. Not-found locations: exit 0
`runList` never returns a non-nil error for resolution failures. `buildNodes` creates `LocationNode{NotFound: true}` entries and continues. Only DB-level errors (failed to open, failed to query) propagate as errors.

### 7. `treeprint.New()` root node rendering
`treeprint.New()` creates an unnamed root. We call `root.SetValue(...)` to set the root display line. The `root.String()` output includes the root value line followed by child lines. This matches the expected format exactly.

## No Deviations from Final Plan
All design decisions from `final-plan.md` were followed as specified.
