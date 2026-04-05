# Implementation Plan: cmd/ vs internal/cli/ Consistency Refactoring

**Date**: 2026-03-03
**Session**: 20260303-205954
**Source review**: `/home/grue/dev/wherehouse/ai-docs/research/reviews/cmd-cli-consistency-review.md`

---

## Overview

Five sequential steps address all issues from the code review. Steps 3 and 4 are
independent of each other and can be executed in parallel once step 2 is complete.
Step 5 has no code dependencies and can be done at any time.

### Dependency Graph

```
Step 1 (cli.AddLocations)
  └── Step 2 (standardize DI across all commands)
        ├── Step 3 (OutputWriter in find/scry/history/initialize) [parallelizable]
        └── Step 4 (FormatRelativeTime in history)              [parallelizable]

Step 5 (normalize help text) -- independent, no blocking dependencies
```

### Agent Routing (per project-config.md)

| Step | Primary agent | Secondary agent |
|------|--------------|-----------------|
| 1 | `golang-developer` (internal/cli/) | `golang-ui-developer` (cmd/add/) |
| 2 | `golang-ui-developer` (cmd/*)      | none |
| 3 | `golang-ui-developer` (cmd/*)      | none |
| 4 | `golang-ui-developer` (cmd/history/) | none |
| 5 | `golang-ui-developer` (cmd/*)      | none |

---

## Step 1: Extract `cli.AddLocations`

**Addresses**: Review issue 2.1 (HIGH) — business logic in `cmd/add/location.go`

### Motivation

`add/location.go:runAddLocation` contains ~85 lines of domain logic:
name validation, canonicalization, uniqueness checking, ID generation, payload
construction, and event creation. The parallel `cli.AddItems` function in
`internal/cli/add.go` provides the exact same service for items in ~38 lines.
Extracting location creation makes it testable without cobra and removes the
asymmetry between `add item` and `add location`.

### Files to Create

**`/home/grue/dev/wherehouse/internal/cli/locations.go`** (new file)

```go
package cli

import (
    "context"
    "fmt"

    "github.com/asphaltbuffet/wherehouse/internal/database"
    "github.com/asphaltbuffet/wherehouse/internal/nanoid"
)

// AddLocationResult holds the outcome of a single location creation.
type AddLocationResult struct {
    LocationID      string
    DisplayName     string
    FullPathDisplay string // empty if fetch failed post-creation
}

// AddLocations creates one or more named locations in the database.
// If parentName is non-empty, all locations are created as children of that parent.
// parentName may be a canonical name or ID; it is resolved via ResolveLocation.
// Fails fast on the first error (validation, uniqueness, or event insertion).
func AddLocations(ctx context.Context, names []string, parentName string) ([]AddLocationResult, error) {
    db, err := OpenDatabase(ctx)
    if err != nil {
        return nil, err
    }
    defer db.Close()

    var parentID *string
    if parentName != "" {
        resolved, resolveErr := ResolveLocation(ctx, db, parentName)
        if resolveErr != nil {
            return nil, fmt.Errorf("failed to resolve parent location %q: %w", parentName, resolveErr)
        }
        if validateErr := db.ValidateLocationExists(ctx, resolved); validateErr != nil {
            return nil, fmt.Errorf("parent location not found: %w", validateErr)
        }
        parentID = &resolved
    }

    actorUserID := GetActorUserID(ctx)
    results := make([]AddLocationResult, 0, len(names))

    for _, name := range names {
        if validateErr := database.ValidateNoColonInName(name); validateErr != nil {
            return nil, validateErr
        }

        canonicalName := database.CanonicalizeString(name)

        if uniqueErr := db.ValidateUniqueLocationName(ctx, canonicalName, nil); uniqueErr != nil {
            return nil, fmt.Errorf("location %q already exists: %w", name, uniqueErr)
        }

        locationID, idErr := nanoid.New()
        if idErr != nil {
            return nil, fmt.Errorf("failed to generate ID for location %q: %w", name, idErr)
        }

        payload := map[string]any{
            "location_id":    locationID,
            "display_name":   name,
            "canonical_name": canonicalName,
            "parent_id":      parentID,
            "is_system":      false,
        }

        if _, insertErr := db.AppendEvent(ctx, database.LocationCreatedEvent, actorUserID, payload, ""); insertErr != nil {
            return nil, fmt.Errorf("failed to create location %q: %w", name, insertErr)
        }

        result := AddLocationResult{
            LocationID:  locationID,
            DisplayName: name,
        }

        // Best-effort: fetch full path for display. Failure is non-fatal.
        if loc, getErr := db.GetLocation(ctx, locationID); getErr == nil {
            result.FullPathDisplay = loc.FullPathDisplay
        }

        results = append(results, result)
    }

    return results, nil
}
```

### Files to Modify

**`/home/grue/dev/wherehouse/cmd/add/location.go`**

Replace the body of `runAddLocation` with a thin wrapper that:
1. Reads the `--in` flag
2. Calls `cli.AddLocations(ctx, args, parentInput)`
3. Formats output via `OutputWriter` (already present in the file)
4. Removes the inline domain logic (steps 3-6 of the current implementation)
5. Removes unused imports: `database`, `nanoid`

The resulting `RunE` body should be ~20 lines, mirroring `add/item.go`.

Key signature change: `cli.AddLocations` returns `([]AddLocationResult, error)` so
`runAddLocation` can iterate results and call `out.Success(...)` for each.

### Integration Points

- `internal/cli/add.go` remains unchanged (this is a new parallel file)
- `internal/cli/locations.go` follows the same import set as `add.go`
- `cmd/add/helpers.go` is unchanged (the `openDatabase` / `resolveLocation` wrappers
  are no longer called from `location.go` after this step, but they remain for other
  use if any; note: review issue 3.2 suggests removing them in step 2)

### Tests to Write (golang-tester)

- `internal/cli/locations_test.go`: table-driven tests for `AddLocations`:
  - happy path: single location, multiple locations, child location
  - error path: duplicate name, invalid colon in name, missing parent
  - Use in-memory SQLite database (same pattern as `internal/cli/add_test.go` if it exists)

---

## Step 2: Standardize Dependency Injection Across All Commands

**Addresses**: Review issues 3.1 (HIGH), 3.2 (MEDIUM)

### Motivation

Three incompatible testability patterns exist: interface+mockery (move only),
package-level test hooks (list), and no injection at all (most commands). The
goal is to migrate all commands with meaningful business logic to the
`move`-pattern: define a narrow `*DB` interface in each package, generate a mock
via `mockery`, and accept the interface in a `New*Cmd` constructor.

### Commands to Migrate

Priority order (most logic first, as recommended in review):

1. `cmd/found` (115 lines inline logic)
2. `cmd/loan` (63 lines inline logic)
3. `cmd/lost` (53 lines inline logic)
4. `cmd/add` (location.go — already simplified by step 1; item.go delegates to cli)
5. `cmd/list` (remove fragile package-level hooks; adopt interface)
6. `cmd/history` (openDatabase wrapper → interface)
7. `cmd/find` (openDatabase wrapper → interface)
8. `cmd/scry` (direct cli.OpenDatabase call → interface)

### Pattern to Apply (mirror `cmd/move/`)

For each command package, the changes are identical in structure:

#### A. Create `<cmd>/db.go` (interface file)

```go
package <cmd>

import (
    "context"
    "github.com/asphaltbuffet/wherehouse/internal/database"
)

// <cmd>DB is the database interface required by the <cmd> command.
// *database.Database satisfies this interface implicitly.
//
//go:generate mockery
type <cmd>DB interface {
    Close() error
    // ... only the methods actually called in this package
}
```

The interface must be **minimal**: include only the methods that the command's
business logic actually calls. Do not copy the entire `*database.Database` API.

#### B. Update `<cmd>/<cmd>.go` (or the primary command file)

Replace the current `Get*Cmd()` singleton pattern with two constructors (as in
`move/move.go`):

```go
// New<Cmd>Cmd returns a command that uses db for all operations (for testing).
func New<Cmd>Cmd(db <cmd>DB) *cobra.Command { ... }

// NewDefault<Cmd>Cmd opens the database from context (production entry point).
func NewDefault<Cmd>Cmd() *cobra.Command { ... }
```

The singleton `var <cmd>Cmd *cobra.Command` pattern used in most packages must
be dropped, because it prevents injection.

#### C. Update the root command registration

**`/home/grue/dev/wherehouse/cmd/root.go`** (or wherever subcommands are added):
Replace `GetFoundCmd()` → `NewDefaultFoundCmd()`, etc. for each migrated command.

#### D. Delete `helpers.go` files that are passthrough-only

After migration to the interface pattern, the one-liner wrappers in
`add/helpers.go`, `list/helpers.go`, `lost/helpers.go`, `loan/helpers.go`,
`history/resolver.go`, and the inline wrapper in `find/find.go:347` should be
removed. Each package will call `cli.OpenDatabase` directly inside
`NewDefault*Cmd`'s RunE, exactly as `move/move.go` does.

Exception: `move/helpers.go` already follows the interface pattern; keep it but
review whether `resolveItemSelector` and `resolveLocation` still need wrapping.

#### E. Generate mocks

After each interface file is created, run:
```
//go:generate mockery
```
This produces `mocks/Mock<Cmd>DB.go` in each package.

### File-Level Changes Per Command

| Command | Create | Modify | Delete |
|---------|--------|--------|--------|
| `found` | `cmd/found/db.go`, `cmd/found/mocks/` | `cmd/found/found.go` | none |
| `loan`  | `cmd/loan/db.go`, `cmd/loan/mocks/`   | `cmd/loan/item.go`, `cmd/loan/loan.go` | `cmd/loan/helpers.go` |
| `lost`  | `cmd/lost/db.go`, `cmd/lost/mocks/`   | `cmd/lost/item.go`, `cmd/lost/lost.go` | `cmd/lost/helpers.go` |
| `add`   | `cmd/add/db.go`, `cmd/add/mocks/`     | `cmd/add/item.go`, `cmd/add/location.go`, `cmd/add/add.go` | `cmd/add/helpers.go` |
| `list`  | `cmd/list/db.go`, `cmd/list/mocks/`   | `cmd/list/list.go` | `cmd/list/helpers.go` |
| `history` | `cmd/history/db.go`, `cmd/history/mocks/` | `cmd/history/history.go` | `cmd/history/resolver.go` |
| `find`  | `cmd/find/db.go`, `cmd/find/mocks/`   | `cmd/find/find.go` | `cmd/find/find.go:347` (inline wrapper removed) |
| `scry`  | `cmd/scry/db.go`, `cmd/scry/mocks/`   | `cmd/scry/scry.go` | none |

### `found` Interface (largest; shown as reference)

```go
type foundDB interface {
    Close() error
    GetItem(ctx context.Context, itemID string) (*database.Item, error)
    GetLocation(ctx context.Context, locationID string) (*database.Location, error)
    GetLocationByCanonicalName(ctx context.Context, canonicalName string) (*database.Location, error)
    GetItemsByCanonicalName(ctx context.Context, canonicalName string) ([]*database.Item, error)
    GetSystemLocationIDs(ctx context.Context) (missingID, borrowedID, loanedID string, err error)
    ValidateFromLocation(ctx context.Context, itemID, expectedFromLocationID string) error
    AppendEvent(ctx context.Context, eventType database.EventType, actorUserID string, payload any, note string) (int64, error)
}
```

### `history` Interface

```go
type historyDB interface {
    Close() error
    GetItem(ctx context.Context, itemID string) (*database.Item, error)
    GetLocation(ctx context.Context, locationID string) (*database.Location, error)
    GetLocationByCanonicalName(ctx context.Context, canonicalName string) (*database.Location, error)
    GetItemsByCanonicalName(ctx context.Context, canonicalName string) ([]*database.Item, error)
    GetEventsByEntity(ctx context.Context, itemID, locationID, projectID *string, opts ...any) ([]*database.Event, error)
}
```

### `find` Interface

```go
type findDB interface {
    Close() error
    SearchByName(ctx context.Context, name string, limit int) ([]*database.SearchResult, error)
    GetItemLoanedInfo(ctx context.Context, itemID string) (*database.LoanedInfo, error)
}
```

### `scry` Interface

```go
type scryDB interface {
    Close() error
    GetItem(ctx context.Context, itemID string) (*database.Item, error)
    GetLocation(ctx context.Context, locationID string) (*database.Location, error)
    GetLocationByCanonicalName(ctx context.Context, canonicalName string) (*database.Location, error)
    GetItemsByCanonicalName(ctx context.Context, canonicalName string) ([]*database.Item, error)
    GetSystemLocationIDs(ctx context.Context) (missingID, borrowedID, loanedID string, err error)
    ScryItem(ctx context.Context, item *database.Item) (*database.ScryResult, error)
}
```

### `list` Interface

Examine `cmd/list/list.go` for the exact DB methods called; construct the
interface to include only those. The package-level `testOpenDatabase` and
`testMustGetConfig` hooks must be removed and replaced by injection.

### Root Command Integration

```go
// cmd/root.go or cmd/wherehouse/main.go — update registrations:
rootCmd.AddCommand(found.NewDefaultFoundCmd())   // was found.GetFoundCmd()
rootCmd.AddCommand(loan.NewDefaultLoanCmd())
rootCmd.AddCommand(lost.NewDefaultLostCmd())
rootCmd.AddCommand(list.NewDefaultListCmd())
rootCmd.AddCommand(history.NewDefaultHistoryCmd())
rootCmd.AddCommand(find.NewDefaultFindCmd())
rootCmd.AddCommand(scry.NewDefaultScryCmd())
// add.GetAddCmd() remains; its subcommands (item, location) follow same pattern
```

### Tests to Write (golang-tester)

After this step, each package has a `New<Cmd>Cmd(mock)` path. Test examples:

- `cmd/found/found_test.go`: inject `MockFoundDB`, test state-validation logic
- `cmd/lost/item_test.go`: refactor existing tests from direct-function to
  command-level using mock
- `cmd/loan/item_test.go`: same

---

## Step 3: Adopt OutputWriter in the Four Bypassing Commands

**Addresses**: Review issues 1.1, 1.2, 1.3, 1.4 (MEDIUM/LOW)
**Depends on**: Step 2 (commands have injectable DB interfaces by this point)
**Parallelizable with**: Step 4

### Commands

- `cmd/find/find.go`
- `cmd/scry/scry.go`
- `cmd/history/output.go`
- `cmd/initialize/database.go`

### Pattern

Each command creates an `OutputWriter` using `cli.NewOutputWriterFromConfig` and
delegates all output through it. The duplicated `json.NewEncoder` setup is
eliminated; calls route to `out.JSON(data)`.

### `find/find.go`

**Current**: `outputJSON` creates its own encoder and writes to `io.Writer`
directly. `outputHuman` writes via `fmt.Fprintf`.

**After**:
1. `runFind` creates `out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)`.
2. `outputJSON` is removed as a standalone function. Its logic moves inline into
   `runFind`'s JSON branch: build the `jsonOutput` struct, then call `out.JSON(jsonOutput)`.
3. `outputHuman` signature changes from `(w io.Writer, ...)` to `(out *cli.OutputWriter, ...)`.
   Internal `fmt.Fprintf(w, ...)` calls become `out.Println(...)` or `out.Print(...)`.
   The `--quiet` flag suppression now comes for free because `out.Println` respects
   quiet mode when called through the `Print`/`Println` path.

Note: `find` uses `out.Print`/`out.Println` (not `out.Success`) for tabular output
because the raw line-by-line format is not a simple success message. `OutputWriter.Println`
does bypass quiet mode (as documented in output.go:134), so the review's concern that
`-q` is silently ignored will be partially addressed. If full quiet-mode suppression
of find results is desired, that is a product decision flagged in gaps.json.

**Signature changes**:
```go
// Before:
func outputJSON(w io.Writer, results []*database.SearchResult, searchTerm string, loanedInfoMap map[string]*database.LoanedInfo) error

// After: inlined into runFind; no standalone function needed
// OR if kept separate:
func outputJSON(out *cli.OutputWriter, results []*database.SearchResult, searchTerm string, loanedInfoMap map[string]*database.LoanedInfo) error

// Before:
func outputHuman(w io.Writer, results []*database.SearchResult, verbose bool, loanedInfoMap map[string]*database.LoanedInfo)

// After:
func outputHuman(out *cli.OutputWriter, results []*database.SearchResult, verbose bool, loanedInfoMap map[string]*database.LoanedInfo)
```

### `scry/scry.go`

Same pattern as find:
1. Create `out` in `runScry`.
2. Replace `outputJSON(cmd.OutOrStdout(), result)` with `out.JSON(scryOutput)`.
3. Replace `outputHuman(cmd.OutOrStdout(), result, verbose)` with `outputHuman(out, result, verbose)`.
4. Replace all `fmt.Fprintf(w, ...)` calls in `outputHuman` and helpers with `out.Println(...)`.

```go
// Before:
func outputHuman(w io.Writer, result *database.ScryResult, verbose bool)
func printScoredCategory(w io.Writer, ...)
func printSimilarItemCategory(w io.Writer, ...)
func printLabeledRow(w io.Writer, ...)
func printContinuationRow(w io.Writer, ...)

// After: replace io.Writer parameter with *cli.OutputWriter in each
```

### `history/output.go`

The history command has a more complex output function (`formatHuman`) that uses
`styles.DefaultStyles()` directly (review issue 1.3). Changes:

1. `formatOutput` receives `out *cli.OutputWriter` instead of reading `jsonMode bool`.
2. `formatJSON` is simplified to call `out.JSON(jsonHistoryOutput)` — eliminates
   the `json.NewEncoder` at line 47.
3. `formatHuman` and `formatEvent` keep their `io.Writer` parameter from the
   internal `out.out` field, OR accept `*cli.OutputWriter` and use `out.Print`.
4. Remove `import "github.com/asphaltbuffet/wherehouse/internal/styles"` from
   `output.go`. The `appStyles` local variable at line 112 of `formatEvent` should
   remain within `formatEvent` for now (the styles singleton is still needed for
   event-type-specific coloring not available through `OutputWriter`'s fixed methods).

```go
// Before:
func formatOutput(ctx context.Context, cmd *cobra.Command, db *database.Database, events []*database.Event, jsonMode bool) error

// After:
func formatOutput(ctx context.Context, out *cli.OutputWriter, db historyDB, events []*database.Event) error
```

Note: `formatHuman` still needs `io.Writer` access for the timeline rendering
(event-specific lipgloss styles per event type). Pass `cmd.OutOrStdout()` through
alongside `out`, or add a `Writer()` accessor to `OutputWriter`. The cleanest
approach is to add a package-private `Writer() io.Writer` method to `OutputWriter`:

```go
// internal/cli/output.go — add:
func (w *OutputWriter) Writer() io.Writer { return w.out }
```

This lets `formatEvent` call `fmt.Fprintf(out.Writer(), ...)` for the timeline
lines while still using `out.JSON(...)` for JSON mode.

### `initialize/database.go`

Replace `printInitResult` entirely:

```go
// Before:
func printInitResult(cmd *cobra.Command, cfg *config.Config, dbPath, backupPath string) error {
    if cfg.IsJSON() {
        enc := json.NewEncoder(cmd.OutOrStdout())
        enc.SetIndent("", "  ")
        return enc.Encode(initResult{...})
    }
    if cfg.IsQuiet() { return nil }
    fmt.Fprintf(cmd.OutOrStdout(), "Database initialized at %s\n", dbPath)
    ...
}

// After:
func printInitResult(cmd *cobra.Command, cfg *config.Config, dbPath, backupPath string) error {
    out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)
    if cfg.IsJSON() {
        return out.JSON(initResult{Status: "initialized", Path: dbPath, BackupPath: backupPath})
    }
    if backupPath != "" {
        out.Info(fmt.Sprintf("Backed up existing database to %s", backupPath))
    }
    out.Success(fmt.Sprintf("Database initialized at %s", dbPath))
    return nil
}
```

Remove `import "encoding/json"` from `initialize/database.go` after this change
(it is the standard library `encoding/json`; the project uses `github.com/goccy/go-json`
elsewhere — verify which is appropriate, see gaps.json).

---

## Step 4: Replace Inline Relative-Time Logic in `history`

**Addresses**: Review issue 1.3 (MEDIUM) — duplicated `formatRelativeTime`
**Depends on**: Step 2
**Parallelizable with**: Step 3

### File to Modify

**`/home/grue/dev/wherehouse/cmd/history/output.go`**

### Changes

The `formatRelativeTime(d time.Duration)` function at lines 194-217 reimplements
what `internal/cli/time_format.go:FormatRelativeTime(t time.Time)` already
provides via `go-humanize`.

The `formatTimestamp` function at lines 175-191 currently:
1. Parses a UTC string to `time.Time`
2. Computes `diff := now.Sub(t)`
3. For `diff < 7 days`: calls `formatRelativeTime(diff)` (custom, duration-based)
4. For `diff >= 7 days`: formats as `"2006-01-02 15:04"` (absolute)

**After**:
1. Delete `formatRelativeTime(d time.Duration)` entirely.
2. Modify `formatTimestamp` to call `cli.FormatRelativeTime(t)` for the relative
   branch, removing the custom duration-based implementation.
3. Keep the 7-day threshold logic in `formatTimestamp` for the absolute fallback
   (this is additional behavior beyond what `humanize.Time` alone provides — see
   gaps.json for the question of whether this threshold should be parameterized).

```go
// Before:
func formatTimestamp(timestampUTC string) string {
    t, err := time.Parse(time.RFC3339, timestampUTC)
    if err != nil { return timestampUTC }
    now := time.Now()
    diff := now.Sub(t)
    if diff < recentDaysThreshold*hoursPerDay*time.Hour && diff > 0 {
        return formatRelativeTime(diff) // custom implementation
    }
    return t.Format("2006-01-02 15:04")
}

// After:
func formatTimestamp(timestampUTC string) string {
    t, err := time.Parse(time.RFC3339, timestampUTC)
    if err != nil { return timestampUTC }
    diff := time.Since(t)
    if diff > 0 && diff < recentDaysThreshold*hoursPerDay*time.Hour {
        return cli.FormatRelativeTime(t) // delegates to go-humanize
    }
    return t.Format("2006-01-02 15:04")
}

// Delete:
func formatRelativeTime(d time.Duration) string { ... }

// Also delete unused constants if no longer referenced:
// const hoursPerDay = 24          -- keep if formatTimestamp still uses it
// const recentDaysThreshold = 7   -- keep if formatTimestamp still uses it
```

**Import cleanup**: Remove `"time"` usage of `time.Duration` subtype; keep `time`
import since `time.Parse`, `time.Since`, `time.Hour` are still used.

Add import: `"github.com/asphaltbuffet/wherehouse/internal/cli"` (already imported
in `history.go` but verify it is also imported in `output.go`).

---

## Step 5: Normalize Help Text Structure

**Addresses**: Review issues 4.1 (MEDIUM), 4.2 (LOW), 4.3 (LOW)
**Depends on**: None (purely cosmetic text changes)
**Can run in parallel with**: All other steps

### Sub-step 5a: Capitalize `migrate` Short Descriptions

**Files**:
- `/home/grue/dev/wherehouse/cmd/migrate/migrate.go`, line 15
- `/home/grue/dev/wherehouse/cmd/migrate/database.go`, line 25

```go
// migrate/migrate.go:
Short: "Run data migration operations",  // was "run data migration operations"

// migrate/database.go:
Short: "Migrate database IDs from UUID to nanoid format",  // was lowercase
```

### Sub-step 5b: Fix Example-Comment Style in `config`

**File**: `/home/grue/dev/wherehouse/cmd/config/config.go`

The `Long` description currently uses tab-aligned descriptions without `#` comments.
Convert to `#`-comment style to match `find`, `add location`, and `scry`:

```
// Before:
  wherehouse config init              Create global config file
  wherehouse config get               Show all configuration values

// After:
  wherehouse config init              # Create global config file
  wherehouse config get               # Show all configuration values
```

Apply the same conversion to any other `config` subcommand help text that uses
the non-`#` style.

### Sub-step 5c: Fix Column Alignment in `add` Parent Command

**File**: `/home/grue/dev/wherehouse/cmd/add/add.go`, lines 22-23

```go
// Before (misaligned):
Long: `Add new items and locations.

Examples:
  wherehouse add location <name> --in <location> Add a new location
  wherehouse add item <name> --in <location>              Add a new item`,

// After (aligned with # style):
Long: `Add new items and locations.

Examples:
  wherehouse add location <name> --in <location>  # Add a new location
  wherehouse add item <name> --in <location>       # Add a new item`,
```

Use consistent spacing so the `#` column aligns between the two example lines.

---

## Parallelization Summary

| Work stream | Steps | Blocking? |
|-------------|-------|-----------|
| Stream A    | 1 → 2 → (3, 4 simultaneously) | 2 blocks 3 and 4 |
| Stream B    | 5 (any time)                   | None |

**Recommended execution order**:
1. Start Step 1 and Step 5 in parallel (different agents, no overlap in files).
2. After Step 1 completes, start Step 2.
3. After Step 2 completes, start Steps 3 and 4 in parallel.

---

## Review Issue Coverage

| Review Issue | Severity | Step | Resolution |
|--------------|----------|------|------------|
| 1.1 `initialize` bypasses OutputWriter | MEDIUM | 3 | `printInitResult` replaced |
| 1.2 `find`, `scry` bypass OutputWriter | MEDIUM | 3 | OutputWriter adopted in both |
| 1.3 `history` bypasses OutputWriter, duplicates relative-time | MEDIUM | 3+4 | Both addressed |
| 1.4 Duplicated JSON encoder boilerplate | LOW | 3 | All 5 instances eliminated |
| 2.1 `add/location` business logic inline | HIGH | 1 | Extracted to `cli.AddLocations` |
| 2.2 `found` business logic inline | MEDIUM | 2 | Interface injection; extraction deferred (see gaps) |
| 2.3 `lost` business logic inline | MEDIUM | 2 | Interface injection |
| 2.4 `loan` business logic inline | MEDIUM | 2 | Interface injection |
| 2.5 `internal/cli/migrate.go` scope | LOW | Deferred | Out of scope for this refactoring |
| 3.1 Inconsistent testability patterns | HIGH | 2 | move-pattern applied uniformly |
| 3.2 Duplicated helpers.go wrappers | MEDIUM | 2 | Wrappers deleted after injection |
| 4.1 Inconsistent example comment style | MEDIUM | 5 | `#` style applied uniformly |
| 4.2 `migrate` lowercase Short | LOW | 5 | Capitalized |
| 4.3 `add` column misalignment | LOW | 5 | Fixed |
