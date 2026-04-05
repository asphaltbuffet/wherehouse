# Wave 2 Core Implementation: internal/cli/migrate.go

**Date:** 2026-03-02
**Status:** Complete
**Tests:** 7/7 passing
**Build:** Clean

## File Created

`/home/grue/dev/wherehouse/internal/cli/migrate.go`

## Function Signature

```go
func MigrateDatabase(cmd *cobra.Command, db *database.Database, dryRun bool) error
```

## Implementation Summary

### Key Design Decisions

1. **Idempotency via looksLikeNanoid**: User locations/items that already have 10-char alphanumeric IDs are mapped to themselves (no DB update). System locations always get their fixed deterministic ID by canonical name lookup.

2. **System location IDs by canonical name**: The `systemLocationIDs` map keys on `canonical_name` (not the old UUID). This means `sys0000001` (9 chars, fails `looksLikeNanoid`) still gets correctly remapped to `sys0000001` on second run — idempotent because it's a system location.

3. **Skip-if-same optimization**: `applyMigration` skips `UPDATE` calls when `oldID == newID`, avoiding unnecessary DB writes on second migration run.

4. **Atomic transaction**: All DB updates (locations_current, items_current, events indexed columns, events payload) happen inside a single `ExecInTransaction` call. Failure at any point rolls back everything.

5. **Dry-run**: Builds the mapping and prints the report, then prints "Dry run complete" message. No DB writes.

## Helper Functions

- `buildMigrateMapping`: Queries all locations and items, constructs old→new mapping
- `looksLikeNanoid`: Returns true for exactly 10-char alphanumeric strings (idempotency check)
- `isNanoidChar`: A-Za-z0-9 character check
- `applyMigration`: Applies all DB updates within a transaction
- `rewriteEventPayloads`: String-replaces all old IDs in events.payload JSON
- `printMigrateReport`: Prints "Location ID mappings" and "Item ID mappings" with `->` arrow format

## Tests Passed

- `TestMigrateDatabase_DryRun_PrintsPreview` — "DRY RUN" and "complete" in output
- `TestMigrateDatabase_DryRun_NoDBChanges` — IDs unchanged after dry-run
- `TestMigrateDatabase_SystemLocations_GetDeterministicIDs` — sys0000001/2/3 assigned correctly
- `TestMigrateDatabase_UserLocations_GetNanoidIDs` — user locations get 10-char alphanumeric IDs
- `TestMigrateDatabase_Items_GetNanoidIDs` — items get 10-char alphanumeric IDs
- `TestMigrateDatabase_EventPayloads_UpdatedCorrectly` — UUID patterns removed from payloads
- `TestMigrateDatabase_Idempotency` — second migration run preserves all IDs
- `TestMigrateDatabase_PrintsMappingReport` — output contains "Location ID mappings", "Item ID mappings", "->"
