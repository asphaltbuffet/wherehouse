# Architecture Plan: Replace UUID with nanoid

## Overview

Replace all use of `github.com/google/uuid` with 10-character nanoid strings throughout the wherehouse codebase, migrate existing database records, and document user-facing steps.

---

## Current State Analysis

### UUID Generation Points

Two files generate UUIDs at runtime:

- `cmd/add/location.go:97` — `uuid.NewV7()` then `.String()` to produce `location_id`
- `cmd/add/item.go:88` — `uuid.NewV7()` then `.String()` to produce `item_id`

### UUID Parsing / Detection

- `internal/cli/selectors.go:59-67` — `LooksLikeUUID()` uses `uuid.Parse()` and checks `len == 36`
- All selector resolution in `internal/cli/selectors.go` and downstream callers (`cmd/add/helpers.go`, `cmd/list/helpers.go`, `cmd/move/helpers.go`, `cmd/lost/helpers.go`, `cmd/loan/helpers.go`) rely on `LooksLikeUUID` to determine if user input is a direct ID or a name.

### Hardcoded UUID Values

- `internal/database/schema_metadata.go` — three system location IDs hardcoded as deterministic UUID strings:
  - `missingID  = "00000000-0000-0000-0000-000000000001"`
  - `borrowedID = "00000000-0000-0000-0000-000000000002"`
  - `loanedID   = "00000000-0000-0000-0000-000000000003"`
- `internal/database/migrations/000002_add_loaned_system_location.up.sql` — hardcodes `'00000000-0000-0000-0000-000000000003'`
- `internal/database/migrations/000002_add_loaned_system_location.down.sql` — hardcodes same

### Test Constants

- `internal/database/helper_test.go` — 14 constants are UUID-format strings used as fixed IDs for reproducible tests (e.g., `TestLocationWorkshop = "01936e3e-1000-7890-abcd-ef0123456789"`).
- Multiple test files generate `uuid.New().String()` for ephemeral test IDs.

### History Display

- `cmd/history/output.go:19` — `uuidPrefixLength = 8` used in fallback display when a location cannot be resolved. Takes `locationID[:8]`. With 10-char nanoids this constant is still reasonable (show full ID or first 8 chars).

### Database Schema

All ID columns are `TEXT PRIMARY KEY` or `TEXT` foreign key references in SQLite. The schema itself is ID-format agnostic — no UUID-specific CHECK constraints exist. Column names remain valid (`location_id`, `item_id`, `project_id`). Only the stored string values change.

### Migration Infrastructure

- `golang-migrate/migrate/v4` with embedded SQL files in `internal/database/migrations/`
- Migrations auto-run on `Open()` when `AutoMigrate=true`
- Current schema version: 2

---

## Chosen nanoid Package

**`github.com/matoous/go-nanoid/v2`** — the canonical Go nanoid implementation. API:

```go
import gonanoid "github.com/matoous/go-nanoid/v2"

id, err := gonanoid.New(10)  // 10-character ID using default alphabet
```

Default alphabet: `A-Za-z0-9_-` (64 characters, URL-safe). At 10 characters: 64^10 ≈ 1.15 × 10^18 unique IDs.

Alternative: `github.com/jaevor/go-nanoid` — faster, crypto-secure, but less used. Either works; `matoous/go-nanoid/v2` is preferred for its widespread adoption and idiomatic API.

---

## Architecture: New `internal/nanoid` Package

Create a thin wrapper package for ID generation to centralize the dependency and allow easy future replacement:

**`internal/nanoid/nanoid.go`**

```go
package nanoid

import gonanoid "github.com/matoous/go-nanoid/v2"

// IDLength is the standard length for all entity IDs in wherehouse.
const IDLength = 10

// New generates a new random ID of IDLength characters.
// Uses the nanoid default alphabet (A-Za-z0-9_-).
// Returns an error only if the system entropy source fails.
func New() (string, error) {
    return gonanoid.New(IDLength)
}

// MustNew generates a new random ID, panicking if entropy fails.
// Suitable for use in test code or initialization where failure is unrecoverable.
func MustNew() string {
    id, err := New()
    if err != nil {
        panic("nanoid: failed to generate ID: " + err.Error())
    }
    return id
}
```

This gives a single import path (`github.com/asphaltbuffet/wherehouse/internal/nanoid`) everywhere IDs are generated.

---

## Changes Required

### 1. `go.mod` / `go.sum`

- Add: `github.com/matoous/go-nanoid/v2`
- Remove: `github.com/google/uuid`

Run:
```
go get github.com/matoous/go-nanoid/v2
go mod tidy
```

### 2. New Package: `internal/nanoid/nanoid.go`

Create as described above.

### 3. `cmd/add/location.go`

Replace:
```go
import "github.com/google/uuid"
// ...
locationUUID, uuidErr := uuid.NewV7()
if uuidErr != nil {
    return fmt.Errorf("failed to generate UUID for location %q: %w", locationName, uuidErr)
}
locationID := locationUUID.String()
```

With:
```go
import "github.com/asphaltbuffet/wherehouse/internal/nanoid"
// ...
locationID, nanoErr := nanoid.New()
if nanoErr != nil {
    return fmt.Errorf("failed to generate ID for location %q: %w", locationName, nanoErr)
}
```

Also update doc string: "Each location receives a unique UUID" -> "Each location receives a unique ID".

### 4. `cmd/add/item.go`

Same pattern as location.go:
```go
import "github.com/asphaltbuffet/wherehouse/internal/nanoid"
// ...
itemID, nanoErr := nanoid.New()
if nanoErr != nil {
    return fmt.Errorf("failed to generate ID: %w", nanoErr)
}
```

Update doc string accordingly.

### 5. `internal/cli/selectors.go`

The `LooksLikeUUID` function must be replaced with `LooksLikeID` (or renamed) that detects 10-character nanoid strings:

```go
// Remove: import "github.com/google/uuid"

// LooksLikeID checks if a string looks like a nanoid-format ID.
// Returns true if the string is exactly IDLength characters of the nanoid alphabet.
func LooksLikeID(s string) bool {
    if len(s) != nanoid.IDLength {
        return false
    }
    for _, c := range s {
        if !isNanoidChar(c) {
            return false
        }
    }
    return true
}

// isNanoidChar returns true if c is in the nanoid default alphabet (A-Za-z0-9_-).
func isNanoidChar(c rune) bool {
    return (c >= 'A' && c <= 'Z') ||
        (c >= 'a' && c <= 'z') ||
        (c >= '0' && c <= '9') ||
        c == '_' || c == '-'
}
```

The function `ResolveLocation` and `ResolveItemSelector` call `LooksLikeUUID` internally — rename all call sites to `LooksLikeID`.

**Note:** The exported name `LooksLikeUUID` is referenced in `cmd/move/helpers_test.go:54` as `cli.LooksLikeUUID`. Rename it to `cli.LooksLikeID` and update all call sites. No external consumers exist since this is an internal package.

### 6. `internal/database/schema_metadata.go`

The three system location ID constants must change from UUID format to fixed 10-character nanoid strings. These must be deterministic and stable across all databases:

```go
const (
    missingID  = "sys_missing"  // 11 chars — PROBLEM: must be exactly 10
    // Alternative: use fixed 10-char strings in nanoid alphabet
    missingID  = "SYSM1ss1ng"  // Not ideal
)
```

**Better approach:** Use fixed, human-recognizable 10-character strings:

```go
const (
    missingID  = "sys0000001"  // 10 chars, nanoid alphabet compatible
    borrowedID = "sys0000002"
    loanedID   = "sys0000003"
)
```

These satisfy the 10-character length and use only alphanumeric characters from the nanoid alphabet. They are clearly artificial and won't collide with randomly generated IDs.

### 7. SQL Migration Files

#### New migration: `000003_nanoid_migration.up.sql`

This migration must:
1. Rename all existing UUID-format IDs to new nanoid IDs
2. Update all foreign key references consistently
3. Update hardcoded system location IDs
4. Update all event payload JSON strings that contain UUID references

```sql
-- Migration 000003: Replace UUID IDs with nanoid format
-- This migration remaps all entity IDs from UUID format to nanoid format.
-- System locations get fixed deterministic IDs; user entities get new random IDs.

-- Step 1: Create a mapping table for old UUID -> new nanoid
-- (populated by Go migration code, not pure SQL)
```

**Critical insight:** This migration cannot be a pure SQL migration because:
- New nanoid values must be generated randomly by the Go runtime
- JSON payloads in the `events` table contain embedded UUID strings that must be updated
- A pure SQL migration cannot call the nanoid generator

**Solution: Go-assisted migration via a new CLI subcommand**

Add a `wherehouse db migrate-ids` command (or integrate into the existing migrate flow) that:
1. Opens the database
2. For each item in `items_current`, generates a new nanoid, stores old->new mapping
3. For each location in `locations_current` (non-system), generates a new nanoid
4. Updates system location IDs to the fixed constants (`sys0000001`, etc.)
5. Updates all foreign key references in both projection tables and the `events` table (including JSON payload strings)
6. Runs as a single transaction for atomicity

**SQL migration file registers the version but delegates actual data work to Go:**

`000003_nanoid_migration.up.sql`:
```sql
-- Migration 000003: ID format migration to nanoid
-- Actual data transformation performed by Go code via 'wherehouse db migrate-ids'
-- This file serves as a schema version marker only.
-- No schema changes required (all ID columns are TEXT, format-agnostic).
SELECT 1; -- no-op, version bump only
```

Actually, a cleaner approach: migrate the schema version in pure SQL (no-op), and the Go migration routine updates data. The `Open()` auto-migrate will bump the schema version, and a separate one-time command performs ID rewriting.

#### `000002_add_loaned_system_location.up.sql` (historical)

The hardcoded UUID `'00000000-0000-0000-0000-000000000003'` was correct for migration 002. Migration 003 will remap it. Do not edit historical migrations — that would break checksum validation in golang-migrate.

### 8. Go ID Migration Command

Add `cmd/db/migrate_ids.go` providing a `wherehouse db migrate-ids` subcommand:

```
wherehouse db migrate-ids [--dry-run] [--backup PATH]
```

Logic:
1. Open database (reads only existing data, no AutoMigrate re-run during migration)
2. Begin transaction
3. Query all `locations_current` WHERE `is_system = 0` — generate nanoid mapping table in memory
4. Query all `items_current` — generate nanoid mapping
5. Update system location IDs from old UUIDs to fixed constants
6. Apply updates to `locations_current`: location_id, parent_id
7. Apply updates to `items_current`: item_id, location_id, temp_origin_location_id
8. Apply updates to `events` table: item_id, location_id columns (indexed columns)
9. Update JSON payloads in `events.payload` — string-replace old IDs with new IDs using SQLite's `replace()` function or Go-side JSON manipulation
10. Commit transaction
11. Print mapping report (old ID -> new ID) for user reference

**Backup**: Always recommend (and optionally enforce) a database backup before running.

### 9. `cmd/history/output.go`

The `uuidPrefixLength = 8` constant is used only in the fallback case where a location cannot be resolved. With 10-character nanoids:
- Option A: Keep `8`, show first 8 chars of the 10-char ID (still meaningful)
- Option B: Change to `nanoidPrefixLength = 10` and show the full ID (better since IDs are short)

Recommended: rename constant to `idDisplayLength` and set to `10` (show full ID since 10 chars is compact). Update the fallback format string accordingly.

### 10. Test Constants (`internal/database/helper_test.go`)

Replace all 14 UUID-format constants with fixed 10-character nanoid-compatible strings:

```go
const (
    TestLocationWorkshop  = "tst_ws0001"  // 10 chars
    TestLocationStorage   = "tst_st0002"
    TestLocationToolbox   = "tst_tb0003"
    TestLocationWorkbench = "tst_wb0004"
    TestLocationShelves   = "tst_sh0005"
    TestLocationBinA      = "tst_ba0006"
    TestLocationBinB      = "tst_bb0007"
    TestItem10mmSocket    = "itm_so0001"
    TestItemScrewdriverSet = "itm_sd0002"
    TestItemHammer        = "itm_hm0003"
    TestItemDrillBits     = "itm_db0004"
    TestItemSandpaper     = "itm_sp0005"
    TestItemMissingWrench = "itm_mw0006"
    TestItemBorrowedSaw   = "itm_bs0007"
)
```

All use only alphanumeric and underscore characters (nanoid alphabet). Exactly 10 characters. Update comments from "UUID" to "ID".

### 11. Test Files Using `uuid.New().String()`

Files that call `uuid.New().String()` to generate random test IDs:
- `cmd/lost/item_test.go`
- `cmd/list/list_test.go`
- `cmd/move/item_test.go`
- `internal/cli/selectors_test.go`

Replace with `nanoid.MustNew()` from `internal/nanoid`:

```go
import "github.com/asphaltbuffet/wherehouse/internal/nanoid"

// Before:
garageID := uuid.New().String()

// After:
garageID := nanoid.MustNew()
```

### 12. `cmd/move/helpers_test.go` — `TestLooksLikeUUID`

The test is titled `TestLooksLikeUUID` and tests `cli.LooksLikeUUID`. After rename:
- Rename test function to `TestLooksLikeID`
- Update test cases: valid inputs are now 10-char nanoid strings; invalid inputs are UUIDs or wrong-length strings
- Update call to `cli.LooksLikeID`

Similarly update `internal/cli/selectors_test.go:TestLooksLikeUUID`.

### 13. User-Facing Documentation Updates

Documentation strings mentioning "UUID" appear in:
- `cmd/lost/doc.go:8` — "UUID: Exact ID match"
- `cmd/found/found.go:30`, `cmd/found/doc.go:12` — "UUID: 550e8400-..."
- `cmd/loan/loan.go:24`, `cmd/move/move.go:12`, `cmd/move/doc.go:10`
- `cmd/history/history.go:30`, `cmd/history/history.go:41`
- `cmd/list/helpers.go:18-20`
- `cmd/add/helpers.go:15-17`
- Multiple helper file comments

Replace "UUID" with "ID" and example UUID strings with example nanoid strings (e.g., `aB3_xK9mPq`).

---

## Implementation Sequence

1. Add `github.com/matoous/go-nanoid/v2` to go.mod
2. Create `internal/nanoid/nanoid.go`
3. Replace generation in `cmd/add/location.go` and `cmd/add/item.go`
4. Rename `LooksLikeUUID` -> `LooksLikeID` in `internal/cli/selectors.go` and update detection logic
5. Update all callers of `LooksLikeUUID` (move helpers_test, selectors_test)
6. Update system location constants in `internal/database/schema_metadata.go`
7. Create `internal/database/migrations/000003_nanoid_migration.up.sql` (version bump no-op)
8. Create `internal/database/migrations/000003_nanoid_migration.down.sql`
9. Create `cmd/db/migrate_ids.go` — the ID migration command
10. Update test constants in `internal/database/helper_test.go`
11. Update test files using `uuid.New().String()` -> `nanoid.MustNew()`
12. Update `cmd/history/output.go` constant
13. Update all doc strings and comments
14. Run `go mod tidy` to remove uuid package
15. Write user-facing migration documentation

---

## Key Design Decisions

### 1. Thin internal/nanoid wrapper

Wrapping the external package prevents spreading the import throughout the codebase and makes future ID scheme changes a single-file edit.

### 2. Fixed deterministic system location IDs

System locations (`Missing`, `Borrowed`, `Loaned`) need stable IDs because migration 002 hardcodes a UUID. The migration 003 will rewrite these to `sys0000001`/`sys0000002`/`sys0000003`. These fixed values are outside the random generation space in practice (collision probability is negligible with 64^10 space).

### 3. Go-side ID migration command rather than pure SQL

The `events.payload` JSON column stores entity IDs as embedded strings. SQLite's `replace()` can do string substitution but requires knowing all old->new mappings, which must come from Go. A Go command that builds the mapping table in memory then applies it atomically is simpler, safer, and testable.

### 4. LooksLikeID replaces LooksLikeUUID

UUID detection (length 36 + uuid.Parse) is replaced by a simple alphabet+length check for 10-character nanoid strings. This is intentionally exact: if an ID is 10 chars of the nanoid alphabet, treat it as a direct ID lookup. Names with 10 characters that happen to match the alphabet will be tried as IDs first, then fall through to canonical name lookup — the same behavior as before.

### 5. No schema changes

All ID columns are `TEXT`. The database schema does not enforce UUID format. Migration 003 is a data-only migration, no DDL changes required.

---

## Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| Events payload JSON not updated | High | Go migration command explicitly updates payload strings |
| System location ID mismatch after migration | High | Fixed deterministic IDs with migration 003 rewriting them |
| Test constants wrong length | Medium | Replace all 14 constants with 10-char strings |
| `LooksLikeID` false positive on names | Low | Same risk as before with UUIDs; canonical name fallback handles it |
| User has cached UUID IDs in scripts | Low | Document migration; old IDs no longer valid after migration |
| Database backup failure | Medium | Make `--backup` default or required in migrate-ids command |

---

## Files to Create/Modify

### New Files
- `internal/nanoid/nanoid.go`
- `internal/database/migrations/000003_nanoid_migration.up.sql`
- `internal/database/migrations/000003_nanoid_migration.down.sql`
- `cmd/db/doc.go`
- `cmd/db/migrate_ids.go`
- `docs/migration-nanoid.md` (user-facing migration guide)

### Modified Files
- `go.mod` / `go.sum`
- `cmd/add/location.go`
- `cmd/add/item.go`
- `internal/cli/selectors.go`
- `internal/database/schema_metadata.go`
- `cmd/history/output.go`
- `internal/database/helper_test.go`
- `cmd/lost/item_test.go`
- `cmd/list/list_test.go`
- `cmd/move/item_test.go`
- `cmd/move/helpers_test.go`
- `internal/cli/selectors_test.go`
- Various doc.go and command long-description strings (12+ files)
