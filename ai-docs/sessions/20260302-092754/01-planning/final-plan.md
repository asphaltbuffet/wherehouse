# Final Implementation Plan: UUID to nanoid Migration (TDD-Structured)

**Date:** 2026-03-02
**Session:** 20260302-092754
**Status:** Ready for implementation
**TDD:** Strict — no production code without a failing test first

---

## Summary of Clarifications Applied

| Topic | Decision |
|-------|----------|
| CLI command | `wherehouse migrate database` (verb-noun, new cobra command) |
| Alphabet | Alphanumeric only: `A-Za-z0-9` (62 chars, no underscores or hyphens) |
| System location IDs | `sys0000001` (Missing), `sys0000002` (Borrowed), `sys0000003` (Loaned) |
| Migration mode | Opt-in, atomic single transaction |
| History fallback | Show full 10-char ID (remove `uuidPrefixLength` constant) |
| Documentation | Committed markdown file + terminal output when migration runs |
| Cmd pattern | Thin-cmd: `cmd/migrate/` is a thin wrapper; all logic in `internal/cli/migrate.go` |
| TDD discipline | Iron Law: tests written and verified failing BEFORE any production code |

---

## Iron Law of TDD

**NO PRODUCTION CODE WITHOUT A FAILING TEST FIRST.**

For every component:
1. golang-tester writes failing tests
2. Tests are verified to fail for the right reason (missing feature, not compilation error from typos)
3. Specialist agent writes MINIMAL code to make tests pass
4. golang-tester verifies tests are green
5. Refactor if needed, keeping tests green

---

## Project-Wide Testing Rule

**ALL test code MUST use testify (assert/require). The standard library functions `t.Fatal`, `t.Fatalf`, `t.Error`, and `t.Errorf` are forbidden in test files.**

- Use `require.NoError(t, err)`, `require.Equal(t, expected, actual)`, etc. for assertions where failure should stop the test immediately (replaces `t.Fatal` / `t.Fatalf`).
- Use `assert.NoError(t, err)`, `assert.Equal(t, expected, actual)`, etc. for assertions where the test should continue after failure (replaces `t.Error` / `t.Errorf`).
- All test files must import `github.com/stretchr/testify/assert` and/or `github.com/stretchr/testify/require` as needed.
- **Testify is already present in go.mod** (`github.com/stretchr/testify v1.11.1`) — no additional dependency changes are required.

The golang-tester agent must apply this rule to every test file it creates or modifies. Code reviewers must reject any test code containing `t.Fatal`, `t.Fatalf`, `t.Error`, or `t.Errorf`.

---

## Thin-Cmd Pattern

The project uses thin cobra commands that delegate to `internal/cli/` for business logic.
Key examples observed in the codebase:

- `internal/cli/database.go` provides `OpenDatabase(ctx)`, `CheckDatabaseExists(path)` — shared infrastructure used by multiple commands
- `cmd/` packages call `cli.OpenDatabase(ctx)`, `cli.GetActorUserID(ctx)`, `cli.MustGetConfig(ctx)`, `cli.NewOutputWriterFromConfig(...)` then do minimal orchestration

For the migrate command the pattern is:

```
cmd/migrate/database.go  (thin)
  - defines cobra.Command struct, flags, long/short help
  - RunE: opens DB via cli.OpenDatabase, calls cli.MigrateDatabase(cmd, db, dryRun)
  - no migration logic

internal/cli/migrate.go  (all business logic)
  - MigrateDatabase(ctx, db, dryRun) error
  - buildIDMapping, applyIDMigration, rewriteEventPayloads, printMappingReport, etc.
```

---

## Dependency Graph

```
[A] go.mod + internal/nanoid package
         |
         +---> [B] cmd/add/*.go (generation)
         +---> [C] internal/cli/selectors.go (detection)
         +---> [D] test files using nanoid.MustNew()
         |
[E] internal/database/schema_metadata.go (system IDs)
         |
         +---> [F] cmd/migrate/ + internal/cli/migrate.go (migration command)
         |
[G] internal/database/migrations/000003_*.sql
         |
         +---> [F] cmd/migrate/ + internal/cli/migrate.go (migration command)
         |
[H] internal/database/helper_test.go (test constants)
         |
         +---> [D] test files (reference helper_test constants)
         |
[I] cmd/history/output.go (uuidPrefixLength removal)
[J] doc strings in cmd/**/*.go (UUID -> ID text)
[K] docs/migration-nanoid.md (user documentation)
```

---

## TDD Execution Waves

Each wave follows: **[test batch] → [implementation batch] → [verify batch]**

Tests are written to fail against the CURRENT codebase (before implementation). Implementation makes tests pass. Verification confirms green.

---

## Wave 1-test: Failing Tests for Foundation Components

**Agent:** golang-tester
**Runs before:** any Wave 1 implementation
**These tests must compile and FAIL (not error) before Wave 1-impl begins.**

**Reminder:** All test code must use testify (assert/require). Do not use t.Fatal, t.Fatalf, t.Error, or t.Errorf.

### 1a. internal/nanoid package tests (new file: `internal/nanoid/nanoid_test.go`)

The package does not exist yet. The test file is written first; it will fail to compile until the package exists. That compile failure IS the failing test state.

Test cases to cover:
- `TestNew_ReturnsIDOfCorrectLength` — `New()` returns string of length `IDLength` (10)
- `TestNew_ReturnsAlphanumericOnly` — every character is in `A-Za-z0-9`; no underscores, hyphens, or other symbols
- `TestNew_NoDuplicates` — generate 1000 IDs; verify no duplicates (collision sanity check)
- `TestNew_ReturnsError` — (mock or build-tag-based test documenting that errors propagate; acceptable to skip if no mock path exists)
- `TestMustNew_ReturnsIDOfCorrectLength` — `MustNew()` returns string of length 10
- `TestMustNew_ReturnsAlphanumericOnly` — same alphabet check as above
- `TestIDLength_IsCorrectValue` — `IDLength == 10`
- `TestAlphabet_Contains62Chars` — if `alphabet` constant is exported or testable, verify len == 62; otherwise test via output distribution

```go
// internal/nanoid/nanoid_test.go
package nanoid_test

import (
    "testing"
    "unicode"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/asphaltbuffet/wherehouse/internal/nanoid"
)

func TestIDLength_IsCorrectValue(t *testing.T) {
    assert.Equal(t, 10, nanoid.IDLength, "IDLength should be 10")
}

func TestNew_ReturnsIDOfCorrectLength(t *testing.T) {
    id, err := nanoid.New()
    require.NoError(t, err, "New() should not return an error")
    assert.Equal(t, nanoid.IDLength, len(id), "New() should return an ID of IDLength characters")
}

func TestNew_ReturnsAlphanumericOnly(t *testing.T) {
    for i := 0; i < 100; i++ {
        id, err := nanoid.New()
        require.NoError(t, err, "New() should not return an error")
        for _, c := range id {
            assert.True(t, unicode.IsLetter(c) || unicode.IsDigit(c),
                "New() returned non-alphanumeric char %q in ID %q", c, id)
        }
    }
}

func TestNew_NoDuplicates(t *testing.T) {
    seen := make(map[string]struct{}, 1000)
    for i := 0; i < 1000; i++ {
        id, err := nanoid.New()
        require.NoError(t, err, "New() should not return an error")
        _, exists := seen[id]
        assert.False(t, exists, "New() produced duplicate ID %q at iteration %d", id, i)
        seen[id] = struct{}{}
    }
}

func TestMustNew_ReturnsIDOfCorrectLength(t *testing.T) {
    id := nanoid.MustNew()
    assert.Equal(t, nanoid.IDLength, len(id), "MustNew() should return an ID of IDLength characters")
}

func TestMustNew_ReturnsAlphanumericOnly(t *testing.T) {
    for i := 0; i < 100; i++ {
        id := nanoid.MustNew()
        for _, c := range id {
            assert.True(t, unicode.IsLetter(c) || unicode.IsDigit(c),
                "MustNew() returned non-alphanumeric char %q in ID %q", c, id)
        }
    }
}
```

### 1b. Updated test constants — verify they fail against current UUID-based code

The constants in `internal/database/helper_test.go` will be rewritten to 10-char alphanumeric values (Wave 1-impl task 8). The tester writes the NEW constants file first. Existing tests that use these constants will fail because the new IDs don't match the UUID values seeded in test databases.

**Tester action:** Write `internal/database/helper_test.go` with the new 10-char constants. Run `go test ./internal/database/...` to confirm failures. Document the failure output. Do NOT fix yet.

New constants to write (failing state):
```go
const (
    TestLocationWorkshop  = "tst0loc001"
    TestLocationStorage   = "tst0loc002"
    TestLocationToolbox   = "tst0loc003"
    TestLocationWorkbench = "tst0loc004"
    TestLocationShelves   = "tst0loc005"
    TestLocationBinA      = "tst0loc006"
    TestLocationBinB      = "tst0loc007"
    TestItem10mmSocket    = "tst0itm001"
    TestItemScrewdriverSet = "tst0itm002"
    TestItemHammer        = "tst0itm003"
    TestItemDrillBits     = "tst0itm004"
    TestItemSandpaper     = "tst0itm005"
    TestItemMissingWrench = "tst0itm006"
    TestItemBorrowedSaw   = "tst0itm007"

    // Not entity IDs — unchanged
    TestProjectDeck     = "test-project-deck"
    TestProjectShelving = "test-project-shelving"
    TestActorUser       = "test-user"
)
```

### 1c. LooksLikeID tests — written before selectors.go is modified

Files to write:
- `cmd/move/helpers_test.go` — rename `TestLooksLikeUUID` to `TestLooksLikeID`, update call sites and test cases
- `internal/cli/selectors_test.go` — same rename and update

These tests will fail because `cli.LooksLikeID` does not exist yet (only `cli.LooksLikeUUID` exists). The compile failure IS the failing state.

Updated test cases for `TestLooksLikeID`:
```go
// Valid IDs (10 alphanumeric chars)
{"aB3xK9mPqR", true},
{"0000000000", true},
{"AAAAAAAAAA", true},
{"tst0loc001", true},   // test constant format

// Invalid: UUID format
{"01936e3e-1000-7890-abcd-ef0123456789", false},
// Invalid: too short
{"aB3xK9mPq", false},
// Invalid: too long
{"aB3xK9mPqRx", false},
// Invalid: contains underscore
{"aB3xK9mP_R", false},
// Invalid: contains hyphen
{"aB3xK9mP-R", false},
// Invalid: empty
{"", false},
```

---

## Wave 1-impl: Foundation Implementation (parallel where possible)

**Blocked until:** Wave 1-test subtasks are written and verified failing.

All Wave 1-impl tasks are independent of each other and can run in parallel.

### Task 1: go.mod + internal/nanoid Package

**Agent:** golang-developer
**Makes green:** Wave 1-test subtask 1a

**Files to create/modify:**
- `go.mod` — add `github.com/matoous/go-nanoid/v2`
- `internal/nanoid/nanoid.go` — new file

#### go.mod change

Add to `require` block:
```
github.com/matoous/go-nanoid/v2 v2.1.0
```

Remove from `require` block (after all callers updated, done in Task 7):
```
github.com/google/uuid v1.6.0
```

Run after all changes: `go get github.com/matoous/go-nanoid/v2 && go mod tidy`

#### internal/nanoid/nanoid.go (new file)

```go
// Package nanoid provides ID generation for wherehouse entities.
// It wraps github.com/matoous/go-nanoid/v2 with a fixed alphabet and length.
package nanoid

import gonanoid "github.com/matoous/go-nanoid/v2"

// IDLength is the standard character length for all entity IDs.
const IDLength = 10

// alphabet contains only alphanumeric characters (A-Za-z0-9).
// 62 characters chosen to avoid ambiguous symbols and maximize readability.
const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

// New generates a new random ID of IDLength alphanumeric characters.
// Returns an error only if the system entropy source fails.
func New() (string, error) {
    return gonanoid.Generate(alphabet, IDLength)
}

// MustNew generates a new random ID, panicking if entropy fails.
// Suitable for test code or initialization where failure is unrecoverable.
func MustNew() string {
    id, err := New()
    if err != nil {
        panic("nanoid: failed to generate ID: " + err.Error())
    }
    return id
}
```

**Note on alphabet:** `gonanoid.Generate(alphabet, size)` accepts a custom alphabet string. Using `alphabet` constant with only `A-Za-z0-9` (62 chars) satisfies the alphanumeric-only requirement. Collision space: 62^10 ≈ 8.4 × 10^17 unique IDs.

### Task 8: Test Constants in helper_test.go

**Agent:** golang-tester (already written in Wave 1-test; this task confirms the write is complete)
**Makes green:** Partially — constants will be correct after Wave 1-impl Tasks 10 and schema seed data align

**Note:** The constants were written in Wave 1-test to create failing tests. No additional work needed here; this is a placeholder to confirm the file was written.

### Task 9: cmd/history/output.go — Remove uuidPrefixLength

**Agent:** golang-developer
**Wave:** 1 (no test required — this is a removal of dead code, not a new behavior)
**Files to modify:** `cmd/history/output.go`

Current code (lines 16-20):
```go
const (
    hoursPerDay         = 24
    recentDaysThreshold = 7
    uuidPrefixLength    = 8
)
```

And usage at line 341-343:
```go
if len(locationID) >= uuidPrefixLength {
    return fmt.Sprintf("location:%s", locationID[:uuidPrefixLength])
}
return fmt.Sprintf("location:%s", locationID)
```

New code — remove `uuidPrefixLength` constant entirely and simplify the fallback:
```go
const (
    hoursPerDay         = 24
    recentDaysThreshold = 7
)
```

Replace the fallback display block with:
```go
return fmt.Sprintf("location:%s", locationID)
```

The `len` guard is no longer needed: nanoid IDs are always exactly 10 characters, and even if an old UUID somehow appears, showing the full string is correct behavior.

### Task 10: System Location IDs in schema_metadata.go

**Agent:** golang-developer
**Wave:** 1 (no dependencies)
**Files to modify:** `internal/database/schema_metadata.go`

Current code (lines 18-23):
```go
const (
    missingID  = "00000000-0000-0000-0000-000000000001"
    borrowedID = "00000000-0000-0000-0000-000000000002"
    loanedID   = "00000000-0000-0000-0000-000000000003"
)
```

New code:
```go
const (
    // System location IDs are deterministic and stable across all databases.
    // These fixed 10-character IDs are used instead of randomly generated nanoids.
    missingID  = "sys0000001"
    borrowedID = "sys0000002"
    loanedID   = "sys0000003"
)
```

Update the comment above `seedSystemLocations` from:
```go
// Deterministic UUIDs for system locations (same across all databases)
```
to:
```go
// Deterministic IDs for system locations (same across all databases)
```

### Task 11: SQL Migration Files

**Agent:** golang-developer
**Wave:** 1 (no dependencies)
**Files to create:**
- `internal/database/migrations/000003_nanoid_migration.up.sql`
- `internal/database/migrations/000003_nanoid_migration.down.sql`

#### 000003_nanoid_migration.up.sql (new file)

```sql
-- Migration 000003: ID format migration to nanoid
--
-- This migration serves as a schema version marker only.
-- No DDL changes are required because all ID columns are TEXT (format-agnostic).
--
-- The actual data transformation (rewriting UUID IDs to nanoid IDs) is performed
-- by the Go command: wherehouse migrate database
--
-- Running `wherehouse migrate database` is opt-in and must be done separately.
-- The application continues to work after this schema version is applied, with
-- new entities receiving nanoid IDs while old entities retain their UUID IDs
-- until the migration command is run.

SELECT 1; -- no-op version marker
```

#### 000003_nanoid_migration.down.sql (new file)

```sql
-- Migration 000003 down: No DDL to reverse.
-- Data changes made by `wherehouse migrate database` are not automatically reversed.
-- To reverse data changes, restore from a database backup taken before migration.

SELECT 1; -- no-op
```

**Note on historical SQL files:** `000002_add_loaned_system_location.up.sql` hardcodes the old UUID `'00000000-0000-0000-0000-000000000003'`. Do NOT edit this file — it would break golang-migrate's checksum validation. Migration 003 (the Go command) handles rewriting this value.

### Task 12: Doc String Updates

**Agent:** golang-developer
**Wave:** 1 (no dependencies — text changes only)
**Files to modify** (grep for "UUID" in cmd/ directory):

Search pattern: `grep -r "UUID\|uuid\|550e8400" cmd/ --include="*.go" -l`

Specific files from initial analysis:
- `cmd/lost/doc.go` — "UUID: Exact ID match" → "ID: Exact ID match"
- `cmd/found/found.go`, `cmd/found/doc.go` — remove example UUID string, replace with `aB3xK9mPqR`
- `cmd/loan/loan.go`
- `cmd/move/move.go`, `cmd/move/doc.go`
- `cmd/history/history.go`
- `cmd/list/helpers.go`
- `cmd/add/helpers.go`

Pattern for all: replace "UUID" with "ID", replace example UUID strings like `550e8400-e29b-...` with `aB3xK9mPqR` (example 10-char alphanumeric ID).

---

## Wave 1-verify: Confirm Wave 1 Tests Green

**Agent:** golang-tester

Run:
```
go test ./internal/nanoid/...
```

Confirm:
- `TestIDLength_IsCorrectValue` passes
- `TestNew_ReturnsIDOfCorrectLength` passes
- `TestNew_ReturnsAlphanumericOnly` passes
- `TestNew_NoDuplicates` passes
- `TestMustNew_*` passes

The `helper_test.go` constants will still cause failures in `./internal/database/...` — that is expected at this stage. Those will be resolved in Wave 2-impl (Task 10 updates schema_metadata.go; the seed SQL and projections must also match).

---

## Wave 2-test: Failing Tests for Wave 2 Components

**Agent:** golang-tester
**Runs before:** any Wave 2 implementation
**Depends on:** Wave 1-impl complete (nanoid package exists; can import it)

**Reminder:** All test code must use testify (assert/require). Do not use t.Fatal, t.Fatalf, t.Error, or t.Errorf.

### 2a. LooksLikeID tests (already written in Wave 1-test — confirm still failing)

Confirm that `internal/cli/selectors_test.go` and `cmd/move/helpers_test.go` still fail to compile because `cli.LooksLikeID` does not yet exist. No additional work needed; this is a checkpoint.

### 2b. cli.MigrateDatabase tests (new file: `internal/cli/migrate_test.go`)

The `internal/cli/migrate.go` file does not exist yet. Tests will fail to compile until implementation begins.

**File to create:** `internal/cli/migrate_test.go`

Test cases to cover:

**Dry-run behavior:**
- `TestMigrateDatabase_DryRun_PrintsPreview` — with `dryRun=true`, output contains "DRY RUN" header, location and item mapping lines, and "Dry run complete" footer; database is unchanged
- `TestMigrateDatabase_DryRun_NoDBChanges` — after dry-run, query locations_current and items_current; IDs are still original UUID format

**Atomic rollback:**
- `TestMigrateDatabase_AtomicRollback_OnError` — inject a DB error mid-migration (e.g., constraint violation); verify no rows were changed (transaction rolled back fully)

**ID remapping correctness:**
- `TestMigrateDatabase_SystemLocations_GetDeterministicIDs` — after migration, "missing" location has ID `sys0000001`, "borrowed" → `sys0000002`, "loaned" → `sys0000003`
- `TestMigrateDatabase_UserLocations_GetNanoidIDs` — after migration, user-created locations have 10-char alphanumeric IDs
- `TestMigrateDatabase_Items_GetNanoidIDs` — after migration, items have 10-char alphanumeric IDs
- `TestMigrateDatabase_EventPayloads_UpdatedCorrectly` — after migration, `events.payload` JSON no longer contains any original UUID strings; new IDs appear in payload

**Idempotency:**
- `TestMigrateDatabase_Idempotency` — run migration twice; second run succeeds (or returns a clear "already migrated" signal); IDs do not change on second run

**Output format:**
- `TestMigrateDatabase_PrintsMappingReport` — output lists all location and item ID mappings in format `  oldID -> newID`
- `TestMigrateDatabase_PrintsPostMigrationInstructions` — output includes post-migration notes after success

```go
// internal/cli/migrate_test.go
package cli_test

import (
    "bytes"
    "context"
    "strings"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/spf13/cobra"

    "github.com/asphaltbuffet/wherehouse/internal/cli"
    // test database helpers
    "github.com/asphaltbuffet/wherehouse/internal/database"
)

func newTestCmd(buf *bytes.Buffer) *cobra.Command {
    cmd := &cobra.Command{}
    cmd.SetOut(buf)
    cmd.SetContext(context.Background())
    return cmd
}

func TestMigrateDatabase_DryRun_PrintsPreview(t *testing.T) {
    db := database.OpenTestDatabase(t)
    var buf bytes.Buffer
    cmd := newTestCmd(&buf)

    err := cli.MigrateDatabase(cmd, db, true)
    require.NoError(t, err, "MigrateDatabase dry-run should not return an error")

    out := buf.String()
    assert.Contains(t, out, "DRY RUN", "dry-run output should contain DRY RUN header")
    assert.Contains(t, out, "Dry run complete", "dry-run output should contain completion message")
}

func TestMigrateDatabase_DryRun_NoDBChanges(t *testing.T) {
    db := database.OpenTestDatabase(t)
    var buf bytes.Buffer
    cmd := newTestCmd(&buf)

    err := cli.MigrateDatabase(cmd, db, true)
    require.NoError(t, err, "MigrateDatabase dry-run should not return an error")

    // Verify original UUIDs still present
    locs, err := db.GetAllLocations(context.Background())
    require.NoError(t, err, "GetAllLocations should not return an error")
    for _, loc := range locs {
        assert.NotEqual(t, 10, len(loc.LocationID),
            "dry-run should not have modified location ID to nanoid format: %q", loc.LocationID)
    }
}

func TestMigrateDatabase_SystemLocations_GetDeterministicIDs(t *testing.T) {
    db := database.OpenTestDatabase(t)
    var buf bytes.Buffer
    cmd := newTestCmd(&buf)

    err := cli.MigrateDatabase(cmd, db, false)
    require.NoError(t, err, "MigrateDatabase should not return an error")

    locs, err := db.GetAllLocations(context.Background())
    require.NoError(t, err, "GetAllLocations should not return an error")

    systemIDs := map[string]string{
        "missing":  "sys0000001",
        "borrowed": "sys0000002",
        "loaned":   "sys0000003",
    }

    for _, loc := range locs {
        if expected, ok := systemIDs[loc.CanonicalName]; ok {
            assert.Equal(t, expected, loc.LocationID,
                "system location %q should have deterministic ID", loc.CanonicalName)
        }
    }
}

func TestMigrateDatabase_Idempotency(t *testing.T) {
    db := database.OpenTestDatabase(t)
    cmd := newTestCmd(&bytes.Buffer{})

    err := cli.MigrateDatabase(cmd, db, false)
    require.NoError(t, err, "first MigrateDatabase should not return an error")

    // Second run must not error
    err = cli.MigrateDatabase(cmd, db, false)
    require.NoError(t, err, "second MigrateDatabase (idempotency) should not return an error")
}

// Ensure strings import is used (output contains -> mapping lines)
var _ = strings.Contains
```

### 2c. wherehouse migrate database command integration tests (new file)

**File to create:** `cmd/migrate/database_test.go`

These will fail to compile until `cmd/migrate/` exists.

Test cases:
- `TestGetDatabaseCmd_RegisteredUnderMigrateCmd` — `GetMigrateCmd()` has a subcommand named "database"
- `TestGetDatabaseCmd_HasDryRunFlag` — `database` subcommand has `--dry-run` boolean flag
- `TestGetDatabaseCmd_DryRunDefaultFalse` — `--dry-run` default is `false`
- `TestGetDatabaseCmd_ShortHelp` — short help text is not empty and mentions "migrate"

```go
// cmd/migrate/database_test.go
package migrate_test

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/asphaltbuffet/wherehouse/cmd/migrate"
)

func TestGetDatabaseCmd_RegisteredUnderMigrateCmd(t *testing.T) {
    migrateCmd := migrate.GetMigrateCmd()
    for _, sub := range migrateCmd.Commands() {
        if sub.Use == "database" {
            return
        }
    }
    assert.Fail(t, "migrate command has no 'database' subcommand")
}

func TestGetDatabaseCmd_HasDryRunFlag(t *testing.T) {
    cmd := migrate.GetDatabaseCmd()
    flag := cmd.Flags().Lookup("dry-run")
    assert.NotNil(t, flag, "database command should have --dry-run flag")
}

func TestGetDatabaseCmd_DryRunDefaultFalse(t *testing.T) {
    cmd := migrate.GetDatabaseCmd()
    flag := cmd.Flags().Lookup("dry-run")
    require.NotNil(t, flag, "database command should have --dry-run flag")
    assert.Equal(t, "false", flag.DefValue, "--dry-run default should be false")
}

func TestGetDatabaseCmd_ShortHelp(t *testing.T) {
    cmd := migrate.GetDatabaseCmd()
    assert.NotEmpty(t, cmd.Short, "database command should have short help text")
}
```

---

## Wave 2-impl: Core Implementation (parallel where possible)

**Blocked until:** Wave 2-test subtasks are written and verified failing, AND Wave 1-impl is complete.

### Task 2: ID Generation in Add Commands

**Agent:** golang-developer
**Depends on:** Task 1 (nanoid package)
**Files to modify:**
- `cmd/add/location.go`
- `cmd/add/item.go`

#### cmd/add/location.go

Remove import: `"github.com/google/uuid"`
Add import: `"github.com/asphaltbuffet/wherehouse/internal/nanoid"`

Replace (around line 97):
```go
locationUUID, uuidErr := uuid.NewV7()
if uuidErr != nil {
    return fmt.Errorf("failed to generate UUID for location %q: %w", locationName, uuidErr)
}
locationID := locationUUID.String()
```

With:
```go
locationID, idErr := nanoid.New()
if idErr != nil {
    return fmt.Errorf("failed to generate ID for location %q: %w", locationName, idErr)
}
```

Update any doc comment mentioning "UUID" to say "ID" instead.

#### cmd/add/item.go

Remove import: `"github.com/google/uuid"`
Add import: `"github.com/asphaltbuffet/wherehouse/internal/nanoid"`

Replace (around line 88):
```go
itemUUID, uuidErr := uuid.NewV7()
if uuidErr != nil {
    return fmt.Errorf("failed to generate UUID: %w", uuidErr)
}
itemID := itemUUID.String()
```

With:
```go
itemID, idErr := nanoid.New()
if idErr != nil {
    return fmt.Errorf("failed to generate ID: %w", idErr)
}
```

Update any doc comment mentioning "UUID" to say "ID" instead.

### Task 3: LooksLikeID in selectors.go

**Agent:** golang-developer
**Depends on:** Task 1 (nanoid package), Wave 2-test 2a (tests written and failing)
**Files to modify:** `internal/cli/selectors.go`

Remove import: `"github.com/google/uuid"`
Add import: `"github.com/asphaltbuffet/wherehouse/internal/nanoid"`

Replace `LooksLikeUUID` function entirely:

```go
// LooksLikeID checks if a string looks like a wherehouse entity ID.
// Returns true if the string is exactly nanoid.IDLength alphanumeric characters.
func LooksLikeID(s string) bool {
    if len(s) != nanoid.IDLength {
        return false
    }
    for _, c := range s {
        if !isIDChar(c) {
            return false
        }
    }
    return true
}

// isIDChar returns true if c is in the ID alphabet (A-Za-z0-9).
func isIDChar(c rune) bool {
    return (c >= 'A' && c <= 'Z') ||
        (c >= 'a' && c <= 'z') ||
        (c >= '0' && c <= '9')
}
```

Update all internal call sites within selectors.go:
- `ResolveLocation`: `LooksLikeUUID(input)` → `LooksLikeID(input)`
- `ResolveItemSelector`: `LooksLikeUUID(selector)` → `LooksLikeID(selector)`

Update doc comments on `ResolveLocation` and `ResolveItemSelector` — remove references to "UUID", say "ID" instead.

### Task 4: wherehouse migrate database Command

**Agent:** golang-developer
**Depends on:** Task 10 (system ID constants known), Task 11 (SQL migration version marker exists), Wave 2-test 2b and 2c (tests written and failing)

**Thin-cmd split:**
- `cmd/migrate/doc.go` — new file (package doc)
- `cmd/migrate/migrate.go` — new file (parent cobra command, thin)
- `cmd/migrate/database.go` — new file (thin `database` subcommand, delegates to internal/cli)
- `internal/cli/migrate.go` — new file (ALL business logic)
- `cmd/root.go` — register new command

#### cmd/migrate/doc.go (new file)

```go
// Package migrate provides the migrate command and its subcommands.
// The migrate command handles data migration operations for wherehouse.
package migrate
```

#### cmd/migrate/migrate.go (new file)

```go
package migrate

import "github.com/spf13/cobra"

var migrateCmd *cobra.Command

// GetMigrateCmd returns the parent migrate command.
func GetMigrateCmd() *cobra.Command {
    if migrateCmd != nil {
        return migrateCmd
    }

    migrateCmd = &cobra.Command{
        Use:   "migrate",
        Short: "run data migration operations",
        Long: `The migrate command provides subcommands for migrating wherehouse data.

Examples:
  wherehouse migrate database        Migrate IDs from UUID to nanoid format`,
    }

    migrateCmd.AddCommand(GetDatabaseCmd())

    return migrateCmd
}
```

#### cmd/migrate/database.go (new file — thin wrapper only)

```go
package migrate

import (
    "fmt"

    "github.com/spf13/cobra"

    "github.com/asphaltbuffet/wherehouse/internal/cli"
)

var databaseCmd *cobra.Command

// GetDatabaseCmd returns the `migrate database` subcommand.
func GetDatabaseCmd() *cobra.Command {
    if databaseCmd != nil {
        return databaseCmd
    }

    var dryRun bool

    databaseCmd = &cobra.Command{
        Use:   "database",
        Short: "migrate database IDs from UUID to nanoid format",
        Long: `Rewrites all entity IDs in the wherehouse database from UUID format
to 10-character alphanumeric nanoid format.

This command is opt-in and must be run explicitly. It operates as a single
atomic transaction: either all IDs are migrated successfully or no changes
are made.

System locations receive deterministic IDs:
  Missing  -> sys0000001
  Borrowed -> sys0000002
  Loaned   -> sys0000003

All other locations and items receive new randomly generated IDs.
Both projection tables and event payload JSON are updated together.

WARNING: Back up your database before running this migration.
After migration, any external references to old UUID-format IDs will be invalid.

Examples:
  wherehouse migrate database --dry-run   Preview migration without making changes
  wherehouse migrate database             Run migration`,
        RunE: func(cmd *cobra.Command, args []string) error {
            db, err := cli.OpenDatabase(cmd.Context())
            if err != nil {
                return fmt.Errorf("failed to open database: %w", err)
            }
            defer db.Close()

            return cli.MigrateDatabase(cmd, db, dryRun)
        },
    }

    databaseCmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview migration without making changes")

    return databaseCmd
}
```

**Explanation of RunE:** It uses the existing `cli.OpenDatabase(ctx)` helper (already in `internal/cli/database.go`), passes the open `*database.Database` to `cli.MigrateDatabase`, and returns. That is the full extent of the cmd layer's responsibility.

#### internal/cli/migrate.go (new file — all business logic)

```go
package cli

import (
    "context"
    "database/sql"
    "fmt"
    "strings"

    "github.com/spf13/cobra"

    "github.com/asphaltbuffet/wherehouse/internal/database"
    "github.com/asphaltbuffet/wherehouse/internal/nanoid"
)

// systemIDMap maps canonical system location names to their fixed deterministic IDs.
var systemIDMap = map[string]string{
    "missing":  "sys0000001",
    "borrowed": "sys0000002",
    "loaned":   "sys0000003",
}

// migrateMapping tracks old-to-new ID remapping for a migration run.
type migrateMapping struct {
    Locations map[string]string // old location_id -> new location_id
    Items     map[string]string // old item_id -> new item_id
}

// MigrateDatabase rewrites all entity IDs from UUID format to nanoid format.
// It uses cmd for output (cmd.OutOrStdout()) and respects the dryRun flag.
// All changes are applied in a single atomic transaction; on failure no changes persist.
func MigrateDatabase(cmd *cobra.Command, db *database.Database, dryRun bool) error {
    ctx := cmd.Context()
    w := cmd.OutOrStdout()

    if dryRun {
        fmt.Fprintln(w, "DRY RUN: No changes will be made to the database.")
        fmt.Fprintln(w)
    }

    // Build ID mapping
    mapping, err := buildIDMapping(ctx, db)
    if err != nil {
        return fmt.Errorf("failed to build ID mapping: %w", err)
    }

    // Print mapping report
    printMappingReport(w, mapping)

    if dryRun {
        fmt.Fprintln(w)
        fmt.Fprintln(w, "Dry run complete. Run without --dry-run to apply changes.")
        return nil
    }

    // Apply migration in a single transaction
    fmt.Fprintln(w)
    fmt.Fprintln(w, "Applying migration...")

    if err := db.ExecInTransaction(ctx, func(tx *sql.Tx) error {
        return applyIDMigration(ctx, tx, mapping)
    }); err != nil {
        return fmt.Errorf("migration failed (no changes applied): %w", err)
    }

    fmt.Fprintln(w, "Migration complete.")
    printPostMigrationInstructions(w)
    return nil
}

// buildIDMapping queries the database and constructs old->new ID mappings.
// System locations get fixed IDs; user entities get new random IDs.
func buildIDMapping(ctx context.Context, db *database.Database) (*migrateMapping, error) {
    mapping := &migrateMapping{
        Locations: make(map[string]string),
        Items:     make(map[string]string),
    }

    // Map location IDs
    locations, err := db.GetAllLocations(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to query locations: %w", err)
    }
    for _, loc := range locations {
        if fixedID, ok := systemIDMap[loc.CanonicalName]; ok {
            mapping.Locations[loc.LocationID] = fixedID
        } else {
            newID, genErr := nanoid.New()
            if genErr != nil {
                return nil, fmt.Errorf("failed to generate ID for location %q: %w", loc.DisplayName, genErr)
            }
            mapping.Locations[loc.LocationID] = newID
        }
    }

    // Map item IDs
    items, err := db.GetAllItems(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to query items: %w", err)
    }
    for _, item := range items {
        newID, genErr := nanoid.New()
        if genErr != nil {
            return nil, fmt.Errorf("failed to generate ID for item %q: %w", item.DisplayName, genErr)
        }
        mapping.Items[item.ItemID] = newID
    }

    return mapping, nil
}

// applyIDMigration applies the full ID rewrite within a transaction.
func applyIDMigration(ctx context.Context, tx *sql.Tx, mapping *migrateMapping) error {
    // 1. Update locations_current - location_id and parent_id
    for oldID, newID := range mapping.Locations {
        if _, err := tx.ExecContext(ctx,
            `UPDATE locations_current SET location_id = ? WHERE location_id = ?`,
            newID, oldID); err != nil {
            return fmt.Errorf("failed to update location_id %q: %w", oldID, err)
        }
        // Update parent references
        if _, err := tx.ExecContext(ctx,
            `UPDATE locations_current SET parent_id = ? WHERE parent_id = ?`,
            newID, oldID); err != nil {
            return fmt.Errorf("failed to update parent_id reference %q: %w", oldID, err)
        }
    }

    // 2. Update items_current - item_id, location_id, temp_origin_location_id
    for oldID, newID := range mapping.Items {
        if _, err := tx.ExecContext(ctx,
            `UPDATE items_current SET item_id = ? WHERE item_id = ?`,
            newID, oldID); err != nil {
            return fmt.Errorf("failed to update item_id %q: %w", oldID, err)
        }
    }
    // Update location_id references in items_current
    for oldLocID, newLocID := range mapping.Locations {
        if _, err := tx.ExecContext(ctx,
            `UPDATE items_current SET location_id = ? WHERE location_id = ?`,
            newLocID, oldLocID); err != nil {
            return fmt.Errorf("failed to update items_current.location_id for %q: %w", oldLocID, err)
        }
        if _, err := tx.ExecContext(ctx,
            `UPDATE items_current SET temp_origin_location_id = ? WHERE temp_origin_location_id = ?`,
            newLocID, oldLocID); err != nil {
            return fmt.Errorf("failed to update items_current.temp_origin_location_id for %q: %w", oldLocID, err)
        }
    }

    // 3. Update events table indexed columns (item_id, location_id)
    for oldID, newID := range mapping.Items {
        if _, err := tx.ExecContext(ctx,
            `UPDATE events SET item_id = ? WHERE item_id = ?`,
            newID, oldID); err != nil {
            return fmt.Errorf("failed to update events.item_id %q: %w", oldID, err)
        }
    }
    for oldID, newID := range mapping.Locations {
        if _, err := tx.ExecContext(ctx,
            `UPDATE events SET location_id = ? WHERE location_id = ?`,
            newID, oldID); err != nil {
            return fmt.Errorf("failed to update events.location_id %q: %w", oldID, err)
        }
    }

    // 4. Update events.payload JSON strings (string replacement for each old->new mapping)
    allMappings := make(map[string]string, len(mapping.Locations)+len(mapping.Items))
    for k, v := range mapping.Locations {
        allMappings[k] = v
    }
    for k, v := range mapping.Items {
        allMappings[k] = v
    }

    if err := rewriteEventPayloads(ctx, tx, allMappings); err != nil {
        return fmt.Errorf("failed to rewrite event payloads: %w", err)
    }

    return nil
}

// rewriteEventPayloads applies string substitution to all event payload JSON blobs.
// Each old ID string is replaced with its new ID string.
// Uses Go-side processing to handle the full mapping correctly.
func rewriteEventPayloads(ctx context.Context, tx *sql.Tx, mapping map[string]string) error {
    rows, err := tx.QueryContext(ctx, `SELECT event_id, payload FROM events`)
    if err != nil {
        return fmt.Errorf("failed to query events: %w", err)
    }
    defer rows.Close()

    type eventRow struct {
        id      int64
        payload string
    }
    var events []eventRow
    for rows.Next() {
        var e eventRow
        if err := rows.Scan(&e.id, &e.payload); err != nil {
            return err
        }
        events = append(events, e)
    }
    if err := rows.Err(); err != nil {
        return err
    }

    stmt, err := tx.PrepareContext(ctx, `UPDATE events SET payload = ? WHERE event_id = ?`)
    if err != nil {
        return err
    }
    defer stmt.Close()

    for _, e := range events {
        updated := e.payload
        for oldID, newID := range mapping {
            updated = strings.ReplaceAll(updated, oldID, newID)
        }
        if updated != e.payload {
            if _, err := stmt.ExecContext(ctx, updated, e.id); err != nil {
                return fmt.Errorf("failed to update event %d payload: %w", e.id, err)
            }
        }
    }
    return nil
}

func printMappingReport(w interface{ Write([]byte) (int, error) }, mapping *migrateMapping) {
    fmt.Fprintf(w, "Location ID mappings (%d):\n", len(mapping.Locations))
    for oldID, newID := range mapping.Locations {
        fmt.Fprintf(w, "  %s -> %s\n", oldID, newID)
    }
    fmt.Fprintf(w, "\nItem ID mappings (%d):\n", len(mapping.Items))
    for oldID, newID := range mapping.Items {
        fmt.Fprintf(w, "  %s -> %s\n", oldID, newID)
    }
}

func printPostMigrationInstructions(w interface{ Write([]byte) (int, error) }) {
    fmt.Fprintln(w)
    fmt.Fprintln(w, "Post-migration notes:")
    fmt.Fprintln(w, "  - Any external scripts or bookmarks using old UUID-format IDs are no longer valid.")
    fmt.Fprintln(w, "  - See docs/migration-nanoid.md for full migration guidance.")
}
```

**Required new database methods** (to be added to `internal/database/` by the developer):
- `db.GetAllLocations(ctx context.Context) ([]*Location, error)` — query all rows from `locations_current`
- `db.GetAllItems(ctx context.Context) ([]*Item, error)` — query all rows from `items_current`
- `db.ExecInTransaction(ctx context.Context, fn func(*sql.Tx) error) error` — execute fn in a transaction

These follow existing query patterns and should be placed in the appropriate query files.

#### cmd/root.go change

Add import:
```go
"github.com/asphaltbuffet/wherehouse/cmd/migrate"
```

Add to `GetRootCmd()` after existing `rootCmd.AddCommand(...)` calls:
```go
rootCmd.AddCommand(migrate.GetMigrateCmd())
```

### Task 5: Test Files Using uuid.New().String()

**Agent:** golang-tester
**Wave:** 2 (depends on Task 1: nanoid package, Task 8: test constants)
**Files to modify:**
- `cmd/lost/item_test.go`
- `cmd/list/list_test.go`
- `cmd/move/item_test.go`
- `internal/cli/selectors_test.go`

In each file:
- Remove import: `"github.com/google/uuid"`
- Add import: `"github.com/asphaltbuffet/wherehouse/internal/nanoid"`
- Replace all occurrences of `uuid.New().String()` with `nanoid.MustNew()`
- Replace all occurrences of `uuid.NewV7().String()` (if any) with `nanoid.MustNew()`
- Replace any `t.Fatal`, `t.Fatalf`, `t.Error`, `t.Errorf` calls with testify equivalents (require/assert)

---

## Wave 2-verify: Confirm Wave 2 Tests Green

**Agent:** golang-tester

Run:
```
go test ./internal/cli/... ./cmd/migrate/... ./cmd/move/... ./internal/database/...
```

Confirm:
- `TestLooksLikeID` passes (both in selectors_test.go and helpers_test.go)
- `TestMigrateDatabase_*` all pass
- `TestGetDatabaseCmd_*` all pass
- `internal/database` tests pass now that constants and schema_metadata.go align

If any test is red, the implementing agent must fix the implementation (not the test) until all tests pass.

---

## Wave 3-test: Failing Tests for Wave 3 Components

**Agent:** golang-tester
**Runs before:** Wave 3 implementation
**Depends on:** Wave 2-impl complete

### 3a. LooksLikeID test cases — final rename verification

Confirm `cmd/move/helpers_test.go` and `internal/cli/selectors_test.go` test functions are named `TestLooksLikeID` (not `TestLooksLikeUUID`). These were written in Wave 1-test; this is a checkpoint to confirm the rename is complete and tests are green.

### 3b. go mod tidy — no new tests required

This is a tooling step; no test cases apply.

### 3c. Documentation — no test cases required

`docs/migration-nanoid.md` is user documentation; no automated tests apply.

---

## Wave 3-impl: Final Steps

**Blocked until:** Wave 2-impl complete and Wave 2-verify green.

### Task 6: Rename LooksLikeUUID Tests (if not already done in Wave 1-test)

**Agent:** golang-tester
**Wave:** 3 (depends on Task 3: LooksLikeID exists)
**Files to modify:**
- `cmd/move/helpers_test.go`
- `internal/cli/selectors_test.go`

If the rename was done in Wave 1-test (as instructed), this task is a no-op verification. If not, complete the rename now.

Final test case inventory for `TestLooksLikeID`:
```go
// Valid IDs (10 alphanumeric chars)
{"aB3xK9mPqR", true},
{"0000000000", true},
{"AAAAAAAAAA", true},
{"tst0loc001", true},

// Invalid: UUID format
{"01936e3e-1000-7890-abcd-ef0123456789", false},
// Invalid: too short
{"aB3xK9mPq", false},
// Invalid: too long
{"aB3xK9mPqRx", false},
// Invalid: contains underscore
{"aB3xK9mP_R", false},
// Invalid: contains hyphen
{"aB3xK9mP-R", false},
// Invalid: empty
{"", false},
```

### Task 7: go mod tidy

**Agent:** golang-developer
**Wave:** 3 (depends on all import changes complete)

```
go get github.com/matoous/go-nanoid/v2
go mod tidy
```

Verify `github.com/google/uuid` no longer appears in `go.mod` or `go.sum`.

### Task 13: User Documentation

**Agent:** golang-developer
**Wave:** 3 (depends on Task 4: command flags finalized)
**File to create:** `docs/migration-nanoid.md`

Content outline:
1. What changed and why (IDs are now 10-char alphanumeric, shorter and cleaner)
2. Impact: existing databases with UUID IDs will continue to work for new records; old UUIDs remain until migration command is run
3. Migration steps:
   - Back up your database: `cp ~/.wherehouse/wherehouse.db ~/.wherehouse/wherehouse.db.bak`
   - Preview changes: `wherehouse migrate database --dry-run`
   - Apply changes: `wherehouse migrate database`
4. Post-migration: any external references (scripts, notes) to old UUID-format IDs are invalid after migration
5. Rollback: restore from backup (no automated rollback provided)

---

## Wave 3-verify: Final Green Bar

**Agent:** golang-tester

Run full test suite:
```
go test ./...
```

All tests must pass. If any fail, the implementing agent fixes the implementation until green. No test modifications are permitted at this stage unless a test contains a factual error (e.g., wrong expected constant value).

---

## TDD Summary: Test-First Checklist

| Component | Test file | Written in wave | Impl in wave | Verifier |
|-----------|-----------|-----------------|--------------|----------|
| `internal/nanoid` package | `internal/nanoid/nanoid_test.go` | Wave 1-test | Wave 1-impl Task 1 | Wave 1-verify |
| `helper_test.go` constants | `internal/database/helper_test.go` | Wave 1-test | Wave 1-impl Task 8 (confirm) | Wave 2-verify |
| `LooksLikeID` function | `cmd/move/helpers_test.go`, `internal/cli/selectors_test.go` | Wave 1-test | Wave 2-impl Task 3 | Wave 2-verify |
| `cli.MigrateDatabase` | `internal/cli/migrate_test.go` | Wave 2-test | Wave 2-impl Task 4 | Wave 2-verify |
| `cmd/migrate` commands | `cmd/migrate/database_test.go` | Wave 2-test | Wave 2-impl Task 4 | Wave 2-verify |
| `nanoid.MustNew()` in test files | (updates to existing test files) | Wave 2-impl | Wave 2-impl Task 5 | Wave 2-verify |

---

## Files Summary

### New Files
| File | Task | TDD role |
|------|------|----------|
| `internal/nanoid/nanoid_test.go` | Wave 1-test | Test-first for nanoid package |
| `internal/nanoid/nanoid.go` | Task 1 | Implementation |
| `internal/cli/migrate_test.go` | Wave 2-test | Test-first for MigrateDatabase |
| `cmd/migrate/database_test.go` | Wave 2-test | Test-first for cmd/migrate |
| `cmd/migrate/doc.go` | Task 4 | Implementation |
| `cmd/migrate/migrate.go` | Task 4 | Implementation |
| `cmd/migrate/database.go` | Task 4 | Implementation |
| `internal/cli/migrate.go` | Task 4 | Implementation |
| `internal/database/migrations/000003_nanoid_migration.up.sql` | Task 11 | Implementation |
| `internal/database/migrations/000003_nanoid_migration.down.sql` | Task 11 | Implementation |
| `docs/migration-nanoid.md` | Task 13 | Documentation |

### Modified Files
| File | Task | TDD role |
|------|------|----------|
| `internal/database/helper_test.go` | Wave 1-test / Task 8 | Test-first (constants create failing tests) |
| `cmd/move/helpers_test.go` | Wave 1-test / Task 6 | Test-first for LooksLikeID |
| `internal/cli/selectors_test.go` | Wave 1-test / Task 5, 6 | Test-first + nanoid.MustNew() |
| `go.mod` / `go.sum` | Task 1, 7 | Implementation |
| `cmd/root.go` | Task 4 | Implementation |
| `cmd/add/location.go` | Task 2 | Implementation |
| `cmd/add/item.go` | Task 2 | Implementation |
| `internal/cli/selectors.go` | Task 3 | Implementation |
| `internal/database/schema_metadata.go` | Task 10 | Implementation |
| `cmd/history/output.go` | Task 9 | Implementation |
| `cmd/lost/item_test.go` | Task 5 | Test update |
| `cmd/list/list_test.go` | Task 5 | Test update |
| `cmd/move/item_test.go` | Task 5 | Test update |
| `cmd/lost/doc.go` | Task 12 | Implementation |
| `cmd/found/found.go`, `cmd/found/doc.go` | Task 12 | Implementation |
| `cmd/loan/loan.go` | Task 12 | Implementation |
| `cmd/move/move.go`, `cmd/move/doc.go` | Task 12 | Implementation |
| `cmd/history/history.go` | Task 12 | Implementation |
| `cmd/list/helpers.go` | Task 12 | Implementation |
| `cmd/add/helpers.go` | Task 12 | Implementation |

### New Database Methods Required
The developer implementing Task 4 must add to `internal/database/`:
- `GetAllLocations(ctx context.Context) ([]*Location, error)`
- `GetAllItems(ctx context.Context) ([]*Item, error)`
- `ExecInTransaction(ctx context.Context, fn func(*sql.Tx) error) error`

These follow existing query patterns and should be placed in the appropriate query files.

---

## Risk Register

| Risk | Severity | Mitigation |
|------|----------|------------|
| events.payload JSON not fully updated | High | Go-side `strings.ReplaceAll` processes every row explicitly |
| System location ID mismatch (old UUID still in migration 002 SQL) | High | Migration 003 Go command rewrites the UUID to `sys000000*` |
| Test constants wrong: `tst0loc001` fails `LooksLikeID` check | Medium | All 10 chars are alphanumeric — verified against new `isIDChar` |
| `LooksLikeID` false positive: user's 10-char alphanumeric item name | Low | Same risk existed with UUID; canonical name fallback still applies |
| `go-nanoid` `Generate` with custom alphabet changes API from `New(size)` | Medium | Verify `gonanoid.Generate(alphabet, size)` signature before implementing |
| Developer forgets to add `GetAllLocations`/`GetAllItems`/`ExecInTransaction` DB methods | Medium | Explicitly called out in Task 4 requirements; covered by migrate_test.go |
| golang-migrate checksum fails if 000002 is edited | High | Explicitly prohibited in Task 11; migration 003 handles data rewrite |
| Implementation agent writes production code before tests are written | Critical | golang-tester wave must complete and be verified failing before impl wave begins |
| Test written against wrong interface (function renamed but test not updated) | Medium | golang-tester verifies compile failure is due to missing feature, not typo |
| Test code uses t.Fatal/t.Error instead of testify | Medium | Project-wide rule enforced; code reviewers reject non-testify test assertions |
