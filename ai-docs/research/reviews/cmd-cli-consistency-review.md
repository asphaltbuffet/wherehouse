# Code Review: cmd/ vs internal/cli/ Consistency

**Date**: 2026-03-03
**Scope**: All `cmd/` subpackages against `internal/cli/` support layer
**Linting**: 0 issues (clean pass)

---

## Strengths

- **Well-designed OutputWriter**: `internal/cli/output.go` provides a solid, consistent abstraction for Success/Error/Warning/Info/KeyValue/JSON output. Commands that use it (config/*, add/location, loan, lost, move) get quiet-mode and JSON-mode handling for free.
- **Shared selector resolution**: `internal/cli/selectors.go` provides `ResolveItemSelector` and `ResolveLocation` with proper error messages and ambiguity handling. Every command that needs item/location resolution delegates to these.
- **Shared database open**: `cli.OpenDatabase` is used consistently via thin wrappers in each command package.
- **move command**: Exemplary testability -- defines a `moveDB` interface, uses `mockery` for generated mocks, and accepts the interface in `NewMoveCmd`. This is the gold standard in the codebase.
- **Help text quality**: Most commands have detailed `Long` descriptions with examples, selector documentation, and validation rules. The loan and lost packages have particularly thorough `doc.go` files.
- **Event-sourcing correctness**: Commands properly validate from_location before creating events, use atomic `AppendEvent` calls, and include required payload fields.

---

## Category 1: Output Formatting

### 1.1 MEDIUM -- `initialize/database.go` bypasses OutputWriter entirely

**File**: `/home/grue/dev/wherehouse/cmd/initialize/database.go`, lines 164-189
**Issue**: `printInitResult` manually checks `cfg.IsJSON()` and `cfg.IsQuiet()`, uses raw `fmt.Fprintf` for human output, and creates its own `json.NewEncoder`. Every other command that adopted OutputWriter gets quiet/JSON behavior automatically.

```go
// Current (lines 164-189):
func printInitResult(cmd *cobra.Command, cfg *config.Config, dbPath, backupPath string) error {
    if cfg.IsJSON() {
        // manual JSON encoding
        enc := json.NewEncoder(cmd.OutOrStdout())
        enc.SetIndent("", "  ")
        return enc.Encode(result)
    }
    if cfg.IsQuiet() {
        return nil
    }
    fmt.Fprintf(cmd.OutOrStdout(), "Database initialized at %s\n", dbPath)
    ...
}
```

**Contrast with the pattern used in `cmd/config/init.go` (lines 86-92)**:
```go
out.Success("Configuration file created")
out.KeyValue("Path", expandedPath)
```

### 1.2 MEDIUM -- `find/find.go` and `scry/scry.go` bypass OutputWriter for all output

**File**: `/home/grue/dev/wherehouse/cmd/find/find.go`, lines 89-95
**File**: `/home/grue/dev/wherehouse/cmd/scry/scry.go`, lines 87-96
**Issue**: Both commands use raw `fmt.Fprintf(cmd.OutOrStdout(), ...)` and build their own JSON encoders. Neither creates an `OutputWriter`. This means:
- No quiet-mode support (the `-q` flag is silently ignored)
- No styled output via `appStyles`
- JSON encoding is duplicated (indented encoder pattern appears in `find/find.go:284`, `scry/scry.go:290`, `history/output.go:47`, `list/list.go:89`, `initialize/database.go:172`)

### 1.3 MEDIUM -- `history/output.go` bypasses OutputWriter; duplicates relative time formatting

**File**: `/home/grue/dev/wherehouse/cmd/history/output.go`, lines 194-217
**Issue**: The `formatRelativeTime` function in `history/output.go` reimplements relative-time formatting. Meanwhile, `internal/cli/time_format.go` already provides `FormatRelativeTime` using `go-humanize`. The history command also uses its own `formatTimestamp` that applies a 7-day threshold, which could be a shared helper.

Additionally, `history/output.go` creates its own JSON encoder (line 47) and uses `styles.DefaultStyles()` directly (line 112) rather than going through OutputWriter.

### 1.4 LOW -- Duplicated JSON encoder boilerplate

Five separate files create `json.NewEncoder(w)` + `enc.SetIndent("", "  ")`:
- `/home/grue/dev/wherehouse/cmd/find/find.go:284`
- `/home/grue/dev/wherehouse/cmd/history/output.go:47`
- `/home/grue/dev/wherehouse/cmd/list/list.go:89`
- `/home/grue/dev/wherehouse/cmd/scry/scry.go:290`
- `/home/grue/dev/wherehouse/cmd/initialize/database.go:172`

`OutputWriter.JSON()` already handles this pattern. Commands that bypass it are duplicating encoding setup.

---

## Category 2: Business Logic in CLI Layer

### 2.1 HIGH -- `add/location.go` has full business logic inline

**File**: `/home/grue/dev/wherehouse/cmd/add/location.go`, lines 45-130
**Issue**: `runAddLocation` performs name validation, canonicalization, uniqueness checks, ID generation, payload construction, and event creation directly in the CLI command handler. This is 85 lines of domain logic in the CLI layer. Compare with `add/item.go` which properly delegates to `cli.AddItems` (only 12 lines in RunE).

The asymmetry is notable: `add item` delegates to `internal/cli/add.go`, but `add location` does everything inline. The location creation logic should be extracted to a shared helper (like `cli.AddLocations`) to match the item pattern and allow testing without cobra.

### 2.2 MEDIUM -- `found/found.go` has ~115 lines of domain logic inline

**File**: `/home/grue/dev/wherehouse/cmd/found/found.go`, lines 127-242
**Issue**: `foundItem` contains significant business logic: state checks, home-location fallback logic, conditional return-to-home flow with its own validation and event creation. This is domain logic that would benefit from extraction to a service layer, both for testability and reuse.

### 2.3 MEDIUM -- `lost/item.go` has domain logic inline

**File**: `/home/grue/dev/wherehouse/cmd/lost/item.go`, lines 70-122
**Issue**: `markItemLost` performs state validation (already-missing check), from-location validation, payload construction, and event creation. While shorter than `found`, it follows the same pattern of embedding domain logic in the CLI layer.

### 2.4 MEDIUM -- `loan/item.go` has domain logic inline

**File**: `/home/grue/dev/wherehouse/cmd/loan/item.go`, lines 132-194
**Issue**: `loanItem` performs item state lookup, re-loan detection, projection validation, payload construction, and event creation inline. Same pattern as lost and found.

### 2.5 LOW -- `internal/cli/migrate.go` contains heavy data-migration logic

**File**: `/home/grue/dev/wherehouse/internal/cli/migrate.go` (297 lines)
**Issue**: This file contains raw SQL execution (`tx.ExecContext` with UPDATE statements), string replacement of event payloads, and full migration orchestration. While it lives in `internal/cli`, this is database-layer work that arguably belongs in `internal/database/` or a dedicated `internal/migration/` package. The `cli` package doc says it provides "reusable helpers for CLI command implementations" -- migration logic goes beyond that scope.

---

## Category 3: Dependency Mocking for Testing

### 3.1 HIGH -- Inconsistent testability patterns across commands

Three distinct patterns exist for dependency injection:

**Pattern A -- Interface + mockery (BEST)**: `move/mover.go` defines `moveDB` interface, uses `//go:generate mockery`. Tests in `move/item_test.go` can use either real DB or mock. Only the `move` command uses this pattern.

**Pattern B -- Package-level test hooks (FRAGILE)**: `list/list.go` (lines 19-21) uses package-level `var testOpenDatabase` and `var testMustGetConfig` that get overridden in tests. This approach is fragile (global state, must remember to reset, not thread-safe) and verbose (every test must set and defer-clear the hooks).

**Pattern C -- No injection at all (UNTESTABLE at command level)**: `find`, `found`, `loan`, `lost`, `scry`, `history`, `add/location` all call `cli.OpenDatabase` directly in their RunE functions. The only way to test their command handlers is with a real database. Some of these packages (e.g., `lost/item_test.go`) work around this by testing the extracted core function (`markItemLost`) directly with a real in-memory database, but the RunE handler itself remains untestable in isolation.

**Commands without any RunE-level tests**: `find`, `scry`, `history` have no tests that exercise the command handler. `found` only tests the singleton pattern.

### 3.2 MEDIUM -- Duplicated helpers.go wrapper pattern

Seven packages define nearly identical `helpers.go` files that wrap `cli.OpenDatabase` and `cli.ResolveItemSelector`:

- `/home/grue/dev/wherehouse/cmd/add/helpers.go` (2 one-liner wrappers)
- `/home/grue/dev/wherehouse/cmd/list/helpers.go` (2 wrappers, one with test hook)
- `/home/grue/dev/wherehouse/cmd/lost/helpers.go` (2 wrappers)
- `/home/grue/dev/wherehouse/cmd/loan/helpers.go` (2 wrappers)
- `/home/grue/dev/wherehouse/cmd/move/helpers.go` (3 wrappers)
- `/home/grue/dev/wherehouse/cmd/history/resolver.go` (1 wrapper)
- `/home/grue/dev/wherehouse/cmd/find/find.go:347` (1 wrapper, inline at bottom)

These wrappers add no value beyond indirection. Most exist solely as potential injection points but are not actually injectable (no interface, no test hook). Only `list/helpers.go` has the test-hook pattern, and only `move` uses a proper interface.

---

## Category 4: Consistent Help Text

### 4.1 MEDIUM -- Inconsistent Example formatting in Long descriptions

**Alignment issue**: Some commands use labeled examples (`# comment` style), others do not.

Good (consistent):
```
# find/find.go:
  wherehouse find screwdriver          # Find all screwdrivers
  wherehouse find toolbox              # Find toolbox location
```

Missing comments:
```
# config/config.go:
  wherehouse config init              Create global config file
  wherehouse config get               Show all configuration values
```

The `config` subcommands use a tab-aligned description without `#`, while `find`, `add location`, `scry` use `#`-comments. Neither is wrong, but consistency would improve readability.

### 4.2 LOW -- `migrate` uses lowercase Short description

**File**: `/home/grue/dev/wherehouse/cmd/migrate/migrate.go`, line 14
```go
Short: "run data migration operations",
```

**File**: `/home/grue/dev/wherehouse/cmd/migrate/database.go`, line 25
```go
Short: "migrate database IDs from UUID to nanoid format",
```

Every other command capitalizes the Short description ("Add items and locations", "Find items or locations by name", "Show event history for an item"). The `migrate` commands break this convention.

### 4.3 LOW -- `add location` example has misaligned columns

**File**: `/home/grue/dev/wherehouse/cmd/add/add.go`, lines 22-23
```
  wherehouse add location <name> --in <location> Add a new location
  wherehouse add item <name> --in <location>              Add a new item
```

The description alignment is inconsistent between the two lines. The item line has excess padding while the location line has no gap before its description.

---

## Summary

```
Assessment: Needs Changes

Critical: 0 issues
High: 2 issues (business logic in add/location, inconsistent testability)
Medium: 9 issues
Low: 4 issues

Key concern: Three different testability patterns (interface mocks, global hooks,
  no injection) create an inconsistent testing story. The move command shows the
  right approach but no other command follows it.

Risk: Medium
Testability Score: Fair (move=Good, list=Fair, others=Poor at command level)
```

---

## Prioritized Recommendations

### Priority 1: Extract `add location` business logic to `internal/cli/`

Mirror the `cli.AddItems` pattern with a `cli.AddLocations` function. This:
- Eliminates the asymmetry between `add item` and `add location`
- Makes location creation testable without cobra
- Removes ~70 lines of domain logic from the CLI layer

### Priority 2: Standardize testability on interface pattern

Define a narrow database interface in each command package (like `move/mover.go`) and accept it as a parameter. This eliminates the fragile global-var hooks in `list` and makes every command testable in isolation. Start with the commands that have the most business logic: `found`, `loan`, `lost`.

### Priority 3: Adopt OutputWriter in find, scry, history, initialize

These four commands bypass OutputWriter entirely. Converting them would:
- Add quiet-mode support for free
- Centralize JSON encoding (eliminate 5 duplicate encoder setups)
- Apply consistent styling via `appStyles`

### Priority 4: Eliminate one-liner helpers.go wrappers

Commands that don't use injection (add, find, loan, lost, history) should call `cli.OpenDatabase` and `cli.ResolveItemSelector` directly instead of through passthrough wrappers that add no value. When a command later needs injection, it should adopt the interface pattern (Priority 2) rather than the wrapper pattern.

### Priority 5: Extract domain logic from found, lost, loan

Move the core business logic (item state validation, event payload construction, conditional follow-up events) into domain-level functions in `internal/cli/` or a new `internal/commands/` package. The CLI RunE handlers should be thin: parse flags, open DB, call domain function, format output.

### Priority 6: Consolidate duplicate relative-time formatting

Replace `history/output.go:formatRelativeTime` with `cli.FormatRelativeTime` (which uses `go-humanize`). If the history command needs the 7-day threshold behavior, extend the `cli` helper with an option rather than reimplementing.

### Priority 7: Normalize help text conventions

- Capitalize all `Short` descriptions (fix `migrate` commands)
- Pick one example-comment style and apply it everywhere
- Fix column alignment in `add` parent command
