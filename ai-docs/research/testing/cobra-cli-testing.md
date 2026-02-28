# Cobra CLI Testing: Effective Unit Testing for `cmd/` Packages

## Section 1 — Agent Quick Reference

**How to use this document economically:**

| If you are... | Read... |
|---|---|
| Writing a new command's test file from scratch | §2 (core principle), §3 (what NOT to test), §4 (what to test), §5 (patterns) |
| Deciding coverage targets | §6 |
| Writing tests for `PersistentPreRunE`/config hooks | §5.3 |
| Evaluating output/format flag testing | §5.4 |
| Getting a linting error on a test you just wrote | §8 |
| Considering integration tests | §7 (brief) |

**Decision rule (applies to every test you write):**
> Before writing a test, ask: "Would this test fail if I deleted the relevant Cobra call?" If no — because Cobra guarantees the behavior — delete the test.

**What Cobra guarantees (never redundantly test):** argument count validators, flag type-safety, required-flag enforcement — see §3.

**What must be tested in `cmd/`:** flag-to-parameter wiring, argument content, cross-flag constraints, error propagation, output routing, context threading, singleton construction — see §4.

---

## Section 2 — The Core Principle: Thin Entrypoint

The authoritative Cobra Enterprise Guide states:

> "Commands become thin orchestration layers while business logic remains isolated and testable."

### The Pattern

Extract all domain logic from `RunE` into a named function. Test that function directly. The `RunE` closure is infrastructure wiring — nearly untestable at the unit level — and need not be driven through `cmd.Execute()`.

```go
// item.go
func runMoveItem(cmd *cobra.Command, args []string) error {
    toLocation, _ := cmd.Flags().GetString("to")
    db, _ := openDatabase(cmd.Context())
    return moveItem(cmd.Context(), db, args[0], toLocation, ...) // ← test this
}

// item_test.go — test the extracted function, not RunE
func TestMoveItem_EventCreated(t *testing.T) {
    db, ctx, ids := setupMoveTest(t)
    result, err := moveItem(ctx, db, ids.itemID1, ids.toolboxID, "rehome", ...)
    require.NoError(t, err)
    assert.Positive(t, result.EventID)
}
```

The two variants are:
- **Function wrapper:** `RunE` is an anonymous closure that calls `runXxx()`
- **Function reference:** `RunE: runXxx` (direct assignment)

Both are valid. The extracted function is what receives comprehensive testing.

### What This Means for Coverage

The `RunE` body itself (flag extraction, `openDatabase`, delegation) is glue code. Achieving 100% coverage on it requires driving through the full Cobra stack with a real or fake database — high cost, low signal. The correct strategy is: test the extracted function exhaustively in `internal/` or the command package; accept lower `cmd/` coverage on the RunE body itself.

---

## Section 3 — What Cobra Guarantees (Do NOT Test These)

These behaviors are implemented and tested by Cobra's own test suite. Writing tests for them is wasted effort and creates brittle tests that break on Cobra upgrades.

### 3.1 Argument Count Validators

Cobra's argument validators run before `RunE` is called. If validation fails, `RunE` is never invoked.

| Validator | Guarantee |
|---|---|
| `cobra.ExactArgs(n)` | Errors before RunE if arg count ≠ n |
| `cobra.MinimumNArgs(n)` | Errors before RunE if count < n |
| `cobra.MaximumNArgs(n)` | Errors before RunE if count > n |
| `cobra.NoArgs` | Errors before RunE if any args present |
| `cobra.ArbitraryArgs` | Never errors regardless of count |
| `cobra.MatchAll(v1, v2)` | Combines validators |

**Do not write:** "what happens when I pass 0 args to an `ExactArgs(1)` command." Cobra handles it.

### 3.2 Flag Type Safety

When you declare `cmd.Flags().StringVar(&s, "name", "", "")`, Cobra (via pflag) guarantees `s` is always a string. Passing `--count foo` to an `IntVar` flag returns an error before `RunE`. Do not test this.

### 3.3 Required Flag Enforcement

`cmd.MarkFlagRequired("name")` causes Cobra to return an error before `RunE` if the flag is absent. Do not test "what if the user omits a required flag."

**One exception:** `DisableFlagParsing = true` skips required-flag validation. Document this explicitly if you use it.

### 3.4 Mutual Exclusion Groups

`cmd.MarkFlagsMutuallyExclusive("a", "b")` is enforced by Cobra before `RunE`. Do not test.

---

## Section 4 — What MUST Be Tested at the `cmd/` Layer

These are NOT guaranteed by Cobra. You own them.

### 4.1 Semantic Flag Validation

Cobra validates type and presence. It does not validate meaning. You must test:
- Is a value within legal range? (`--count -1`)
- Does a UUID flag parse as a valid UUID?
- Is a path argument an existing file?
- Does a string flag match an allowed set of values?

### 4.2 Argument Content (Not Just Count)

`ExactArgs(1)` verifies count only. If `args[0]` must be a non-empty string, a valid selector, or a canonical name — test that. The validation typically lives in an extracted helper (`resolveItemSelector`, `resolveLocation`) and is tested there.

### 4.3 Cross-Flag Constraints

Cobra does not validate relationships between flags. If `--project` requires `--to`, that is your responsibility. These constraints typically belong in `PreRunE`; test the `PreRunE` function directly.

### 4.4 Error Propagation

When an internal package returns an error, verify the `cmd/` layer propagates it rather than swallowing it. This catches silent discard bugs in RunE bodies.

### 4.5 Flag-to-Parameter Wiring (at cmd/ boundary only)

Verify that `--json` sets the JSON output mode, that `--as alice` reaches the actor field, that `--temp` produces the correct move type. These wiring tests are the primary value of Level-2 command tests.

### 4.6 Output Routing

Verify human-readable output goes to `cmd.OutOrStdout()` and not `os.Stdout` directly. Verify errors go to `cmd.ErrOrStderr()`. Verify `--json` switches format. These cannot be tested inside `internal/`.

### 4.7 Context Threading

Verify the context passed to `ExecuteContext` (carrying config, database, etc.) reaches the service call. This is especially important for `PersistentPreRunE` hooks that inject values into context.

### 4.8 Subcommand Registration

For the root command and group commands (e.g., `add`, `config`), verify that expected subcommands are registered. Use `cmd.Commands()` and check names. This catches registration omissions.

### 4.9 Singleton Constructor Correctness

When using the singleton `GetXxxCmd()` pattern, verify: (a) it returns a non-nil command, (b) successive calls return the same pointer (`assert.Same`).

---

## Section 5 — Test Patterns

### 5.1 Testing Extracted Business Logic (Primary Pattern)

Create an in-memory SQLite database, call the extracted function, assert on the result struct and database state.

```go
func setupTest(t *testing.T) (*database.Database, context.Context) {
    t.Helper()
    db, err := database.Open(database.Config{
        Path: ":memory:", AutoMigrate: true,
        BusyTimeout: database.DefaultBusyTimeout,
    })
    require.NoError(t, err)
    return db, t.Context()
}

func TestMoveItem_SystemLocationFails(t *testing.T) {
    db, ctx := setupTest(t)
    defer db.Close()
    // seed state...
    _, err := moveItem(ctx, db, itemID, systemLocID, "rehome", "clear", "", "user", "")
    require.ErrorContains(t, err, "cannot move items from system location")
}
```

### 5.2 Testing Helper/Resolver Functions (Table Tests)

For pure resolution logic (`resolveLocation`, `resolveItemSelector`, `looksLikeUUID`), use table-driven tests — compact coverage of all input variants.

```go
tests := []struct {
    name         string
    input        string
    wantID       string
    errAssertion require.ErrorAssertionFunc
}{
    {"resolve by UUID", garageID, garageID, require.NoError},
    {"resolve by canonical name", "garage", garageID, require.NoError},
    {"not found", "nonexistent", "", require.Error},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        gotID, err := resolveLocation(ctx, db, tt.input)
        tt.errAssertion(t, err)
    })
}
```

### 5.3 Testing Config Hooks / `PersistentPreRunE`

Test `initConfig` (or equivalent hook) directly as a function:

```go
func TestInitConfig_SetsContextWithConfig(t *testing.T) {
    testCmd := &cobra.Command{}
    testCmd.SetContext(t.Context())
    testCmd.PersistentFlags().AddFlag(rootCmd.PersistentFlags().Lookup("no-config"))
    require.NoError(t, testCmd.PersistentFlags().Set("no-config", "true"))

    require.NoError(t, initConfig(testCmd, []string{}))

    cfg := testCmd.Context().Value(config.ConfigKey)
    require.NotNil(t, cfg)
    assert.IsType(t, (*config.Config)(nil), cfg)
}
```

Key points:
- Borrow flags from the real root command rather than re-declaring them
- Use `t.Context()` (Go 1.21+) for the base context
- Test environment variable config paths with `t.Setenv`

### 5.4 Testing Command Output (SetOut/SetErr Pattern)

**Critical prerequisite:** production code must write to `cmd.OutOrStdout()` and `cmd.ErrOrStderr()`, never `os.Stdout` directly. `cmd.SetOut` only intercepts `cmd.Print*` methods and `cmd.OutOrStdout()`-routed writes.

```go
func TestAddCmd_JSONOutput(t *testing.T) {
    cmd := NewAddCmd(realOrMockDeps)
    outBuf := &bytes.Buffer{}
    errBuf := &bytes.Buffer{}
    cmd.SetOut(outBuf)
    cmd.SetErr(errBuf)
    cmd.SetArgs([]string{"item-name", "--json"})

    err := cmd.Execute()

    require.NoError(t, err)
    assert.Empty(t, errBuf.String())
    var result map[string]any
    require.NoError(t, json.Unmarshal(outBuf.Bytes(), &result))
    assert.Equal(t, "item-name", result["display_name"])
}
```

### 5.5 Testing Context Propagation

Use `cmd.ExecuteContext(ctx)` (not `Execute()`) in tests that need context values to reach subcommands. Context set via `cmd.SetContext` is overwritten by `ExecuteContext`.

```go
ctx := context.WithValue(t.Context(), config.ConfigKey, testConfig)
err := rootCmd.ExecuteContext(ctx)
```

### 5.6 Singleton Reset Pattern

When using the singleton pattern (package-level `var rootCmd`), reset the global before each test that calls the constructor:

```go
func TestGetRootCmd_HasSubcommands(t *testing.T) {
    rootCmd = nil  // reset singleton
    cmd := GetRootCmd()
    // ...
}
```

**Linting note:** `reassign` is enabled with `patterns: [".*"]`, which flags reassignment of any package-level variable. `rootCmd = nil` in a test may require a directive:

```go
rootCmd = nil //nolint:reassign // reset singleton for test isolation
```

This is a weakness of the singleton approach vs. constructor functions. The constructor (`NewRootCmd()`) approach is preferred for new code — it eliminates global state entirely, each call returns a fully independent instance, and avoids the `reassign` issue. Note: `gochecknoglobals` is explicitly disabled in `.golangci.yml` with the reason `[disabled: Cobra CLI uses globals]`, so the singleton itself is acceptable; only its reassignment in tests requires attention.

---

## Section 6 — Coverage Thresholds

No single Go community standard exists. The following is evidence-informed guidance.

| Layer | Recommended Threshold | Rationale |
|---|---|---|
| `internal/`, `pkg/` | 80–90% | Pure business logic; high testability via function calls |
| `cmd/` | 60–70% | Glue code, flag declarations, RunE bodies inherently hard to cover at unit level |

**Why `cmd/` is harder to cover:**
- Command `Use`, `Short`, `Long` fields are string literals — not executable
- `PersistentPreRunE` chains fire only under specific inheritance conditions
- RunE bodies contain `openDatabase()` calls that require a full test environment

**Note:** `gochecknoinits` is enabled in this project — `init()` functions are forbidden entirely. The general concern about `init()` registering flags at startup (common in scaffolded Cobra projects) does not apply here.

**Correct response to low `cmd/` coverage:** invest in `testscript` integration tests (§7), not in adding more unit tests to hit a number.

**Tool:** [`go-test-coverage`](https://github.com/vladopajic/go-test-coverage) supports per-package threshold overrides via regexp, enabling different thresholds for `cmd/` vs `internal/`.

---

## Section 7 — Integration Tests with testscript (Brief)

`testscript` (extracted from Go's own test infrastructure) enables black-box CLI tests via `.txtar` script files. Each script is a self-contained sandbox — no shared state.

```
# testdata/scripts/add_item.txtar
exec wherehouse add "hammer" --location garage
stdout 'added'
! stderr .
```

This approach provides:
- Documentation-quality tests that read like usage examples
- Zero test pollution between cases
- Coverage integration via Go 1.20+ binary instrumentation (`-cover` + `GOCOVERDIR`)

Use testscript to cover the RunE body and command routing that unit tests cannot reach economically. This is how you achieve meaningful `cmd/` coverage without fighting Cobra's internal state.

Full wiring reference: [rogpeppe/go-internal/testscript](https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript)

---

## Section 8 — Linting Gotchas in Test Code

These rules are active in `.golangci.yml` and directly affect how tests must be written.

### 8.1 `thelper` — Test Helpers Must Call `t.Helper()`

Every `func setupXxx(t *testing.T)` or `func assertXxx(t *testing.T, ...)` helper must call `t.Helper()` as its first statement. Failure causes linting errors and poor test failure output (wrong line numbers).

```go
func setupMoveTest(t *testing.T) (*database.Database, context.Context) {
    t.Helper() // required — must be first
    db, err := database.Open(...)
    require.NoError(t, err)
    return db, t.Context()
}
```

### 8.2 `testifylint` — Required Testify Assertion Styles

The linter enforces specific testify patterns. Common violations:

| Wrong | Correct |
|---|---|
| `assert.Contains(t, err.Error(), "msg")` | `assert.ErrorContains(t, err, "msg")` |
| `assert.True(t, errors.Is(err, target))` | `assert.ErrorIs(t, err, target)` |
| `assert.Equal(t, "", s)` | `assert.Empty(t, s)` |
| `assert.Equal(t, true, condition)` | `assert.True(t, condition)` |
| `assert.Nil(t, err)` for errors | `assert.NoError(t, err)` |

`require` variants follow the same rules (prefer `require.ErrorContains`, `require.ErrorIs`, etc.).

### 8.3 `usetesting` — Prefer Test-Scoped Functions

`os.TempDir()` → `t.TempDir()` (auto-cleanup on test end)

In tests that need a base context, prefer `t.Context()` (Go 1.21+) over `context.Background()`. `t.Context()` is cancelled automatically when the test ends, which catches hanging goroutines.

### 8.4 `mnd` — Magic Numbers in `cobra.ExactArgs(N)`

`cobra.ExactArgs(2)` triggers the magic-number linter. Suppress with a comment that explains the value:

```go
var cmd = &cobra.Command{
    Args: cobra.ExactArgs(2), //nolint:mnd // item selector and destination location
}
```

### 8.5 `govet shadow` (`strict: true`) — Error Variable Shadowing

`govet` runs with `shadow.strict = true`. Re-declaring `err` in an inner scope while an outer `err` exists is flagged:

```go
// WRONG — shadows outer err
err := doFirst()
if err == nil {
    err := doSecond() // flagged: shadows outer err
}

// CORRECT — reuse outer err
err := doFirst()
if err == nil {
    err = doSecond() // ok
}
```

### 8.6 Test File Linter Exclusions

These linters are **disabled for `_test.go` files**. Their constraints do not apply to test code:

| Excluded linter | Effect on test code |
|---|---|
| `dupl` | Duplicate setup code across test functions is fine |
| `funlen` | Long test functions (e.g., large table tests) are fine |
| `errcheck` | Unchecked errors in test setup are fine (prefer `require.NoError` anyway) |
| `gosec` | Security checks relaxed (e.g., `#nosec` not needed for `t.TempDir()`) |
| `goconst` | Repeated string literals in tests are fine |

### 8.7 `gochecknoinits` — No `init()` Functions

`init()` is **forbidden project-wide**. Command registration and flag setup must happen in constructor functions (`GetXxxCmd()`, `NewXxxCmd()`), not in `init()`. This is already enforced; any test advice about `init()` side effects is irrelevant here.

---

## Section 9 — Anti-Patterns

| Anti-pattern | Why Wrong | Correct Approach |
|---|---|---|
| Testing `ExactArgs(n)` behavior | Cobra guarantees this | Delete the test |
| Testing required-flag omission | Cobra guarantees this | Delete the test |
| Testing flag type rejection (`--count foo`) | pflag guarantees this | Delete the test |
| Using `Run` instead of `RunE` | `Run` cannot return errors; tests must call `os.Exit` handlers | Always use `RunE`; return errors instead of calling `os.Exit` |
| `fmt.Printf` in production RunE | `cmd.SetOut` won't capture it | Use `fmt.Fprintln(cmd.OutOrStdout(), ...)` |
| `cmd.Execute()` without resetting singleton | Prior test state bleeds in | Reset global or use constructor function |
| Calling `cmd.SetContext(ctx)` before `cmd.Execute()` | Execute overwrites it | Use `cmd.ExecuteContext(ctx)` |
| Placing domain invariants in RunE | Hard to test, untestable without Cobra | Extract to internal function |
| Skipping tests with `t.Skip` for flag helpers | Indicates untestable design | Extract flag reading from cobra into testable function |
| Package-level flag variables (`var actor string`) | Retain values across tests in same package | Bind to struct fields; create new struct per test via constructor |
