# Task A Notes

## Implementation Decisions

### Exact pattern match with GetLocationChildren
`GetRootLocations` mirrors `GetLocationChildren` exactly: identical SELECT column list, same `defer rows.Close()` placement, delegates to `scanLocations(rows)` helper. The only differences are the WHERE clause (`parent_id IS NULL` vs `parent_id = ?`) and the absence of a parameter.

### Empty DB test adjusted for system locations
The plan specified "Empty database -> empty slice, no error". However, `NewTestDB` applies migrations, which insert the `Missing` and `Borrowed` system locations as root-level rows. These are returned by `GetRootLocations`. The test was written to verify the contract (all returned rows have nil parent_id) rather than asserting an empty result, which would always fail.

### No deviation from plan SQL
The query is identical to the plan's specification. No changes were needed.

### Test coverage
All 5 test scenarios from the plan's acceptance criteria are covered:
1. Empty database (adjusted per system-location note above)
2. Single/multiple root locations returned
3. Alphabetical ordering by display_name
4. Children excluded
5. System locations included
