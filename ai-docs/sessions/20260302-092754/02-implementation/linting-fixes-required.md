# Linting Fixes Required for Wave 2 Completion

**Status**: BLOCKING - Tests pass, but linting must be fixed before merge

## Summary

Three linting errors must be fixed to complete Wave 2:

1. **Cognitive complexity** in `applyMigration()` - 41 > 20
2. **Variable shadowing** in `MigrateDatabase()` - err declared twice
3. **Unused parameter** in `GetDatabaseCmd()` - args parameter not used

---

## Fix #1: Reduce Cognitive Complexity (PRIORITY)

**File**: `internal/cli/migrate.go`, line 146
**Function**: `applyMigration()`
**Issue**: Cognitive complexity is 41 (limit: 20)
**Impact**: Function is too complex and difficult to test/maintain

### Current Location
Line 146 in `internal/cli/migrate.go`

### Solution
Break `applyMigration()` into smaller, focused helper functions:

**Option A** (Recommended): Extract into 3-4 helper functions:
```go
// applyMigration applies location and item ID migrations atomically
func applyMigration(ctx context.Context, tx *sql.Tx, mapping *migrateMapping) error {
    // Step 1: Migrate locations
    if err := migrateLocations(ctx, tx, mapping); err != nil {
        return err
    }

    // Step 2: Migrate items
    if err := migrateItems(ctx, tx, mapping); err != nil {
        return err
    }

    // Step 3: Migrate event payloads
    if err := migrateEventPayloads(ctx, tx, mapping); err != nil {
        return err
    }

    return nil
}

// migrateLocations updates all location IDs in the database
func migrateLocations(ctx context.Context, tx *sql.Tx, mapping *migrateMapping) error {
    // Extract current location migration logic here
    // (reduces complexity from ~13 to ~5)
}

// migrateItems updates all item IDs and parent references
func migrateItems(ctx context.Context, tx *sql.Tx, mapping *migrateMapping) error {
    // Extract current items migration logic here
}

// migrateEventPayloads updates ID references in event JSON payloads
func migrateEventPayloads(ctx context.Context, tx *sql.Tx, mapping *migrateMapping) error {
    // Extract current events migration logic here
}
```

### Verification
After refactoring, run:
```bash
mise run lint
```
Expected output should show gocognit issue resolved.

---

## Fix #2: Variable Shadowing

**File**: `internal/cli/migrate.go`, line 61
**Function**: `MigrateDatabase()`
**Issue**: Variable `err` shadows declaration at line 43

### Current Code
```go
// Line 43
mapping, err := buildMigrateMapping(ctx, db)  // First err declaration
if err != nil {
    return fmt.Errorf("failed to build ID mapping: %w", err)
}

// ...

// Line 61
if err := db.ExecInTransaction(ctx, func(tx *sql.Tx) error {  // Second err shadows first
    return applyMigration(ctx, tx, mapping)
}); err != nil {
    return fmt.Errorf("migration failed (no changes applied): %w", err)
}
```

### Solution
Rename the second error variable:

```go
// Line 43 - unchanged
mapping, err := buildMigrateMapping(ctx, db)
if err != nil {
    return fmt.Errorf("failed to build ID mapping: %w", err)
}

// ...

// Line 61 - rename to txErr or execErr
if txErr := db.ExecInTransaction(ctx, func(tx *sql.Tx) error {
    return applyMigration(ctx, tx, mapping)
}); txErr != nil {
    return fmt.Errorf("migration failed (no changes applied): %w", txErr)
}
```

---

## Fix #3: Unused Parameter

**File**: `cmd/migrate/database.go`, line 45
**Function**: `RunE` in `GetDatabaseCmd()`
**Issue**: Parameter `args` is not used

### Current Code
```go
RunE: func(cmd *cobra.Command, args []string) error {
    db, err := cli.OpenDatabase(cmd.Context())
    if err != nil {
        return fmt.Errorf("failed to open database: %w", err)
    }
    defer db.Close()

    return cli.MigrateDatabase(cmd, db, dryRun)
},
```

### Solution
Rename unused parameter to `_`:

```go
RunE: func(cmd *cobra.Command, _ []string) error {
    db, err := cli.OpenDatabase(cmd.Context())
    if err != nil {
        return fmt.Errorf("failed to open database: %w", err)
    }
    defer db.Close()

    return cli.MigrateDatabase(cmd, db, dryRun)
},
```

---

## Verification Steps

After applying all fixes:

1. Run linting:
   ```bash
   mise run lint
   ```
   Expected: 0 errors

2. Run tests to ensure no regression:
   ```bash
   go test ./internal/cli/... -v
   go test ./cmd/migrate/... -v
   ```
   Expected: All tests pass

3. Run full build:
   ```bash
   go build ./...
   ```
   Expected: Success

---

## Notes

- All three fixes are straightforward and require no logic changes
- Tests should all continue to pass after fixes
- The refactoring of `applyMigration()` will improve code maintainability and testability
