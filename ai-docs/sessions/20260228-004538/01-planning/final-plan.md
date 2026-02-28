# Final Implementation Plan: cmd/move Test Refactoring & Singleton Removal

**Session**: 20260228-004538
**Date**: 2026-02-28
**Scope**: Remove singleton `GetMoveCmd()`, introduce constructor functions, add flag-wiring and output tests using mockery v3, fix linting violations in existing tests.

---

## 0. Architecture Findings (Investigation Results)

### 0.1 Database Layer

`internal/database` exposes a concrete `*database.Database` struct — **no interface exists**. All DB calls in `cmd/move` go through methods on this struct. The full method surface used by `cmd/move` (directly and via `internal/cli`) is:

| Method | Called by |
|---|---|
| `GetItem(ctx, itemID)` | `moveItem()` and `cli.ResolveItemSelector` |
| `GetLocation(ctx, locationID)` | `moveItem()`, `validateDestinationNotSystem()`, `cli.ResolveLocation`, `cli.buildAmbiguousItemError` |
| `GetLocationByCanonicalName(ctx, canonicalName)` | `cli.ResolveLocation`, `cli.resolveLocationItemSelector` |
| `GetItemsByCanonicalName(ctx, canonicalName)` | `cli.resolveLocationItemSelector`, `cli.resolveItemByCanonicalName` |
| `ValidateFromLocation(ctx, itemID, fromLocationID)` | `moveItem()` |
| `ValidateProjectExists(ctx, projectID, status)` | `runMoveItem()` |
| `AppendEvent(ctx, type, actor, payload, note)` | `moveItem()` |
| `Close()` | `runMoveItem()` (deferred) |

### 0.2 openDatabase and Context

`cli.OpenDatabase(ctx)` pulls `*config.Config` from `context.WithValue` (key `config.ConfigKey`), resolves the path, then calls `os.Stat` to verify the file exists before calling `database.Open()`. **This means `:memory:` paths fail the `CheckDatabaseExists` guard without workaround.** For CLI-layer wiring tests, the only clean option is to avoid `openDatabase` entirely — which the mockery approach achieves by injecting the DB dependency directly.

### 0.3 Existing Mockery Setup

- `.mockery.yaml` exists at the project root with mockery v3 configuration (`with-expecter: true`, `issue-845-fix: true`)
- One existing mock: `internal/logging/mocks/mock_logger.go` (for the `Logger` interface)
- Pattern: interface lives in source package; mock generated to `{InterfaceDir}/mocks/`

### 0.4 Mockery Injection Architecture Decision

**Where does the interface live?** `cmd/move` package as a local unexported interface. Rationale:
- The interface is narrow (only what `cmd/move` needs — including all methods used transitively via `internal/cli` selectors)
- Placing it in `internal/database` would make a wide interface just for CLI testing purposes
- Go's implicit interface satisfaction means `*database.Database` satisfies the interface automatically without any changes to `internal/database`
- Same-package test files (`wiring_test.go` in `package move`) can use the unexported interface and the generated mock

**Interface name**: `mover` (unexported) or `moveDB` — reflects exactly what the move command needs from the database.

**How is the mock injected?** Via the `NewMoveCmd(db moveDB)` constructor. The production path `NewDefaultMoveCmd()` calls `NewMoveCmd(nil)` with a database-opening function... wait — this approach requires rethinking the constructor shape.

**Revised injection architecture:**

The core problem: `runMoveItem` currently calls `openDatabase(ctx)` at the top. With mock injection, we need the DB to be _provided_ not _opened_. Two options:

**Option A**: `NewMoveCmd(db moveDB)` — inject DB directly; `NewDefaultMoveCmd()` becomes a function that opens DB then builds the cmd (not idiomatic for cobra).

**Option B**: `NewMoveCmd(openDB func(ctx context.Context) (moveDB, error))` — inject the opener function. Production uses `openDatabase`; tests provide a func that returns the mock.

**Option C (preferred)**: Separate `openDB` from the constructor entirely. Keep `runMoveItemWith(cmd, args, openDB)` where `openDB` is a `func(context.Context) (moveDB, error)`. `NewMoveCmd` and `NewDefaultMoveCmd` handle the wiring.

Option C is cleanest because:
- `NewMoveCmd(fn openDBFunc)` — the seam is at DB-open time, not at construction time
- Tests provide a `func(ctx) (moveDB, error)` that returns the mock immediately
- No need to thread the mock through the command struct
- The mock is created in the test, wrapped in a one-liner opener function

**Final interface definition (in `cmd/move/mover.go` or top of `helpers.go`):**

```go
// moveDB is the database interface required by the move command.
// *database.Database satisfies this interface.
type moveDB interface {
    Close() error
    GetItem(ctx context.Context, itemID string) (*database.Item, error)
    GetLocation(ctx context.Context, locationID string) (*database.Location, error)
    GetLocationByCanonicalName(ctx context.Context, canonicalName string) (*database.Location, error)
    GetItemsByCanonicalName(ctx context.Context, canonicalName string) ([]*database.Item, error)
    ValidateFromLocation(ctx context.Context, itemID, expectedFromLocationID string) error
    ValidateProjectExists(ctx context.Context, projectID string, requiredStatus *string) error
    AppendEvent(ctx context.Context, eventType, actorUserID string, payload map[string]any, note string) (int64, error)
}
```

**Mock generation**: Add to `.mockery.yaml`:
```yaml
github.com/asphaltbuffet/wherehouse/cmd/move:
  interfaces:
    moveDB:
```

This places the generated mock at `cmd/move/mocks/mock_movedb.go`.

**Note on `internal/cli` functions**: `cli.ResolveLocation` and `cli.ResolveItemSelector` take `*database.Database` not an interface. Two options:
1. Change `internal/cli` to accept the interface — but that's a wider change requiring review
2. **Wrap the resolvers in `cmd/move` local functions that take `moveDB`** — already done (`resolveLocation` and `resolveItemSelector` in `helpers.go` wrap `cli.ResolveLocation`/`cli.ResolveItemSelector`, but they currently pass `*database.Database`)

**Resolution**: The `cmd/move` local wrappers `resolveLocation(ctx, db moveDB, input)` and `resolveItemSelector(ctx, db moveDB, selector)` must be updated to accept `moveDB` instead of `*database.Database`. Since the `cli.*` functions take `*database.Database`, we have two paths:

- Path 1: Update `internal/cli` selectors to use an interface (correct long-term, but out of scope)
- Path 2: In wiring tests, skip the selector layer by having the mock return immediately on `GetItem`/`GetLocationByCanonicalName` — this is fine because `resolveLocation` and `resolveItemSelector` only call DB methods also in the `moveDB` interface

**Path 2 is correct for this scope.** The `cmd/move` wrappers currently accept `*database.Database` but the underlying `cli.*` calls all go through interface-compatible methods. We change the wrapper signatures from `*database.Database` → `moveDB`, and `internal/cli` functions take `*database.Database`. This creates a compile error.

**Correct resolution**: The `cmd/move` package local wrappers must **not** delegate to `cli.ResolveLocation`/`cli.ResolveItemSelector` directly if we want to pass `moveDB`. Instead:

**Option A**: Move the resolver wrapper bodies inline — but that duplicates logic.

**Option B**: Update `internal/cli` resolver signatures to accept a `database.Querier` interface — out of scope.

**Option C (simplest)**: Keep `*database.Database` in `resolveLocation`/`resolveItemSelector` helpers but change the `runMoveItemWith` internals to perform a type assertion. Ugly.

**Option D (correct)**: The `openDB func` approach means the wiring tests bypass `resolveLocation`/`resolveItemSelector` entirely by choosing what to test. For **wiring tests** (flag-to-parameter mapping), we still need resolution to work. The mock must satisfy `*database.Database`-accepting functions.

**Final decision**: Do NOT change `resolveLocation`/`resolveItemSelector` signatures. Instead, the `openDB func(ctx) (moveDB, error)` returns a `*MockMoveDB` from the test, but `resolveLocation` and `resolveItemSelector` accept `*database.Database`. This is the type mismatch.

**The cleanest architectural solution**: Change the internal helpers to accept `moveDB` and avoid calling `cli.ResolveLocation`/`cli.ResolveItemSelector`. Instead, inline the resolution logic using `moveDB` methods directly. `cli.ResolveLocation` is only ~20 lines; `cli.ResolveItemSelector` is ~25 lines. The wrappers already exist in `helpers.go` — move the logic there, parameterized on `moveDB`.

However, this duplicates logic from `internal/cli`. Better approach:

**Update `internal/cli` selectors to accept an interface** — or define a `LocationQuerier` / `ItemQuerier` interface in `internal/cli` and have the functions accept it. This is the architecturally correct approach and `*database.Database` satisfies it implicitly. This is a one-time change that benefits all future CLI commands.

Since this adds scope, the plan will note it as a **prerequisite step**: define a `Querier` interface in `internal/cli` used by `ResolveLocation`/`ResolveItemSelector`. Then `cmd/move`'s `moveDB` embeds this interface implicitly.

**Scope boundary for this session**: Keep it minimal. The wiring tests will use the `openDB` func injection and inject a mock that satisfies both `moveDB` AND the `*database.Database` requirement of `cli.*`. Since the mock IS-A `*MockMoveDB` (not `*database.Database`), this cannot work without changing `internal/cli`.

**Final final decision — pragmatic approach**:

Use the `openDB func` injection for `runMoveItemWith`, where the `openDB` returns `*database.Database`. For wiring tests that need a mock, **use a real in-memory SQLite via a temp file** for the resolver layer, but inject a spy/mock at the `moveItem` call boundary. This is the hybrid approach:

- `runMoveItemWith(cmd, args, openDB func(ctx) (*database.Database, error), fn moveItemFunc)` — two injected dependencies
- `openDB` provides the database (tests use temp file); `fn` is the moveItemFunc spy
- Wiring tests use real SQLite for resolution (2 locations, 1 item), spy for the actual move execution
- This avoids the interface/type-mismatch problem entirely
- `CheckDatabaseExists` is bypassed by using a temp file path (file exists on disk)

This aligns with the user's preference: **mockery v3 for the mock, but the mock replaces `moveItem` (the business logic), not the resolution layer.** The resolution layer runs against real SQLite because the `cli.*` functions are already well-tested there.

**Wait — re-reading the clarification:**

> Keep `moveItem()` tested with real in-memory SQLite (where database behavior is relevant)
> Flag-wiring / output-routing tests → use mock (tests CLI layer only)

The clarification says flag-wiring tests should use a mock to AVOID seeding. The intention is: for tests that verify `--temp` sets `moveType="temporary_use"`, we don't want to open a database at all. The mock should let the test say "I don't care about DB details, just verify the flag reached the function call."

This requires that `runMoveItemWith` can operate without opening a real database. The only way to do this cleanly is to inject the DB (or an interface) at construction time, not open it from context.

**Architecturally sound final design:**

```
NewMoveCmd(db moveDB) *cobra.Command
NewDefaultMoveCmd()   *cobra.Command  // opens DB from context inside RunE
```

But `NewDefaultMoveCmd` can't open the DB at construction time — it must open per-invocation. So `NewMoveCmd(db moveDB)` must receive a DB that's already open, meaning the production path has to open the DB before constructing the command. This inverts cobra's typical lifecycle.

**The true solution**: Change `RunE` to accept `db moveDB` via closure. The seam is at `RunE` definition time:

```go
func NewMoveCmd(db moveDB) *cobra.Command {
    cmd := &cobra.Command{
        RunE: func(cmd *cobra.Command, args []string) error {
            return runMoveItemCore(cmd, args, db)
        },
    }
    // flags...
    return cmd
}

func NewDefaultMoveCmd() *cobra.Command {
    return &cobra.Command{
        RunE: func(cmd *cobra.Command, args []string) error {
            db, err := openDatabase(cmd.Context())
            if err != nil { return fmt.Errorf("...") }
            defer db.Close()
            return runMoveItemCore(cmd, args, db)
        },
    }
}
```

For production: `NewDefaultMoveCmd()` opens DB each invocation (same as current behavior).
For tests: `NewMoveCmd(mockDB)` — mock is pre-constructed, no DB open needed.

`runMoveItemCore` takes `moveDB` interface and uses it everywhere — including passing to `resolveLocation`/`resolveItemSelector`.

The `cli.ResolveLocation`/`cli.ResolveItemSelector` problem remains. **For this session, we accept the following**: Change `internal/cli` selectors to accept a `cli.LocationItemQuerier` interface. This is a clean, small change confined to 2 function signatures and their callers.

---

## 1. Overview of Changes

### Files Modified (in dependency order)

| Order | File | Type of Change |
|---|---|---|
| 1 | `internal/cli/selectors.go` | Accept `LocationItemQuerier` interface instead of `*database.Database` |
| 2 | `internal/cli/selectors_test.go` | No change needed (tests use `*database.Database` which satisfies interface) |
| 3 | `cmd/move/mover.go` | NEW: `moveDB` interface + `LocationItemQuerier` embedding |
| 4 | `cmd/move/helpers.go` | Change `*database.Database` → `moveDB` in resolver wrappers |
| 5 | `cmd/move/item.go` | Change `*database.Database` → `moveDB`; extract `runMoveItemCore`; add `moveItemFunc` |
| 6 | `cmd/move/move.go` | Remove singleton; add `NewMoveCmd(db moveDB)` and `NewDefaultMoveCmd()` |
| 7 | `cmd/root.go` | `GetMoveCmd()` → `NewDefaultMoveCmd()` |
| 8 | `.mockery.yaml` | Add `moveDB` interface to generation config |
| 9 | `cmd/move/mocks/mock_movedb.go` | GENERATED: Run `go generate` or `mockery` |
| 10 | `cmd/move/item_test.go` | Fix linting violations; update `TestGetMoveCmd_Structure` |
| 11 | `cmd/move/helpers_test.go` | Fix violations; reduce table cases |
| 12 | `cmd/move/wiring_test.go` | NEW: flag-wiring + output tests using `MockMoveDB` |

---

## 2. Production Code Changes

### 2.1 `internal/cli/selectors.go` — Accept Interface

Define a `LocationItemQuerier` interface in `internal/cli` covering only the methods `ResolveLocation` and `ResolveItemSelector` need:

```go
// LocationItemQuerier is the database query interface required by resolver functions.
// *database.Database satisfies this interface.
type LocationItemQuerier interface {
    GetLocation(ctx context.Context, locationID string) (*database.Location, error)
    GetLocationByCanonicalName(ctx context.Context, canonicalName string) (*database.Location, error)
    GetItem(ctx context.Context, itemID string) (*database.Item, error)
    GetItemsByCanonicalName(ctx context.Context, canonicalName string) ([]*database.Item, error)
}
```

Change `ResolveLocation(ctx, db *database.Database, input)` → `ResolveLocation(ctx, db LocationItemQuerier, input)`.
Change `ResolveItemSelector(ctx, db *database.Database, ...)` → `ResolveItemSelector(ctx, db LocationItemQuerier, ...)`.

All internal helpers (`resolveLocationItemSelector`, `resolveItemByCanonicalName`, `buildAmbiguousItemError`) change to `LocationItemQuerier` accordingly. Since `*database.Database` satisfies `LocationItemQuerier`, all callers compile unchanged.

### 2.2 `cmd/move/mover.go` — New File: Interface Definition

```go
package move

import (
    "context"

    "github.com/asphaltbuffet/wherehouse/internal/database"
)

// moveDB is the database interface required by the move command.
// *database.Database satisfies this interface implicitly.
//
//go:generate mockery --name=moveDB
type moveDB interface {
    Close() error
    GetItem(ctx context.Context, itemID string) (*database.Item, error)
    GetLocation(ctx context.Context, locationID string) (*database.Location, error)
    GetLocationByCanonicalName(ctx context.Context, canonicalName string) (*database.Location, error)
    GetItemsByCanonicalName(ctx context.Context, canonicalName string) ([]*database.Item, error)
    ValidateFromLocation(ctx context.Context, itemID, expectedFromLocationID string) error
    ValidateProjectExists(ctx context.Context, projectID string, requiredStatus *string) error
    AppendEvent(ctx context.Context, eventType, actorUserID string, payload map[string]any, note string) (int64, error)
}
```

**Note**: `*database.Database` satisfies `moveDB` because all methods exist on the concrete struct. No changes to `internal/database` are needed.

### 2.3 `cmd/move/helpers.go` — Update Signatures

Change `resolveLocation` and `resolveItemSelector` to accept `moveDB`:

```go
func resolveLocation(ctx context.Context, db moveDB, input string) (string, error) {
    return cli.ResolveLocation(ctx, db, input)
}

func resolveItemSelector(ctx context.Context, db moveDB, selector string) (string, error) {
    return cli.ResolveItemSelector(ctx, db, selector, "wherehouse move")
}
```

This compiles because `moveDB` embeds `LocationItemQuerier` (all 4 querier methods are in `moveDB`), and `cli.ResolveLocation` now accepts `LocationItemQuerier`. Go's structural typing handles this.

### 2.4 `cmd/move/item.go` — Refactor for Interface + Constructor Seam

**Key changes:**
1. Replace `*database.Database` with `moveDB` everywhere
2. Extract `runMoveItemCore(cmd, args, db moveDB) error` from `runMoveItem`
3. Keep `moveItem` unexported, change its `db` param to `moveDB`
4. Remove `runMoveItem` (replaced by `runMoveItemCore`)

```go
// runMoveItemCore is the main entry point for the move command.
// db must be open; the caller is responsible for defer db.Close().
func runMoveItemCore(cmd *cobra.Command, args []string, db moveDB) error {
    ctx := cmd.Context()

    // Parse flags
    toLocation, _ := cmd.Flags().GetString("to")
    temp, _ := cmd.Flags().GetBool("temp")
    projectID, _ := cmd.Flags().GetString("project")
    keepProject, _ := cmd.Flags().GetBool("keep-project")
    note, _ := cmd.Flags().GetString("note")

    // Get actor user ID
    actorUserID := cli.GetActorUserID(ctx)

    // Resolve destination location once (shared across all moves)
    toLocationID, err := resolveLocation(ctx, db, toLocation)
    if err != nil {
        return fmt.Errorf("destination location not found: %w", err)
    }

    // Validate destination is not a system location
    if sysErr := validateDestinationNotSystem(ctx, db, toLocationID); sysErr != nil {
        return sysErr
    }

    // Validate project if specified
    if projectID != "" {
        activeStatus := "active"
        if projErr := db.ValidateProjectExists(ctx, projectID, &activeStatus); projErr != nil {
            return fmt.Errorf("project validation failed: %w", projErr)
        }
    }

    // Determine move type and project action
    moveType := determineMoveType(temp)
    projectAction := determineProjectAction(projectID, keepProject)

    // Set up output writer
    cfg := cli.MustGetConfig(ctx)
    out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

    // Process each item selector in order (fail-fast)
    var results []Result

    for _, selector := range args {
        itemID, itemErr := resolveItemSelector(ctx, db, selector)
        if itemErr != nil {
            return fmt.Errorf("failed to resolve %q: %w", selector, itemErr)
        }

        result, moveErr := moveItem(
            ctx, db, itemID, toLocationID,
            moveType, projectAction, projectID, actorUserID, note,
        )
        if moveErr != nil {
            return fmt.Errorf("failed to move %q: %w", selector, moveErr)
        }

        results = append(results, *result)

        if !cfg.IsJSON() {
            out.Success(fmt.Sprintf("Moved item %q from %s to %s",
                result.DisplayName, result.FromLocation, result.ToLocation))
        }
    }

    if cfg.IsJSON() {
        output := map[string]any{"moved": results}
        if jsonErr := out.JSON(output); jsonErr != nil {
            return fmt.Errorf("failed to encode JSON output: %w", jsonErr)
        }
    }

    return nil
}

// moveItem performs a single item move operation.
func moveItem(
    ctx context.Context,
    db moveDB,
    itemID, toLocationID, moveType, projectAction, projectID, actorUserID, note string,
) (*Result, error) {
    // ... body unchanged except db type is now moveDB
}

// validateDestinationNotSystem checks that destination is not a system location.
func validateDestinationNotSystem(ctx context.Context, db moveDB, locationID string) error {
    // ... body unchanged except db type is now moveDB
}
```

The existing `moveItem` and its `item_test.go` tests continue to use `*database.Database` which satisfies `moveDB`. **No changes needed to `item_test.go`'s `moveItem` calls** — `setupMoveTest` returns a `*database.Database` which satisfies the interface.

### 2.5 `cmd/move/move.go` — Constructor Approach

**Delete** `var moveCmd *cobra.Command` and `GetMoveCmd()`.

**Add** two constructors:

```go
package move

import (
    "fmt"

    "github.com/spf13/cobra"

    "github.com/asphaltbuffet/wherehouse/internal/cli"
)

// NewMoveCmd constructs a move command with an injected database.
// Use for testing — db is pre-opened and ready to use.
// Caller must ensure db is closed when done (the command closes it via defer in RunE).
func NewMoveCmd(db moveDB) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "move <item-selector>... --to <location>",
        Short: "Move items to a different location",
        Long:  moveLongDescription,
        Args:  cobra.MinimumNArgs(1), //nolint:mnd // minimum 1 item selector required
        RunE: func(cmd *cobra.Command, args []string) error {
            defer db.Close()
            return runMoveItemCore(cmd, args, db)
        },
    }
    registerMoveFlags(cmd)
    return cmd
}

// NewDefaultMoveCmd constructs the production move command.
// Opens the database from context configuration on each invocation.
func NewDefaultMoveCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "move <item-selector>... --to <location>",
        Short: "Move items to a different location",
        Long:  moveLongDescription,
        Args:  cobra.MinimumNArgs(1), //nolint:mnd // minimum 1 item selector required
        RunE: func(cmd *cobra.Command, args []string) error {
            db, err := openDatabase(cmd.Context())
            if err != nil {
                return fmt.Errorf("failed to open database: %w", err)
            }
            defer db.Close()
            return runMoveItemCore(cmd, args, db)
        },
    }
    registerMoveFlags(cmd)
    return cmd
}

// registerMoveFlags registers all flags on the move command.
func registerMoveFlags(cmd *cobra.Command) {
    cmd.Flags().StringP("to", "t", "", "destination location (required)")
    _ = cmd.MarkFlagRequired("to")
    cmd.Flags().Bool("temp", false, "temporary move (preserve origin for return)")
    cmd.Flags().String("project", "", "associate with project")
    cmd.Flags().Bool("keep-project", false, "preserve current project association")
    cmd.Flags().Bool("clear-project", false, "clear project association (default behavior)")
    cmd.MarkFlagsMutuallyExclusive("project", "keep-project")
    cmd.MarkFlagsMutuallyExclusive("project", "clear-project")
    cmd.MarkFlagsMutuallyExclusive("keep-project", "clear-project")
    cmd.Flags().StringP("note", "n", "", "optional note for event")
}

const moveLongDescription = `Move one or more items to a different location.
... (identical to current Long string)`
```

### 2.6 `cmd/root.go` — Update Call Site

Line 72: `move.GetMoveCmd()` → `move.NewDefaultMoveCmd()`

### 2.7 `.mockery.yaml` — Add moveDB Interface

```yaml
with-expecter: true
dir: "{{.InterfaceDir}}/mocks"
outpkg: mocks
mockname: "Mock{{.InterfaceName}}"
filename: "mock_{{.InterfaceName | lower}}.go"
resolve-type-alias: false
disable-version-string: true
issue-845-fix: true
packages:
  github.com/asphaltbuffet/wherehouse/internal/logging:
    interfaces:
      Logger:
  github.com/asphaltbuffet/wherehouse/cmd/move:
    interfaces:
      moveDB:
```

Run `mockery` from the project root to generate `cmd/move/mocks/mock_movedb.go`.

---

## 3. Test Changes

### 3.1 `cmd/move/item_test.go` — Modifications Only

The existing `moveItem()` tests use real in-memory SQLite via `setupMoveTest`. These tests exercise the database behavior (event creation, projection updates, system location checks). **Keep using real SQLite** — this is the correct layer for these tests.

#### 3.1.1 `setupMoveTest` — Fix `context.Background()`

```go
// BEFORE
ctx := context.Background()
// AFTER
ctx := t.Context()
```

#### 3.1.2 Six error assertion fixes

`assert.Contains(t, err.Error(), ...)` → `assert.ErrorContains(t, err, ...)` for all six error-path tests:

| Test | Message substring |
|---|---|
| `TestMoveItem_FromSystemLocation_Missing_Fails` | `"cannot move items from system location"` |
| `TestMoveItem_FromSystemLocation_Borrowed_Fails` | `"cannot move items from system location"` |
| `TestMoveItem_ToSystemLocation_Missing_Fails` | `"cannot move items to system location"` |
| `TestMoveItem_ToSystemLocation_Borrowed_Fails` | `"cannot move items to system location"` |
| `TestMoveItem_ItemNotFound_Fails` | `"item not found"` |
| `TestMoveItem_DestinationNotFound_Fails` | `"to location not found"` |

#### 3.1.3 `TestResult_JSONMarshal` → `TestResult_JSONFieldNames`

Rename and replace body with tag-name verification only:

```go
func TestResult_JSONFieldNames(t *testing.T) {
    result := &Result{ItemID: "abc", DisplayName: "socket", EventID: 42}
    data, err := json.Marshal(result)
    require.NoError(t, err)
    var m map[string]any
    require.NoError(t, json.Unmarshal(data, &m))
    assert.Contains(t, m, "item_id")
    assert.Contains(t, m, "display_name")
    assert.Contains(t, m, "event_id")
    assert.Contains(t, m, "from_location")
    assert.Contains(t, m, "to_location")
    assert.Contains(t, m, "move_type")
    _, hasProjectAction := m["project_action"]
    assert.False(t, hasProjectAction, "project_action should be omitted when empty")
    _, hasProjectID := m["project_id"]
    assert.False(t, hasProjectID, "project_id should be omitted when empty")
}
```

#### 3.1.4 `TestGetMoveCmd_Structure` → `TestNewMoveCmd_FlagsRegistered`

```go
func TestNewMoveCmd_FlagsRegistered(t *testing.T) {
    cmd := NewDefaultMoveCmd()
    require.NotNil(t, cmd)
    require.NotNil(t, cmd.Flags().Lookup("to"))
    assert.NotNil(t, cmd.Flags().Lookup("temp"))
    assert.NotNil(t, cmd.Flags().Lookup("project"))
    assert.NotNil(t, cmd.Flags().Lookup("keep-project"))
    assert.NotNil(t, cmd.Flags().Lookup("clear-project"))
    assert.NotNil(t, cmd.Flags().Lookup("note"))
}
```

### 3.2 `cmd/move/helpers_test.go` — Modifications

#### 3.2.1 Delete `TestIsQuietMode`

Remove entirely — unconditional skip.

#### 3.2.2 Fix `context.Background()` in `TestResolveLocation` and `TestResolveItemSelector`

`context.Background()` → `t.Context()`.

#### 3.2.3 Reduce `TestResolveLocation` to 2 cases

Keep: `"resolve by UUID"`, `"location not found"`.
Delete: `"resolve by canonical name"`, `"resolve by display name"`, `"resolve with spaces in name"`.

#### 3.2.4 Reduce `TestResolveItemSelector` to 2 cases

Keep: `"resolve by UUID"`, `"item not found"`.
Delete: `"resolve by LOCATION:ITEM"`, `"resolve by canonical name"`, `"invalid UUID"`.

#### 3.2.5 Use `require.ErrorAssertionFunc` style for both tests

```go
tests := []struct {
    name         string
    input        string
    wantID       string
    errAssertion require.ErrorAssertionFunc
}{
    {"resolve by UUID", garageID, garageID, require.NoError},
    {"location not found", "nonexistent", "", require.Error},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        gotID, err := resolveLocation(t.Context(), db, tt.input)
        tt.errAssertion(t, err)
        if err == nil {
            assert.Equal(t, tt.wantID, gotID)
        }
    })
}
```

Apply same pattern to `TestResolveItemSelector`.

#### 3.2.6 Remove unused `context` import

After replacing `ctx := context.Background()` and using `t.Context()` inline, remove the `"context"` import if unused.

### 3.3 `cmd/move/wiring_test.go` — NEW FILE

**Package**: `package move` (same-package — access to `moveDB`, `runMoveItemCore`, `NewMoveCmd`)

**Import** the generated mock: `"github.com/asphaltbuffet/wherehouse/cmd/move/mocks"`

#### Setup helper

```go
// setupWiringTest creates a NewMoveCmd with a mock database and output buffers.
// The mock is returned for configuring expectations.
func setupWiringTest(t *testing.T) (
    cmd *cobra.Command,
    mockDB *mocks.MockMoveDB,
    outBuf *bytes.Buffer,
    errBuf *bytes.Buffer,
) {
    t.Helper()

    mockDB = mocks.NewMockMoveDB(t)

    // Config in context (needed for MustGetConfig and GetActorUserID)
    cfg := config.GetDefaults()
    cmdCtx := context.WithValue(t.Context(), config.ConfigKey, cfg)

    outBuf = &bytes.Buffer{}
    errBuf = &bytes.Buffer{}

    cmd = NewMoveCmd(mockDB)
    cmd.SetContext(cmdCtx)
    cmd.SetOut(outBuf)
    cmd.SetErr(errBuf)

    return cmd, mockDB, outBuf, errBuf
}
```

**Note**: `NewMoveCmd(db moveDB)` receives the mock. The `RunE` closure captures it. `db.Close()` is deferred — the mock must expect `Close()`.

#### Shared mock setup helper for happy-path tests

Every wiring test that succeeds needs the mock to handle resolution and the move. A shared builder avoids repetition:

```go
// configureMockForSuccessfulMove sets up mock expectations for a standard single-item move.
// itemID and toLocationID are the resolved IDs the mock returns.
func configureMockForSuccessfulMove(
    t *testing.T,
    m *mocks.MockMoveDB,
    itemID, fromLocationID, toLocationID string,
) {
    t.Helper()

    m.EXPECT().Close().Return(nil).Once()

    // resolveLocation (GetLocationByCanonicalName for "toolbox")
    m.EXPECT().
        GetLocationByCanonicalName(mock.Anything, "toolbox").
        Return(&database.Location{LocationID: toLocationID, DisplayName: "Toolbox", IsSystem: false}, nil).
        Once()

    // validateDestinationNotSystem (GetLocation for toLocationID)
    m.EXPECT().
        GetLocation(mock.Anything, toLocationID).
        Return(&database.Location{LocationID: toLocationID, DisplayName: "Toolbox", IsSystem: false}, nil).
        Once()

    // resolveItemSelector (GetItemsByCanonicalName for "socket")
    m.EXPECT().
        GetItemsByCanonicalName(mock.Anything, mock.AnythingOfType("string")).
        Return([]*database.Item{{ItemID: itemID, DisplayName: "socket", LocationID: fromLocationID}}, nil).
        Once()

    // moveItem: GetItem
    m.EXPECT().
        GetItem(mock.Anything, itemID).
        Return(&database.Item{ItemID: itemID, DisplayName: "socket", LocationID: fromLocationID}, nil).
        Once()

    // moveItem: GetLocation for fromLocation
    m.EXPECT().
        GetLocation(mock.Anything, fromLocationID).
        Return(&database.Location{LocationID: fromLocationID, DisplayName: "Garage", IsSystem: false}, nil).
        Once()

    // moveItem: GetLocation for toLocation (second call)
    m.EXPECT().
        GetLocation(mock.Anything, toLocationID).
        Return(&database.Location{LocationID: toLocationID, DisplayName: "Toolbox", IsSystem: false}, nil).
        Once()

    // moveItem: ValidateFromLocation
    m.EXPECT().
        ValidateFromLocation(mock.Anything, itemID, fromLocationID).
        Return(nil).
        Once()

    // moveItem: AppendEvent
    m.EXPECT().
        AppendEvent(mock.Anything, "item.moved", mock.Anything, mock.Anything, mock.Anything).
        Return(int64(1), nil).
        Once()
}
```

**Note on call order for `GetLocation`**: `validateDestinationNotSystem` calls `GetLocation(toLocationID)` once, and `moveItem` calls `GetLocation(toLocationID)` a second time. Mockery's `Once()` handles this correctly — two separate `EXPECT()` calls for the same args each match once.

**Note on `resolveLocation` path**: When the input is `"toolbox"` (not a UUID), `cli.ResolveLocation` calls `GetLocationByCanonicalName`. When the input is a UUID, it calls `GetLocation` first. Tests should use canonical names for `--to` to trigger the canonical path, keeping mock expectations predictable.

#### Flag-to-Parameter Wiring Tests

The mock's `AppendEvent` captures the payload. To verify that `moveType` and `projectAction` reach the event payload, use `mock.MatchedBy`:

```go
// TestRunMoveItemCore_TempFlag_SetsMoveType verifies --temp results in "temporary_use" move type in the event payload.
func TestRunMoveItemCore_TempFlag_SetsMoveType(t *testing.T) {
    const itemID = "item-uuid-001"
    const fromLocID = "from-loc-uuid"
    const toLocID = "to-loc-uuid"

    cmd, mockDB, _, _ := setupWiringTest(t)
    configureMockForSuccessfulMove(t, mockDB, itemID, fromLocID, toLocID)

    // Override AppendEvent expectation to verify moveType in payload
    // (remove the generic one and add a specific one)
    // Simpler: use mock.MatchedBy on the AppendEvent call in configureMockForSuccessfulMove
    // ... see note below on expectation override

    cmd.SetArgs([]string{"socket", "--to", "toolbox", "--temp"})
    err := cmd.ExecuteContext(cmd.Context())
    require.NoError(t, err)
}
```

**Practical note on payload verification**: `AppendEvent`'s `payload` parameter is `map[string]any`. To verify `moveType` in the payload, use `mock.MatchedBy`:

```go
m.EXPECT().
    AppendEvent(
        mock.Anything,
        "item.moved",
        mock.Anything,
        mock.MatchedBy(func(p map[string]any) bool {
            return p["move_type"] == "temporary_use"
        }),
        mock.Anything,
    ).
    Return(int64(1), nil).
    Once()
```

The `configureMockForSuccessfulMove` helper should accept an `AppendEvent` matcher so tests can customize payload verification. Refactored:

```go
func configureMockForSuccessfulMove(
    t *testing.T,
    m *mocks.MockMoveDB,
    itemID, fromLocationID, toLocationID string,
    payloadMatcher interface{}, // pass mock.Anything for don't-care, or mock.MatchedBy(...) for assertions
) {
    // ... all expectations as above, but AppendEvent uses payloadMatcher
    m.EXPECT().
        AppendEvent(mock.Anything, "item.moved", mock.Anything, payloadMatcher, mock.Anything).
        Return(int64(1), nil).
        Once()
}
```

#### Full Wiring Test Set

```go
// TestRunMoveItemCore_TempFlag_SetsMoveType verifies --temp sets move_type to "temporary_use".
func TestRunMoveItemCore_TempFlag_SetsMoveType(t *testing.T) {
    const (itemID = "aaa"; fromLoc = "bbb"; toLoc = "ccc")
    cmd, m, _, _ := setupWiringTest(t)
    configureMockForSuccessfulMove(t, m, itemID, fromLoc, toLoc,
        mock.MatchedBy(func(p map[string]any) bool { return p["move_type"] == "temporary_use" }))
    cmd.SetArgs([]string{"socket", "--to", "toolbox", "--temp"})
    require.NoError(t, cmd.ExecuteContext(cmd.Context()))
}

// TestRunMoveItemCore_DefaultMoveType verifies default move_type is "rehome".
func TestRunMoveItemCore_DefaultMoveType(t *testing.T) {
    const (itemID = "aaa"; fromLoc = "bbb"; toLoc = "ccc")
    cmd, m, _, _ := setupWiringTest(t)
    configureMockForSuccessfulMove(t, m, itemID, fromLoc, toLoc,
        mock.MatchedBy(func(p map[string]any) bool { return p["move_type"] == "rehome" }))
    cmd.SetArgs([]string{"socket", "--to", "toolbox"})
    require.NoError(t, cmd.ExecuteContext(cmd.Context()))
}

// TestRunMoveItemCore_ProjectFlag_SetsProjectIDAndAction verifies --project sets project_id and project_action.
func TestRunMoveItemCore_ProjectFlag_SetsProjectIDAndAction(t *testing.T) {
    const (itemID = "aaa"; fromLoc = "bbb"; toLoc = "ccc")
    cmd, m, _, _ := setupWiringTest(t)

    // ValidateProjectExists is called when --project is set
    m.EXPECT().ValidateProjectExists(mock.Anything, "my-project", mock.Anything).Return(nil).Once()
    configureMockForSuccessfulMove(t, m, itemID, fromLoc, toLoc,
        mock.MatchedBy(func(p map[string]any) bool {
            return p["project_action"] == "set" && p["project_id"] == "my-project"
        }))
    cmd.SetArgs([]string{"socket", "--to", "toolbox", "--project", "my-project"})
    require.NoError(t, cmd.ExecuteContext(cmd.Context()))
}

// TestRunMoveItemCore_KeepProjectFlag_SetsKeepAction verifies --keep-project sets project_action=keep.
func TestRunMoveItemCore_KeepProjectFlag_SetsKeepAction(t *testing.T) {
    const (itemID = "aaa"; fromLoc = "bbb"; toLoc = "ccc")
    cmd, m, _, _ := setupWiringTest(t)
    configureMockForSuccessfulMove(t, m, itemID, fromLoc, toLoc,
        mock.MatchedBy(func(p map[string]any) bool { return p["project_action"] == "keep" }))
    cmd.SetArgs([]string{"socket", "--to", "toolbox", "--keep-project"})
    require.NoError(t, cmd.ExecuteContext(cmd.Context()))
}

// TestRunMoveItemCore_NoteFlag_ForwardsNote verifies --note is passed to AppendEvent.
func TestRunMoveItemCore_NoteFlag_ForwardsNote(t *testing.T) {
    const (itemID = "aaa"; fromLoc = "bbb"; toLoc = "ccc")
    cmd, m, _, _ := setupWiringTest(t)
    // For note, AppendEvent's last param is note string
    // configureMockForSuccessfulMove generic version; add specific AppendEvent check here:
    // Use mock.MatchedBy on note arg — requires changing helper to accept note matcher too.
    // Simplest: use mock.Anything for payload, check AppendEvent note arg directly.
    configureMockForSuccessfulMoveWithNote(t, m, itemID, fromLoc, toLoc, "organizing notes")
    cmd.SetArgs([]string{"socket", "--to", "toolbox", "--note", "organizing notes"})
    require.NoError(t, cmd.ExecuteContext(cmd.Context()))
}
```

**Helper for note verification**:
```go
func configureMockForSuccessfulMoveWithNote(t *testing.T, m *mocks.MockMoveDB,
    itemID, fromLoc, toLoc, expectedNote string) {
    t.Helper()
    // ... same as configureMockForSuccessfulMove but AppendEvent checks note:
    m.EXPECT().
        AppendEvent(mock.Anything, "item.moved", mock.Anything, mock.Anything,
            mock.MatchedBy(func(n string) bool { return n == expectedNote })).
        Return(int64(1), nil).Once()
}
```

#### Output Routing Tests

```go
// TestRunMoveItemCore_HumanOutput_WritesToOut verifies human-readable output goes to cmd.OutOrStdout().
func TestRunMoveItemCore_HumanOutput_WritesToOut(t *testing.T) {
    const (itemID = "aaa"; fromLoc = "bbb"; toLoc = "ccc")
    cmd, m, outBuf, errBuf := setupWiringTest(t)
    configureMockForSuccessfulMove(t, m, itemID, fromLoc, toLoc, mock.Anything)
    cmd.SetArgs([]string{"socket", "--to", "toolbox"})

    require.NoError(t, cmd.ExecuteContext(cmd.Context()))

    assert.Contains(t, outBuf.String(), "Moved item")
    assert.Empty(t, errBuf.String())
}

// TestRunMoveItemCore_JSONOutput_WritesToOut verifies JSON output goes to cmd.OutOrStdout().
func TestRunMoveItemCore_JSONOutput_WritesToOut(t *testing.T) {
    const (itemID = "aaa"; fromLoc = "bbb"; toLoc = "ccc")
    cmd, m, outBuf, errBuf := setupWiringTest(t)
    configureMockForSuccessfulMove(t, m, itemID, fromLoc, toLoc, mock.Anything)

    // Inject JSON-mode config into context
    cfg := config.GetDefaults()
    cfg.Output.DefaultFormat = "json"
    cmd.SetContext(context.WithValue(t.Context(), config.ConfigKey, cfg))
    cmd.SetArgs([]string{"socket", "--to", "toolbox"})

    require.NoError(t, cmd.ExecuteContext(cmd.Context()))

    assert.Empty(t, errBuf.String())
    var result map[string]any
    require.NoError(t, json.Unmarshal(outBuf.Bytes(), &result))
    assert.Contains(t, result, "moved")
}
```

#### Error Propagation Tests

```go
// TestRunMoveItemCore_DestinationNotFound_ReturnsError verifies bad --to propagates error.
func TestRunMoveItemCore_DestinationNotFound_ReturnsError(t *testing.T) {
    cmd, m, _, _ := setupWiringTest(t)
    m.EXPECT().Close().Return(nil).Once()
    m.EXPECT().
        GetLocationByCanonicalName(mock.Anything, "nonexistent").
        Return(nil, database.ErrLocationNotFound).Once()

    cmd.SetArgs([]string{"socket", "--to", "nonexistent"})
    err := cmd.ExecuteContext(cmd.Context())
    require.Error(t, err)
    assert.ErrorContains(t, err, "destination location not found")
}

// TestRunMoveItemCore_InvalidProject_ReturnsError verifies project validation error propagates.
func TestRunMoveItemCore_InvalidProject_ReturnsError(t *testing.T) {
    const toLoc = "ccc"
    cmd, m, _, _ := setupWiringTest(t)
    m.EXPECT().Close().Return(nil).Once()
    m.EXPECT().
        GetLocationByCanonicalName(mock.Anything, "toolbox").
        Return(&database.Location{LocationID: toLoc, DisplayName: "Toolbox", IsSystem: false}, nil).Once()
    m.EXPECT().
        GetLocation(mock.Anything, toLoc).
        Return(&database.Location{LocationID: toLoc, DisplayName: "Toolbox", IsSystem: false}, nil).Once()
    m.EXPECT().
        ValidateProjectExists(mock.Anything, "bad-project", mock.Anything).
        Return(fmt.Errorf("project not found")).Once()

    cmd.SetArgs([]string{"socket", "--to", "toolbox", "--project", "bad-project"})
    err := cmd.ExecuteContext(cmd.Context())
    require.Error(t, err)
    assert.ErrorContains(t, err, "project validation failed")
}

// TestRunMoveItemCore_UnresolvableItem_ReturnsError verifies bad item selector propagates error.
func TestRunMoveItemCore_UnresolvableItem_ReturnsError(t *testing.T) {
    const toLoc = "ccc"
    cmd, m, _, _ := setupWiringTest(t)
    m.EXPECT().Close().Return(nil).Once()
    m.EXPECT().
        GetLocationByCanonicalName(mock.Anything, "toolbox").
        Return(&database.Location{LocationID: toLoc, DisplayName: "Toolbox", IsSystem: false}, nil).Once()
    m.EXPECT().
        GetLocation(mock.Anything, toLoc).
        Return(&database.Location{LocationID: toLoc, DisplayName: "Toolbox", IsSystem: false}, nil).Once()
    m.EXPECT().
        GetItemsByCanonicalName(mock.Anything, mock.Anything).
        Return([]*database.Item{}, nil).Once()

    cmd.SetArgs([]string{"nonexistent-item", "--to", "toolbox"})
    err := cmd.ExecuteContext(cmd.Context())
    require.Error(t, err)
    assert.ErrorContains(t, err, "failed to resolve")
}
```

---

## 4. Test Inventory — Complete Final State

### `item_test.go`

| Test | Action | Notes |
|---|---|---|
| `setupMoveTest` | MODIFY | `context.Background()` → `t.Context()` |
| `TestMoveItem_EventCreated` | KEEP | Real SQLite — core success path |
| `TestMoveItem_TemporaryMove_EventCreated` | KEEP | Real SQLite |
| `TestMoveItem_WithProject_EventCreated` | KEEP | Real SQLite |
| `TestMoveItem_KeepProject_EventCreated` | KEEP | Real SQLite |
| `TestMoveItem_ClearProject_EventCreated` | KEEP | Real SQLite |
| `TestMoveItem_FromSystemLocation_Missing_Fails` | MODIFY | `assert.ErrorContains` |
| `TestMoveItem_FromSystemLocation_Borrowed_Fails` | MODIFY | `assert.ErrorContains` |
| `TestMoveItem_ToSystemLocation_Missing_Fails` | MODIFY | `assert.ErrorContains` |
| `TestMoveItem_ToSystemLocation_Borrowed_Fails` | MODIFY | `assert.ErrorContains` |
| `TestMoveItem_ItemNotFound_Fails` | MODIFY | `assert.ErrorContains` |
| `TestMoveItem_DestinationNotFound_Fails` | MODIFY | `assert.ErrorContains` |
| `TestDetermineMoveType` | KEEP | Pure function |
| `TestDetermineProjectAction` | KEEP | Pure function |
| `TestValidateDestinationNotSystem` | KEEP | Real SQLite — direct function test |
| `TestResult_JSONMarshal` | RENAME+MODIFY | → `TestResult_JSONFieldNames`; narrow to tag names |
| `TestGetMoveCmd_Structure` | RENAME+MODIFY | → `TestNewMoveCmd_FlagsRegistered`; use `NewDefaultMoveCmd()` |
| `TestMoveItem_WithNote_EventCreated` | KEEP | Real SQLite |
| `TestMoveItem_MultipleSequential` | KEEP | Real SQLite |
| `TestMoveItem_MultipleToSameDestination` | KEEP | Real SQLite |

### `helpers_test.go`

| Test | Action | Notes |
|---|---|---|
| `TestLooksLikeUUID` | KEEP | Pure function; 6 cases acceptable |
| `TestResolveLocation` | MODIFY | Reduce to 2 cases; `require.ErrorAssertionFunc`; `t.Context()` |
| `TestResolveItemSelector` | MODIFY | Reduce to 2 cases; same style |
| `TestIsQuietMode` | DELETE | Unconditional skip |
| `setupTestDatabase` | KEEP | Already correct |

### `wiring_test.go` (NEW)

| Test | Category | Mock Method Calls |
|---|---|---|
| `TestRunMoveItemCore_TempFlag_SetsMoveType` | Flag wiring | Full chain; AppendEvent payload verified |
| `TestRunMoveItemCore_DefaultMoveType` | Flag wiring | Full chain; AppendEvent payload verified |
| `TestRunMoveItemCore_ProjectFlag_SetsProjectIDAndAction` | Flag wiring | + ValidateProjectExists |
| `TestRunMoveItemCore_KeepProjectFlag_SetsKeepAction` | Flag wiring | Full chain |
| `TestRunMoveItemCore_NoteFlag_ForwardsNote` | Flag wiring | AppendEvent note arg verified |
| `TestRunMoveItemCore_HumanOutput_WritesToOut` | Output routing | Full chain; outBuf checked |
| `TestRunMoveItemCore_JSONOutput_WritesToOut` | Output routing | Full chain; JSON structure checked |
| `TestRunMoveItemCore_DestinationNotFound_ReturnsError` | Error propagation | GetLocationByCanonicalName returns error |
| `TestRunMoveItemCore_InvalidProject_ReturnsError` | Error propagation | ValidateProjectExists returns error |
| `TestRunMoveItemCore_UnresolvableItem_ReturnsError` | Error propagation | GetItemsByCanonicalName returns empty |

---

## 5. Linting Requirements

### 5.1 `thelper` — All helpers must call `t.Helper()` first

- `setupWiringTest`: first statement `t.Helper()`
- `configureMockForSuccessfulMove`: first statement `t.Helper()`
- `configureMockForSuccessfulMoveWithNote`: first statement `t.Helper()`

### 5.2 `testifylint`

- All `assert.Contains(t, err.Error(), ...)` → `assert.ErrorContains(t, err, ...)`
- Error table branches: `require.ErrorAssertionFunc`
- `assert.NotEmpty(t, result.EventID)` → `assert.Positive(t, result.EventID)` if EventID is int64

### 5.3 `usetesting`

- All `context.Background()` in test files → `t.Context()`

### 5.4 `mnd`

- `cobra.MinimumNArgs(1)` in both `NewMoveCmd` and `NewDefaultMoveCmd`: add `//nolint:mnd // minimum 1 item selector required`

### 5.5 `govet shadow`

- Check wiring test functions for `err :=` in nested scopes where outer `err` already exists — use `err =`

### 5.6 `gochecknoglobals`

- `var moveCmd *cobra.Command` removed — no new package globals introduced
- `moveLongDescription` const is fine (const, not var)

### 5.7 `reassign`

- With singleton removed, no `//nolint:reassign` directives needed

### 5.8 `wrapcheck` / error wrapping in wiring tests

- Wiring tests call `cmd.ExecuteContext` — cobra wraps RunE errors. Verify error message propagation patterns match what cobra surfaces (typically the error string is preserved).

---

## 6. Implementation Order

Execute in strict order to avoid compile errors at each step:

1. **`internal/cli/selectors.go`**: Define `LocationItemQuerier` interface; update `ResolveLocation` and `ResolveItemSelector` signatures. Verify `*database.Database` satisfies it.

2. **`go build ./...`**: Must pass before continuing. `*database.Database` satisfies `LocationItemQuerier` — no DB package changes needed.

3. **`cmd/move/mover.go`**: Create new file with `moveDB` interface definition and `//go:generate mockery --name=moveDB` directive.

4. **`cmd/move/helpers.go`**: Change `resolveLocation` and `resolveItemSelector` params from `*database.Database` to `moveDB`.

5. **`cmd/move/item.go`**: Change `*database.Database` → `moveDB` throughout; extract `runMoveItemCore`; remove old `runMoveItem`.

6. **`cmd/move/move.go`**: Replace singleton with `NewMoveCmd(db moveDB)` and `NewDefaultMoveCmd()`; extract `registerMoveFlags`.

7. **`cmd/root.go`**: `move.GetMoveCmd()` → `move.NewDefaultMoveCmd()`.

8. **`go build ./...`**: Must pass before generating mocks or touching tests.

9. **`.mockery.yaml`**: Add `cmd/move` package + `moveDB` interface.

10. **Run mockery**: `mockery` from project root generates `cmd/move/mocks/mock_movedb.go`.

11. **`go build ./...`**: Verify generated mock compiles.

12. **`cmd/move/item_test.go`**: Apply all modifications.

13. **`cmd/move/helpers_test.go`**: Delete `TestIsQuietMode`; fix `context.Background()`; reduce table cases; `require.ErrorAssertionFunc` style.

14. **`cmd/move/wiring_test.go`**: Create with all wiring, output, and error tests.

15. **`mise run lint`**: Fix any lint issues.

16. **`mise run test`**: All tests must pass.

---

## 7. Key Decisions and Trade-offs

| Decision | Rationale |
|---|---|
| `moveDB` interface in `cmd/move` package (not `internal/database`) | Narrow interface; follows Go proverb "accept interfaces, return concrete types"; `internal/database` stays clean |
| `LocationItemQuerier` interface in `internal/cli` | Enables `moveDB` (which includes those methods) to be used with resolver functions; one-time change benefiting all future CLI commands; `*database.Database` satisfies it with zero changes to the database package |
| `NewMoveCmd(db moveDB)` injects DB at construction time | Cleanest seam for testing; no `openDatabase` call inside `RunE` for the test path; mock is pre-built in test |
| `NewDefaultMoveCmd()` opens DB inside `RunE` | Preserves per-invocation DB open for production; consistent with cobra lifecycle |
| Wiring tests use mockery v3 `MockMoveDB` | Eliminates all DB setup for CLI-layer tests; mock verifies exact parameters at each layer |
| `moveItem()` tests keep real SQLite | These tests exercise actual database behavior (event append, projection update, system location enforcement) — mocking would make them worthless |
| No spy `moveItemFunc` type | Rejected per clarification — mockery is the idiomatic approach; no ad-hoc function type needed |
| Singleton removed entirely | Clarification required this; constructor-per-call eliminates global state and all `//nolint:reassign` directives |
| `configureMockForSuccessfulMove` helper | Centralizes the full call sequence; `AppendEvent` payload is parameterized so individual tests can assert specific field values |
| `GetLocation` called twice for `--to` location | `validateDestinationNotSystem` and `moveItem` both call `GetLocation(toLocationID)`; mock `EXPECT()` uses `Once()` for each — mockery handles ordering correctly |
