# Final Implementation Plan: cmd/ vs internal/cli/ Consistency Refactoring

**Date**: 2026-03-03
**Session**: 20260303-205954
**Supersedes**: `initial-plan.md`
**Clarifications applied**: `clarifications.md`

---

## Changes From Initial Plan

| Area | Initial | Final |
|------|---------|-------|
| find/scry quiet mode | Use `out.Print`/`out.Println`; quiet flag noted as product question | Use `out.Println` — results always print; quiet only suppresses info/warnings. No ambiguity. |
| history timestamps | Keep 7-day threshold; call `cli.FormatRelativeTime` for relative branch | Remove ALL inline logic and the threshold entirely. Call `cli.FormatRelativeTime` unconditionally. `go-humanize` handles transitions naturally. |
| found/loan/lost logic | Interface injection only (DI); extraction deferred | Extract to `internal/cli/` NOW (found.go, loan.go, lost.go). Same pattern as `cli.AddLocations`. |
| `OutputWriter.Writer()` | Noted as "cleanest approach" but left optional | Required: add `Writer() io.Writer` to `OutputWriter` in `internal/cli/output.go`. |

---

## Dependency Graph (Updated)

```
Step 0 (OutputWriter.Writer accessor) — internal/cli/ only, no cmd/ deps
  └── (unblocks Step 3 history work)

Step 1a (cli.AddLocations)           — internal/cli/locations.go
Step 1b (cli.FoundItem)              — internal/cli/found.go      [parallel with 1a, 1c]
Step 1c (cli.LoanItem / cli.LostItem)— internal/cli/operations.go [parallel with 1a, 1b]
  └── All of Step 1 must complete before Step 2

Step 2 (standardize DI across all commands — db.go + New*Cmd constructors)
  └── Step 3 (OutputWriter in find/scry/history/initialize) [parallelizable]
  └── Step 4 (FormatRelativeTime in history — simplified)   [parallelizable with 3]

Step 5 (normalize help text) — independent, no blocking dependencies
```

---

## Step 0: Add `Writer()` Accessor to OutputWriter

**Addresses**: Clarification — small surface addition required before history output work
**Depends on**: Nothing
**Blocks**: Step 3 (history subwork)
**Agent**: `golang-developer`

### File to Modify

**`/home/grue/dev/wherehouse/internal/cli/output.go`**

Add one exported method to `OutputWriter`:

```go
// Writer returns the underlying io.Writer used for human-readable output.
// Use this when third-party renderers (e.g. lipgloss) need a raw writer
// but the caller must stay coupled to the same output sink OutputWriter controls.
func (w *OutputWriter) Writer() io.Writer {
    return w.out
}
```

Placement: after the existing constructor / before the output methods.

No new imports required. The `w.out` field is already `io.Writer`.

### Verification

- Compile-time: `go build ./internal/cli/...`
- The field name `out` must match the actual struct field in `OutputWriter`. If the field is named differently (e.g. `writer`), adjust accordingly.

---

## Step 1: Extract Domain Logic to `internal/cli/`

**Addresses**: Review issues 2.1, 2.2, 2.3, 2.4; Clarification (found/loan/lost extraction now)
**Depends on**: Nothing (pure library additions)
**All sub-steps are parallelizable with each other**

### Step 1a: `cli.AddLocations`

**Agent**: `golang-developer`

#### File to Create

**`/home/grue/dev/wherehouse/internal/cli/locations.go`**

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
func AddLocations(ctx context.Context, names []string, parentName string) ([]AddLocationResult, error)
```

Full body: as specified in `initial-plan.md` Step 1. No changes from initial plan.

#### File to Modify

**`/home/grue/dev/wherehouse/cmd/add/location.go`**

Replace `runAddLocation` body with thin wrapper (~20 lines):
1. Read `--in` flag
2. Call `cli.AddLocations(ctx, args, parentInput)`
3. For each result: call `out.Success(...)` via `OutputWriter`
4. Remove inline domain logic
5. Remove unused imports: `database`, `nanoid`

---

### Step 1b: `cli.FoundItem`

**Agent**: `golang-developer`

**Motivation**: `cmd/found/found.go:foundItem` contains ~115 lines of domain logic: state checks, home-location fallback, conditional return-to-home flow, event construction. Extracting to `internal/cli/` makes it testable and reusable from TUI/API layers.

#### File to Create

**`/home/grue/dev/wherehouse/internal/cli/found.go`**

```go
package cli

import (
    "context"
    "fmt"

    "github.com/asphaltbuffet/wherehouse/internal/database"
)

// FoundItemResult holds the outcome of marking an item as found.
type FoundItemResult struct {
    ItemID          string
    DisplayName     string
    NewLocationID   string
    NewLocationName string
    ReturnedHome    bool // true if item was also returned to home location
}

// FoundItemOptions configures optional behavior for FoundItem.
type FoundItemOptions struct {
    // LocationName is the name/selector of the location where the item was found.
    // If empty, the item's home location is used (if set).
    LocationName string
    // Note is an optional free-text note attached to the event.
    Note string
}

// FoundItem marks an item as found at the specified (or home) location.
// Validates current item state before creating the LocationFound event.
// If the item has a home location and was returned there, ReturnedHome is set in the result.
// db must satisfy the foundDB interface (same methods as cmd/found/db.go).
func FoundItem(ctx context.Context, db foundDB, itemSelector string, opts FoundItemOptions) (*FoundItemResult, error)
```

The `foundDB` interface is defined in this file (package `cli`):

```go
// foundDB is the database interface required by FoundItem.
// *database.Database satisfies this interface.
type foundDB interface {
    GetItem(ctx context.Context, itemID string) (*database.Item, error)
    GetLocation(ctx context.Context, locationID string) (*database.Location, error)
    GetLocationByCanonicalName(ctx context.Context, canonicalName string) (*database.Location, error)
    GetItemsByCanonicalName(ctx context.Context, canonicalName string) ([]*database.Item, error)
    GetSystemLocationIDs(ctx context.Context) (missingID, borrowedID, loanedID string, err error)
    ValidateFromLocation(ctx context.Context, itemID, expectedFromLocationID string) error
    AppendEvent(ctx context.Context, eventType database.EventType, actorUserID string, payload any, note string) (int64, error)
}
```

Body: extract the logic from `cmd/found/found.go:foundItem` (lines 127-242), adapting to accept `foundDB` instead of `*database.Database`.

#### File to Modify

**`/home/grue/dev/wherehouse/cmd/found/found.go`**

Replace `foundItem` body with thin wrapper:
1. Accept the `foundDB` interface (injected via `NewFoundCmd`)
2. Call `cli.FoundItem(ctx, db, itemSelector, opts)`
3. Format output via `OutputWriter`
4. Remove all inline domain logic
5. Remove unused imports

---

### Step 1c: `cli.LoanItem` and `cli.LostItem`

**Agent**: `golang-developer`

**Motivation**: `cmd/loan/item.go:loanItem` (~63 lines) and `cmd/lost/item.go:markItemLost` (~53 lines) contain domain logic that should live in `internal/cli/` for parity with `cli.AddLocations` and `cli.FoundItem`.

#### File to Create

**`/home/grue/dev/wherehouse/internal/cli/loan.go`**

```go
package cli

import (
    "context"

    "github.com/asphaltbuffet/wherehouse/internal/database"
)

// LoanItemResult holds the outcome of loaning an item.
type LoanItemResult struct {
    ItemID      string
    DisplayName string
    LoanedTo    string
}

// LoanItemOptions configures optional behavior for LoanItem.
type LoanItemOptions struct {
    // Borrower is the name of the person/entity borrowing the item. Required.
    Borrower string
    // Note is an optional free-text note attached to the event.
    Note string
}

// loanDB is the database interface required by LoanItem.
// *database.Database satisfies this interface.
type loanDB interface {
    GetItem(ctx context.Context, itemID string) (*database.Item, error)
    GetLocation(ctx context.Context, locationID string) (*database.Location, error)
    GetItemsByCanonicalName(ctx context.Context, canonicalName string) ([]*database.Item, error)
    GetSystemLocationIDs(ctx context.Context) (missingID, borrowedID, loanedID string, err error)
    AppendEvent(ctx context.Context, eventType database.EventType, actorUserID string, payload any, note string) (int64, error)
}

// LoanItem records an item as loaned to a borrower.
// Validates item state (must not already be loaned/missing) before creating the ItemLoaned event.
func LoanItem(ctx context.Context, db loanDB, itemSelector string, opts LoanItemOptions) (*LoanItemResult, error)
```

#### File to Create

**`/home/grue/dev/wherehouse/internal/cli/lost.go`**

```go
package cli

import (
    "context"

    "github.com/asphaltbuffet/wherehouse/internal/database"
)

// LostItemResult holds the outcome of marking an item as lost.
type LostItemResult struct {
    ItemID      string
    DisplayName string
    FromLocation string
}

// LostItemOptions configures optional behavior for LostItem.
type LostItemOptions struct {
    // FromLocationName is the location the item was lost from.
    // If empty, the item's current location is used.
    FromLocationName string
    // Note is an optional free-text note attached to the event.
    Note string
}

// lostDB is the database interface required by LostItem.
// *database.Database satisfies this interface.
type lostDB interface {
    GetItem(ctx context.Context, itemID string) (*database.Item, error)
    GetLocation(ctx context.Context, locationID string) (*database.Location, error)
    GetItemsByCanonicalName(ctx context.Context, canonicalName string) ([]*database.Item, error)
    GetSystemLocationIDs(ctx context.Context) (missingID, borrowedID, loanedID string, err error)
    ValidateFromLocation(ctx context.Context, itemID, expectedFromLocationID string) error
    AppendEvent(ctx context.Context, eventType database.EventType, actorUserID string, payload any, note string) (int64, error)
}

// LostItem marks an item as lost (moved to the system Missing location).
// Validates item state (must not already be missing) before creating the ItemLost event.
func LostItem(ctx context.Context, db lostDB, itemSelector string, opts LostItemOptions) (*LostItemResult, error)
```

#### Files to Modify

**`/home/grue/dev/wherehouse/cmd/loan/item.go`**
- Replace `loanItem` body with thin call to `cli.LoanItem(ctx, db, selector, opts)`
- Remove all inline domain logic

**`/home/grue/dev/wherehouse/cmd/lost/item.go`**
- Replace `markItemLost` body with thin call to `cli.LostItem(ctx, db, selector, opts)`
- Remove all inline domain logic

**Note on interface visibility**: The `foundDB`, `loanDB`, and `lostDB` interfaces in `internal/cli/` are unexported (package-private). Each `cmd/` package's own `db.go` interface (created in Step 2) is independently defined and may have a slightly different name/shape. The `*database.Database` concrete type satisfies both implicitly. There is intentionally no forced embedding relationship.

---

## Step 2: Standardize Dependency Injection Across All Commands

**Addresses**: Review issues 3.1 (HIGH), 3.2 (MEDIUM)
**Depends on**: Step 1 (all sub-steps)
**Blocks**: Steps 3 and 4
**Agent**: `golang-ui-developer`

### Pattern (mirror `cmd/move/`)

For each command package:

**A. Create `<cmd>/db.go`** — minimal interface file with `//go:generate mockery` directive.

**B. Update primary command file** — replace `Get*Cmd()` singleton with:
```go
func New<Cmd>Cmd(db <cmd>DB) *cobra.Command { ... }
func NewDefault<Cmd>Cmd() *cobra.Command { ... }  // opens DB from context
```

**C. Update root command registration** in `cmd/root.go` — replace `GetFoundCmd()` etc. with `NewDefaultFoundCmd()`.

**D. Delete passthrough `helpers.go` files** — after DI adoption, one-liner wrappers add no value.

**E. Generate mocks** — `go generate ./cmd/<cmd>/...` in each package.

### File-Level Changes Per Command

| Command | Create | Modify | Delete |
|---------|--------|--------|--------|
| `found` | `cmd/found/db.go`, `cmd/found/mocks/MockFoundDB.go` | `cmd/found/found.go` | none |
| `loan`  | `cmd/loan/db.go`, `cmd/loan/mocks/MockLoanDB.go` | `cmd/loan/item.go`, `cmd/loan/loan.go` | `cmd/loan/helpers.go` |
| `lost`  | `cmd/lost/db.go`, `cmd/lost/mocks/MockLostDB.go` | `cmd/lost/item.go`, `cmd/lost/lost.go` | `cmd/lost/helpers.go` |
| `add`   | `cmd/add/db.go`, `cmd/add/mocks/MockAddDB.go` | `cmd/add/item.go`, `cmd/add/location.go`, `cmd/add/add.go` | `cmd/add/helpers.go` |
| `list`  | `cmd/list/db.go`, `cmd/list/mocks/MockListDB.go` | `cmd/list/list.go` | `cmd/list/helpers.go` |
| `history` | `cmd/history/db.go`, `cmd/history/mocks/MockHistoryDB.go` | `cmd/history/history.go` | `cmd/history/resolver.go` |
| `find`  | `cmd/find/db.go`, `cmd/find/mocks/MockFindDB.go` | `cmd/find/find.go` | (inline wrapper at find.go:347 removed) |
| `scry`  | `cmd/scry/db.go`, `cmd/scry/mocks/MockScryDB.go` | `cmd/scry/scry.go` | none |

### Interface Definitions

#### `cmd/found/db.go`
```go
package found

import (
    "context"
    "github.com/asphaltbuffet/wherehouse/internal/database"
)

//go:generate mockery
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

#### `cmd/history/db.go`
```go
package history

//go:generate mockery
type historyDB interface {
    Close() error
    GetItem(ctx context.Context, itemID string) (*database.Item, error)
    GetLocation(ctx context.Context, locationID string) (*database.Location, error)
    GetLocationByCanonicalName(ctx context.Context, canonicalName string) (*database.Location, error)
    GetItemsByCanonicalName(ctx context.Context, canonicalName string) ([]*database.Item, error)
    GetEventsByEntity(ctx context.Context, itemID, locationID, projectID *string, opts ...any) ([]*database.Event, error)
}
```

#### `cmd/find/db.go`
```go
package find

//go:generate mockery
type findDB interface {
    Close() error
    SearchByName(ctx context.Context, name string, limit int) ([]*database.SearchResult, error)
    GetItemLoanedInfo(ctx context.Context, itemID string) (*database.LoanedInfo, error)
}
```

#### `cmd/scry/db.go`
```go
package scry

//go:generate mockery
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

#### `cmd/list/db.go`
Examine `cmd/list/list.go` for exact DB methods called. Remove package-level `testOpenDatabase` and `testMustGetConfig` hooks; replace with interface injection.

### Root Command Integration

**`/home/grue/dev/wherehouse/cmd/root.go`** (or equivalent registration file):
```go
rootCmd.AddCommand(found.NewDefaultFoundCmd())   // was found.GetFoundCmd()
rootCmd.AddCommand(loan.NewDefaultLoanCmd())
rootCmd.AddCommand(lost.NewDefaultLostCmd())
rootCmd.AddCommand(list.NewDefaultListCmd())
rootCmd.AddCommand(history.NewDefaultHistoryCmd())
rootCmd.AddCommand(find.NewDefaultFindCmd())
rootCmd.AddCommand(scry.NewDefaultScryCmd())
// add retains Get*Cmd() at parent level; subcommands follow same pattern
```

---

## Step 3: Adopt OutputWriter in the Four Bypassing Commands

**Addresses**: Review issues 1.1, 1.2, 1.3, 1.4
**Depends on**: Step 2 (injectable DB), Step 0 (`Writer()` accessor for history)
**Parallelizable with**: Step 4
**Agent**: `golang-ui-developer`

### `cmd/find/find.go`

**Clarification applied**: Results always print via `out.Println`. The `-q` flag suppresses info/warnings only; tabular results are never suppressed.

**Signature changes**:
```go
// Before:
func outputJSON(w io.Writer, results []*database.SearchResult, searchTerm string, loanedInfoMap map[string]*database.LoanedInfo) error
func outputHuman(w io.Writer, results []*database.SearchResult, verbose bool, loanedInfoMap map[string]*database.LoanedInfo)

// After:
// outputJSON inlined into runFind's JSON branch — no standalone function
// OR if kept for readability:
func outputJSON(out *cli.OutputWriter, results []*database.SearchResult, searchTerm string, loanedInfoMap map[string]*database.LoanedInfo) error

func outputHuman(out *cli.OutputWriter, results []*database.SearchResult, verbose bool, loanedInfoMap map[string]*database.LoanedInfo)
```

**Changes**:
1. `runFind` creates `out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)`
2. All result output uses `out.Println(...)` — results always appear regardless of `-q`
3. Info/warning messages use `out.Info(...)` / `out.Warning(...)` — suppressed by `-q`
4. `outputJSON` inlined or refactored to accept `*cli.OutputWriter`; calls `out.JSON(jsonOutput)`
5. Remove standalone `json.NewEncoder` setup

### `cmd/scry/scry.go`

**Clarification applied**: Same as find — results via `out.Println`, info/warnings via `out.Info`.

**Signature changes**:
```go
// Before:
func outputHuman(w io.Writer, result *database.ScryResult, verbose bool)
func printScoredCategory(w io.Writer, ...)
func printSimilarItemCategory(w io.Writer, ...)
func printLabeledRow(w io.Writer, ...)
func printContinuationRow(w io.Writer, ...)

// After: replace io.Writer with *cli.OutputWriter in each signature
func outputHuman(out *cli.OutputWriter, result *database.ScryResult, verbose bool)
func printScoredCategory(out *cli.OutputWriter, ...)
func printSimilarItemCategory(out *cli.OutputWriter, ...)
func printLabeledRow(out *cli.OutputWriter, ...)
func printContinuationRow(out *cli.OutputWriter, ...)
```

**Changes**:
1. `runScry` creates `out := cli.NewOutputWriterFromConfig(...)`
2. `out.JSON(scryOutput)` replaces standalone encoder
3. All result lines use `out.Println(...)` (always visible)
4. Info messages use `out.Info(...)` (quiet-suppressible)

### `cmd/history/output.go`

**Requires**: Step 0 (`Writer()` accessor), Step 4 (`formatTimestamp` simplification)

**Signature changes**:
```go
// Before:
func formatOutput(ctx context.Context, cmd *cobra.Command, db *database.Database, events []*database.Event, jsonMode bool) error

// After:
func formatOutput(ctx context.Context, out *cli.OutputWriter, db historyDB, events []*database.Event) error
```

**Changes**:
1. `formatOutput` receives `*cli.OutputWriter` — callers no longer pass `jsonMode bool`
2. `formatJSON` simplified: `out.JSON(jsonHistoryOutput)` — removes json.NewEncoder at line 47
3. `formatHuman` and `formatEvent` use `fmt.Fprintf(out.Writer(), ...)` for lipgloss-styled timeline lines (uses the `Writer()` accessor from Step 0)
4. Remove `import "github.com/asphaltbuffet/wherehouse/internal/styles"` from `output.go` if `appStyles` can be removed; otherwise keep it scoped to `formatEvent` only (event-type-specific coloring not available through `OutputWriter`'s fixed methods)

### `cmd/initialize/database.go`

**Signature unchanged** — `printInitResult(cmd *cobra.Command, cfg *config.Config, dbPath, backupPath string) error`

**Body replacement**:
```go
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

Remove `import "encoding/json"` (or `github.com/goccy/go-json` — verify which is used) after this change.

---

## Step 4: Replace ALL Inline Timestamp Logic in `history`

**Addresses**: Review issue 1.3 — duplicated `formatRelativeTime`
**Clarification applied**: Remove ALL inline logic including the 7-day threshold. Call `cli.FormatRelativeTime` unconditionally. `go-humanize` handles transitions naturally (e.g. "3 months ago").
**Depends on**: Step 2
**Parallelizable with**: Step 3
**Agent**: `golang-ui-developer`

### File to Modify

**`/home/grue/dev/wherehouse/cmd/history/output.go`**

### Changes

**Delete entirely**:
- `formatRelativeTime(d time.Duration) string` (lines 194-217) — replaced by `cli.FormatRelativeTime`
- The 7-day threshold constants (`recentDaysThreshold`, `hoursPerDay`) if no longer referenced after simplification

**Replace `formatTimestamp`**:
```go
// Before:
func formatTimestamp(timestampUTC string) string {
    t, err := time.Parse(time.RFC3339, timestampUTC)
    if err != nil { return timestampUTC }
    now := time.Now()
    diff := now.Sub(t)
    if diff < recentDaysThreshold*hoursPerDay*time.Hour && diff > 0 {
        return formatRelativeTime(diff) // custom duration-based implementation
    }
    return t.Format("2006-01-02 15:04")
}

// After:
func formatTimestamp(timestampUTC string) string {
    t, err := time.Parse(time.RFC3339, timestampUTC)
    if err != nil { return timestampUTC }
    return cli.FormatRelativeTime(t)
}
```

The absolute date format (`"2006-01-02 15:04"`) is removed. `go-humanize` produces appropriate human strings at all time distances ("just now", "3 minutes ago", "2 days ago", "3 months ago", "2 years ago").

**Import cleanup**:
- Add `"github.com/asphaltbuffet/wherehouse/internal/cli"` if not already present in `output.go`
- Remove any `time.Duration` usage that was only needed for the deleted `formatRelativeTime`
- Keep `"time"` import for `time.Parse`, `time.RFC3339`

---

## Step 5: Normalize Help Text Structure

**Addresses**: Review issues 4.1 (MEDIUM), 4.2 (LOW), 4.3 (LOW)
**Depends on**: Nothing — purely cosmetic text changes
**Parallelizable with**: All other steps
**Agent**: `golang-ui-developer`

### Sub-step 5a: Capitalize `migrate` Short Descriptions

**Files to modify**:
- `/home/grue/dev/wherehouse/cmd/migrate/migrate.go`, line 15:
  `Short: "Run data migration operations",`
- `/home/grue/dev/wherehouse/cmd/migrate/database.go`, line 25:
  `Short: "Migrate database IDs from UUID to nanoid format",`

### Sub-step 5b: Fix Example-Comment Style in `config`

**File to modify**: `/home/grue/dev/wherehouse/cmd/config/config.go`

Convert `Long` description examples from tab-aligned-without-`#` to `#`-comment style:
```
// Before:
  wherehouse config init              Create global config file

// After:
  wherehouse config init              # Create global config file
```

Apply to all `config` subcommand help text using the non-`#` style.

### Sub-step 5c: Fix Column Alignment in `add` Parent Command

**File to modify**: `/home/grue/dev/wherehouse/cmd/add/add.go`, lines 22-23

```go
// After (aligned with # style):
Long: `Add new items and locations.

Examples:
  wherehouse add location <name> --in <location>  # Add a new location
  wherehouse add item <name> --in <location>       # Add a new item`,
```

Use consistent spacing so the `#` column aligns between both example lines.

---

## Parallelization Summary

| Stream | Steps | Notes |
|--------|-------|-------|
| Stream A | Step 0 | No deps; unblocks Step 3 history work |
| Stream B | Steps 1a, 1b, 1c in parallel | All are internal/cli/ additions; no shared files |
| Stream C | Step 5 | Fully independent; help text only |
| (wait for 0, 1a/b/c) | Step 2 | DI standardization across all cmd/ |
| (after Step 2) | Steps 3 and 4 in parallel | OutputWriter adoption + timestamp cleanup |

**Recommended execution**:
1. Start Step 0, Steps 1a/1b/1c, and Step 5 all in parallel (different files/agents, no overlap)
2. After all of Step 1 completes, start Step 2
3. After Step 2 completes, start Steps 3 and 4 in parallel

---

## Agent Assignments

| Step | Agent | Rationale |
|------|-------|-----------|
| Step 0 | `golang-developer` | `internal/cli/output.go` |
| Step 1a | `golang-developer` (new file) + `golang-ui-developer` (cmd/add/) | Cross-boundary; split by file |
| Step 1b | `golang-developer` (new file) + `golang-ui-developer` (cmd/found/) | Cross-boundary; split by file |
| Step 1c | `golang-developer` (new files) + `golang-ui-developer` (cmd/loan/, cmd/lost/) | Cross-boundary; split by file |
| Step 2 | `golang-ui-developer` | All changes in `cmd/` |
| Step 3 | `golang-ui-developer` | All changes in `cmd/` |
| Step 4 | `golang-ui-developer` | All changes in `cmd/history/` |
| Step 5 | `golang-ui-developer` | All changes in `cmd/` |

---

## Review Issue Coverage

| Review Issue | Severity | Step | Resolution |
|--------------|----------|------|------------|
| 1.1 `initialize` bypasses OutputWriter | MEDIUM | 3 | `printInitResult` replaced |
| 1.2 `find`, `scry` bypass OutputWriter | MEDIUM | 3 | OutputWriter adopted; results via `out.Println` (always visible) |
| 1.3 `history` bypasses OutputWriter, duplicates relative-time | MEDIUM | 3+4 | Both addressed; threshold removed entirely |
| 1.4 Duplicated JSON encoder boilerplate | LOW | 3 | All 5 instances eliminated via `out.JSON()` |
| 2.1 `add/location` business logic inline | HIGH | 1a | Extracted to `cli.AddLocations` |
| 2.2 `found` business logic inline | MEDIUM | 1b | Extracted to `cli.FoundItem` (not deferred) |
| 2.3 `lost` business logic inline | MEDIUM | 1c | Extracted to `cli.LostItem` (not deferred) |
| 2.4 `loan` business logic inline | MEDIUM | 1c | Extracted to `cli.LoanItem` (not deferred) |
| 2.5 `internal/cli/migrate.go` scope | LOW | Deferred | Out of scope for this refactoring |
| 3.1 Inconsistent testability patterns | HIGH | 2 | move-pattern applied uniformly |
| 3.2 Duplicated helpers.go wrappers | MEDIUM | 2 | Wrappers deleted after interface injection |
| 4.1 Inconsistent example comment style | MEDIUM | 5 | `#` style applied uniformly |
| 4.2 `migrate` lowercase Short | LOW | 5 | Capitalized |
| 4.3 `add` column misalignment | LOW | 5 | Fixed |

---

## Files Created or Modified (Complete Index)

### New Files

| File | Step | Agent |
|------|------|-------|
| `/home/grue/dev/wherehouse/internal/cli/locations.go` | 1a | `golang-developer` |
| `/home/grue/dev/wherehouse/internal/cli/found.go` | 1b | `golang-developer` |
| `/home/grue/dev/wherehouse/internal/cli/loan.go` | 1c | `golang-developer` |
| `/home/grue/dev/wherehouse/internal/cli/lost.go` | 1c | `golang-developer` |
| `/home/grue/dev/wherehouse/cmd/found/db.go` | 2 | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/loan/db.go` | 2 | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/lost/db.go` | 2 | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/add/db.go` | 2 | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/list/db.go` | 2 | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/history/db.go` | 2 | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/find/db.go` | 2 | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/scry/db.go` | 2 | `golang-ui-developer` |
| `cmd/*/mocks/Mock*DB.go` (8 files, generated) | 2 | `golang-ui-developer` |

### Modified Files

| File | Step | Agent |
|------|------|-------|
| `/home/grue/dev/wherehouse/internal/cli/output.go` | 0 | `golang-developer` |
| `/home/grue/dev/wherehouse/cmd/add/location.go` | 1a | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/found/found.go` | 1b+2 | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/loan/item.go` | 1c+2 | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/loan/loan.go` | 2 | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/lost/item.go` | 1c+2 | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/lost/lost.go` | 2 | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/add/item.go` | 2 | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/add/add.go` | 2+5c | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/list/list.go` | 2 | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/history/history.go` | 2 | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/history/output.go` | 3+4 | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/find/find.go` | 2+3 | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/scry/scry.go` | 2+3 | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/initialize/database.go` | 3 | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/migrate/migrate.go` | 5a | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/migrate/database.go` | 5a | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/config/config.go` | 5b | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/root.go` | 2 | `golang-ui-developer` |

### Deleted Files

| File | Step | Agent |
|------|------|-------|
| `/home/grue/dev/wherehouse/cmd/add/helpers.go` | 2 | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/list/helpers.go` | 2 | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/lost/helpers.go` | 2 | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/loan/helpers.go` | 2 | `golang-ui-developer` |
| `/home/grue/dev/wherehouse/cmd/history/resolver.go` | 2 | `golang-ui-developer` |
