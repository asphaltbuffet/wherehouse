# Task A Changes

## Files Modified

### /home/grue/dev/wherehouse/internal/database/location.go

Added one new exported function `GetRootLocations` before the existing `GetLocationChildren` function (line ~272 in original).

Function added:
```go
// GetRootLocations retrieves all locations with no parent (top-level),
// ordered by display_name. Includes system locations (Missing, Borrowed).
func (d *Database) GetRootLocations(ctx context.Context) ([]*Location, error) {
    const query = `
        SELECT
            location_id,
            display_name,
            canonical_name,
            parent_id,
            full_path_display,
            full_path_canonical,
            depth,
            is_system,
            updated_at
        FROM locations_current
        WHERE parent_id IS NULL
        ORDER BY display_name
    `

    rows, err := d.db.QueryContext(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("failed to query root locations: %w", err)
    }
    defer rows.Close()

    return scanLocations(rows)
}
```

### /home/grue/dev/wherehouse/internal/database/location_test.go

Added 5 new test functions at the end of the file:

- `TestGetRootLocations_EmptyDatabase` — verifies all returned rows have nil parent_id on a fresh DB (system locations only)
- `TestGetRootLocations_ReturnsRootsAlphabetically` — verifies alphabetical ordering and that Workshop/Storage are present
- `TestGetRootLocations_ExcludesChildren` — verifies child locations (Toolbox, Workbench, Shelves, Bin A, Bin B) are not returned
- `TestGetRootLocations_IncludesSystemLocations` — verifies Missing and Borrowed system locations appear in results
- `TestGetRootLocations_AllFieldsPopulated` — verifies all Location struct fields are non-zero and that root path invariants hold (full_path_display == display_name, depth == 0)
