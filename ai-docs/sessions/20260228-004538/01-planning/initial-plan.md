# Architecture Plan: cmd/move Test Refactoring

## Analysis Summary

The existing tests are mostly well-structured around the extracted `moveItem()` function (§2 thin entrypoint pattern is already applied). However, several gaps and anti-patterns exist that this plan addresses.

---

## Section 1 — Tests to DELETE (Cobra-Guaranteed Behaviors or Redundant)

### 1.1 `TestIsQuietMode` in `helpers_test.go` (line 195–203)

This test calls `t.Skip(...)` with a comment admitting it tests flag helpers. The skip itself signals an untestable design smell (§9 anti-pattern: "Skipping tests with `t.Skip` for flag helpers"). The test body is empty in effect. DELETE it.

**Why:** The test skips itself unconditionally. It adds no coverage. It is a placeholder that signals a known gap but doesn't fix it.

### 1.2 `TestGetMoveCmd_Structure` in `item_test.go` (lines 390–407) — PARTIAL deletion

The `cmd.Name()`, `cmd.Short`, `cmd.Long` assertions test string literals that are not executable behaviors (§6: "Command `Use`, `Short`, `Long` fields are string literals — not executable"). These three assertions should be removed.

The flag existence checks (`Lookup("to")`, `Lookup("temp")`, etc.) are borderline — they do verify registration, which is §4.8 territory (subcommand/flag registration). However, they only check that flags exist, not that they wire to the correct parameters. They are acceptable to keep but need augmentation with wiring tests. Recommended: keep flag existence checks, remove string literal assertions.

---

## Section 2 — Tests to ADD

### 2.1 Flag-to-Parameter Wiring Tests (§4.5 — critical gap)

The current tests call `moveItem()` directly with explicit string parameters. This means they bypass the `runMoveItem()` wiring entirely. There are ZERO tests that verify `--temp` sets `moveType="temporary_use"`, that `--project foo` reaches `projectID="foo"`, that `--keep-project` reaches `keepProject=true`, or that `--note "text"` reaches `note="text"`.

These must be tested at the `cmd/` layer boundary, not in `moveItem()` directly.

**Approach:** Build a `newTestMoveCmd()` helper that wires the command with a captured-parameter spy function in place of `moveItem`. The spy records what parameters were passed; the test asserts the parameters match what the flags dictated.

Since `moveItem` is a package-level function (not injected), we need a small refactor: extract a `moveFn` type and make `runMoveItem` accept it, or use a package-level var that tests can swap. See Section 4 (Implementation Changes) below.

**New test functions:**

```go
// TestRunMoveItem_TempFlag_WiresMoveType verifies --temp reaches moveType="temporary_use"
func TestRunMoveItem_TempFlag_WiresMoveType(t *testing.T) { ... }

// TestRunMoveItem_ProjectFlag_WiresProjectID verifies --project foo reaches projectID
func TestRunMoveItem_ProjectFlag_WiresProjectID(t *testing.T) { ... }

// TestRunMoveItem_KeepProjectFlag_WiresProjectAction verifies --keep-project reaches projectAction="keep"
func TestRunMoveItem_KeepProjectFlag_WiresProjectAction(t *testing.T) { ... }

// TestRunMoveItem_NoteFlag_WiresNote verifies --note "text" reaches note parameter
func TestRunMoveItem_NoteFlag_WiresNote(t *testing.T) { ... }

// TestRunMoveItem_ToFlag_WiresToLocation verifies --to garage resolves and reaches toLocationID
func TestRunMoveItem_ToFlag_WiresToLocation(t *testing.T) { ... }
```

### 2.2 Output Routing Tests (§4.6 — missing entirely)

No test verifies that success output goes to `cmd.OutOrStdout()` and not `os.Stdout`. No test verifies `--json` switches to JSON format. The `runMoveItem()` function uses `out.Success(...)` and `out.JSON(...)` via `cli.OutputWriter`, but no test exercises this path.

**New test functions:**

```go
// TestRunMoveItem_JSONOutput_WritesToCmdOut verifies JSON output routed to cmd.OutOrStdout()
func TestRunMoveItem_JSONOutput_WritesToCmdOut(t *testing.T) {
    // Uses cmd.SetOut(&buf) + cmd.SetArgs([...,"--json"]) + cmd.Execute()
    // Asserts buf contains valid JSON with "moved" key
    // Asserts errBuf is empty
}

// TestRunMoveItem_HumanOutput_WritesToCmdOut verifies human output routed to cmd.OutOrStdout()
func TestRunMoveItem_HumanOutput_WritesToCmdOut(t *testing.T) {
    // Uses cmd.SetOut(&buf)
    // Asserts buf contains "Moved item" substring
}

// TestRunMoveItem_QuietMode_SuppressesOutput verifies --quiet suppresses success output
func TestRunMoveItem_QuietMode_SuppressesOutput(t *testing.T) { ... }
```

### 2.3 Error Propagation Tests (§4.4 — partial gap)

`moveItem()` errors are tested directly. But `runMoveItem()` has its own error formatting wrapping (`fmt.Errorf("failed to resolve %q: %w", selector, ...)` and `fmt.Errorf("failed to move %q: %w", ...)`). No test verifies that these error strings are propagated correctly from the RunE boundary.

Additionally, project validation failure (`db.ValidateProjectExists`) is not tested at all — the `--project` flag path has no error case test.

**New test functions:**

```go
// TestRunMoveItem_InvalidProject_PropagatesError verifies project validation error propagates
func TestRunMoveItem_InvalidProject_PropagatesError(t *testing.T) {
    // Pass --project nonexistent-project-id
    // Expect error containing "project validation failed"
}

// TestRunMoveItem_UnresolvableDestination_PropagatesError
func TestRunMoveItem_UnresolvableDestination_PropagatesError(t *testing.T) {
    // Pass --to totally-unknown-location
    // Expect error containing "destination location not found"
}

// TestRunMoveItem_UnresolvableSelector_PropagatesError
func TestRunMoveItem_UnresolvableSelector_PropagatesError(t *testing.T) {
    // Pass selector that doesn't exist
    // Expect error containing "failed to resolve"
}
```

### 2.4 Singleton Pattern Tests (§4.9 — missing)

`GetMoveCmd()` uses the singleton pattern (package-level `var moveCmd`). No test verifies same-pointer identity on successive calls.

```go
// TestGetMoveCmd_Singleton_ReturnsSamePointer verifies successive calls return same pointer
func TestGetMoveCmd_Singleton_ReturnsSamePointer(t *testing.T) {
    moveCmd = nil //nolint:reassign // reset singleton for test isolation
    cmd1 := GetMoveCmd()
    cmd2 := GetMoveCmd()
    assert.Same(t, cmd1, cmd2)
}
```

### 2.5 `resolveItemSelector` — Ambiguous Match Case (missing from helpers_test.go)

The current `TestResolveItemSelector` table does not include the case where a canonical name matches multiple items (ambiguous). The implementation calls `cli.ResolveItemSelector` which should return an error for ambiguous matches. This case must be tested.

```go
{
    name:      "ambiguous canonical name returns error with ID list",
    selector:  "socket",  // two items share this substring... actually exact canonical must match
    wantError: true,
}
```

Note: The exact behavior (whether it's "multiple matches" or "not found") depends on `cli.ResolveItemSelector`. This should be verified against its test in `internal/cli/selectors_test.go` and aligned.

### 2.6 `testifylint` Compliance Fixes

Current tests use `assert.Contains(t, err.Error(), "...")` in multiple places — this is a `testifylint` violation (§8.2). All such calls must become `assert.ErrorContains(t, err, "...")`.

Affected tests:
- `TestMoveItem_FromSystemLocation_Missing_Fails` (line 188)
- `TestMoveItem_FromSystemLocation_Borrowed_Fails` (line 206)
- `TestMoveItem_ToSystemLocation_Missing_Fails` (line 219)
- `TestMoveItem_ToSystemLocation_Borrowed_Fails` (line 232)
- `TestMoveItem_ItemNotFound_Fails` (line 245)
- `TestMoveItem_DestinationNotFound_Fails` (line 258)

### 2.7 `usetesting` Compliance Fixes

`setupMoveTest` uses `context.Background()` (line 32) instead of `t.Context()` (§8.3). `setupTestDatabase` in `helpers_test.go` does not create a context but callers use `context.Background()` in-line (lines 61, 121). All must become `t.Context()`.

---

## Section 3 — Tests to MODIFY (Wrong Approach, Right Intent)

### 3.1 `TestResolveLocation` and `TestResolveItemSelector` — error assertion style

Both use `assert.Error(t, gotErr)` for error cases. Per §5.2 pattern, use `require.ErrorAssertionFunc` in the table struct for richer assertions, or at minimum switch to `require.Error` where the test should stop on failure.

Also: these tests call `resolveLocation` and `resolveItemSelector`, which are thin wrappers around `cli.ResolveLocation` and `cli.ResolveItemSelector`. The tests are testing the `internal/cli` functions indirectly, not the wrappers themselves. The wrappers add no logic (just pass-throughs). These tests could be simplified to verify the wrapper connects correctly (one happy path + one error path) rather than exhaustively retesting `internal/cli` behavior. The exhaustive cases belong in `internal/cli/selectors_test.go`.

**Decision:** Reduce `TestResolveLocation` and `TestResolveItemSelector` to 2 cases each (one success, one error) since they're testing pass-through wrappers. Move edge cases to `internal/cli/selectors_test.go` if not already there.

### 3.2 `TestResult_JSONMarshal`

This test verifies Go's standard `json.Marshal`/`json.Unmarshal` on a struct with json tags. This is testing the Go standard library behavior, not application logic. The only application-owned decision is the json tag names themselves (e.g., `"item_id"`, `"display_name"`). The test should be narrowed to verify tag names only:

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
}
```

---

## Section 4 — Implementation Changes for Testability

### 4.1 Inject `moveFn` Dependency (Enables Wiring Tests)

The core problem for testing flag-to-parameter wiring is that `runMoveItem` calls the package-level `moveItem()` function directly. There is no seam to intercept parameter values without running a real database operation.

**Recommended approach:** Extract a `moveItemFunc` type and pass it through a command constructor.

```go
// item.go
type moveItemFunc func(
    ctx context.Context,
    db *database.Database,
    itemID, toLocationID, moveType, projectAction, projectID, actorUserID, note string,
) (*Result, error)
```

Change `move.go` to use a `NewMoveCmd(fn moveItemFunc)` constructor pattern alongside the existing singleton `GetMoveCmd()` for backwards compatibility:

```go
// move.go
func NewMoveCmd(fn moveItemFunc) *cobra.Command {
    cmd := &cobra.Command{ ... }
    cmd.RunE = func(cmd *cobra.Command, args []string) error {
        return runMoveItemWith(cmd, args, fn)
    }
    // ... flag declarations ...
    return cmd
}

func GetMoveCmd() *cobra.Command {
    if moveCmd != nil { return moveCmd }
    moveCmd = NewMoveCmd(moveItem)
    return moveCmd
}
```

This pattern:
- Preserves backward compatibility for `cmd/root.go` which calls `GetMoveCmd()`
- Enables wiring tests to pass a spy `moveItemFunc`
- Eliminates the need for package-level variable swapping
- Follows constructor function preference over singleton for test code (§5.6)

### 4.2 `runMoveItemWith` Signature

Extract the inner logic of `runMoveItem` into a function that accepts the `fn moveItemFunc` parameter:

```go
func runMoveItemWith(cmd *cobra.Command, args []string, fn moveItemFunc) error {
    // ... existing runMoveItem body, but calls fn(...) instead of moveItem(...)
}
```

`RunE` becomes:
```go
RunE: func(cmd *cobra.Command, args []string) error {
    return runMoveItemWith(cmd, args, moveItem)
}
```

### 4.3 Context Threading Test via `NewMoveCmd`

With `NewMoveCmd`, tests can use `cmd.ExecuteContext(ctx)` (§5.5) to pass a context carrying `config.ConfigKey`. This is how output routing tests will inject a test config that enables JSON mode.

---

## Section 5 — Complete Test Inventory (After Refactoring)

### `helpers_test.go` — Final State

| Test | Status | Notes |
|---|---|---|
| `TestLooksLikeUUID` | KEEP (reduce) | 6 cases → 3 representative cases sufficient; UUID detection is tested in internal/cli |
| `TestResolveLocation` | MODIFY | Reduce to 2 cases (pass-through wrapper); remove edge case duplication from internal/cli |
| `TestResolveItemSelector` | MODIFY | Reduce to 2-3 cases; add ambiguous match case |
| `TestIsQuietMode` | DELETE | Unconditional skip; no coverage value |

### `item_test.go` — Final State

| Test | Status | Notes |
|---|---|---|
| `setupMoveTest` | MODIFY | `context.Background()` → `t.Context()` |
| `setupTestDatabase` | KEEP | Already compliant |
| `TestMoveItem_EventCreated` | KEEP | Core success path |
| `TestMoveItem_TemporaryMove_EventCreated` | KEEP | Valid `moveItem()` behavior test |
| `TestMoveItem_WithProject_EventCreated` | KEEP | Valid project association test |
| `TestMoveItem_KeepProject_EventCreated` | KEEP | Valid project keep test |
| `TestMoveItem_ClearProject_EventCreated` | KEEP | Valid project clear test |
| `TestMoveItem_FromSystemLocation_Missing_Fails` | MODIFY | `assert.Contains(err.Error())` → `assert.ErrorContains` |
| `TestMoveItem_FromSystemLocation_Borrowed_Fails` | MODIFY | Same fix |
| `TestMoveItem_ToSystemLocation_Missing_Fails` | MODIFY | Same fix |
| `TestMoveItem_ToSystemLocation_Borrowed_Fails` | MODIFY | Same fix |
| `TestMoveItem_ItemNotFound_Fails` | MODIFY | Same fix |
| `TestMoveItem_DestinationNotFound_Fails` | MODIFY | Same fix |
| `TestDetermineMoveType` | KEEP | Pure function, correct table test |
| `TestDetermineProjectAction` | KEEP | Pure function, correct table test |
| `TestValidateDestinationNotSystem` | KEEP | Direct function test |
| `TestResult_JSONMarshal` | MODIFY | Narrow to tag name verification only |
| `TestGetMoveCmd_Structure` | MODIFY | Remove string literal assertions; keep flag existence checks; add singleton identity test |
| `TestMoveItem_WithNote_EventCreated` | KEEP | Valid |
| `TestMoveItem_MultipleSequential` | KEEP | Valid sequential event ordering test |
| `TestMoveItem_MultipleToSameDestination` | KEEP | Valid multiple-move test |

### New Tests to ADD (in `item_test.go` or `wiring_test.go`)

| Test | Category |
|---|---|
| `TestGetMoveCmd_Singleton_ReturnsSamePointer` | §4.9 singleton |
| `TestRunMoveItem_TempFlag_WiresMoveType` | §4.5 wiring |
| `TestRunMoveItem_ProjectFlag_WiresProjectID` | §4.5 wiring |
| `TestRunMoveItem_KeepProjectFlag_WiresProjectAction` | §4.5 wiring |
| `TestRunMoveItem_NoteFlag_WiresNote` | §4.5 wiring |
| `TestRunMoveItem_JSONOutput_WritesToCmdOut` | §4.6 output routing |
| `TestRunMoveItem_HumanOutput_WritesToCmdOut` | §4.6 output routing |
| `TestRunMoveItem_QuietMode_SuppressesOutput` | §4.6 output routing |
| `TestRunMoveItem_InvalidProject_PropagatesError` | §4.4 error propagation |
| `TestRunMoveItem_UnresolvableDestination_PropagatesError` | §4.4 error propagation |
| `TestRunMoveItem_UnresolvableSelector_PropagatesError` | §4.4 error propagation |

---

## Section 6 — Test File Organization

The new wiring/output tests require a real or fake database context plus a spy function. Recommended organization:

- **`item_test.go`**: All `moveItem()` direct tests (current structure, after modifications)
- **`wiring_test.go`**: New wiring, output routing, and error propagation tests that use `NewMoveCmd` with a spy function
- **`helpers_test.go`**: `resolveLocation`, `resolveItemSelector`, `looksLikeUUID` (reduced)

---

## Section 7 — Anti-Patterns Identified (§9 Reference)

| Anti-pattern | Location | Severity |
|---|---|---|
| `assert.Contains(t, err.Error(), ...)` instead of `assert.ErrorContains` | item_test.go, 6 tests | Medium — linting violation |
| `context.Background()` instead of `t.Context()` | setupMoveTest, helpers_test.go | Medium — linting violation |
| `t.Skip` placeholder test | helpers_test.go:TestIsQuietMode | Low — dead test |
| Domain logic in RunE (project validation) | item.go:runMoveItem | Medium — not easily testable; should stay but needs wiring test |
| Zero flag-to-parameter wiring tests | item_test.go | High — critical coverage gap |
| Zero output routing tests | item_test.go | High — critical coverage gap |
| Singleton `GetMoveCmd()` with no injectable constructor | move.go | Medium — prevents wiring tests without global swap |
| Testing string literal metadata (Name, Short, Long) | TestGetMoveCmd_Structure | Low — wasted test |

---

## Section 8 — Implementation Order for Developer

1. Refactor `move.go`: Add `NewMoveCmd(fn moveItemFunc)` constructor; update `GetMoveCmd()` to use it
2. Refactor `item.go`: Extract `runMoveItemWith(cmd, args, fn)` function
3. Fix linting violations in existing tests (`assert.ErrorContains`, `t.Context()`)
4. Delete `TestIsQuietMode`
5. Modify `TestGetMoveCmd_Structure` (remove string literal assertions)
6. Reduce `TestResolveLocation` and `TestResolveItemSelector` to pass-through verification
7. Add `TestGetMoveCmd_Singleton_ReturnsSamePointer`
8. Add wiring tests in `wiring_test.go`
9. Add output routing tests in `wiring_test.go`
10. Add error propagation tests in `wiring_test.go`
11. Run `mise run lint` and fix any issues
12. Run `mise run test` and verify all tests pass
