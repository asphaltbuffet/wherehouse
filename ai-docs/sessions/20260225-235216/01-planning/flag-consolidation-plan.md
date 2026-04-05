# Flag Consolidation Implementation Plan

**Session**: 20260225-235216
**Date**: 2026-02-26
**Author**: golang-architect
**Status**: Ready for implementation

---

## Problem Summary

`bindFlagsToConfig` in `cmd/root.go` already writes `--json` and `--quiet` flag values into
`cfg.Output` during `PersistentPreRunE`. But every command then ignores `cfg` and re-reads flags
directly, producing three concrete bugs:

1. **Config file values never honored**: `output.default_format` and `output.quiet` in config files
   are written into `cfg.Output` by `bindFlagsToConfig` but never read back by any command handler.

2. **`--quiet` broken on all config subcommands** (CRITICAL): `cmd/config/{init,get,set,edit,check,path}.go`
   all call `cmd.Flags().GetBool("quiet")`, but the root declares `quiet` as `CountP` not `Bool`.
   `GetBool` on a `CountP` flag returns `false` always (type mismatch, error silently discarded).

3. **`--json` duplicate local declarations** (MEDIUM): `cmd/find/find.go`, `cmd/scry/scry.go`, and
   `cmd/config/get.go` each redeclare `--json` as a local flag, shadowing the root persistent flag
   and creating divergent help text and maintenance burden.

---

## Design Decisions

### Decision 1: Quiet level -- binary bool vs. preserving count

**Choice: Preserve the count level in `OutputConfig`, expose it through `IsQuiet() bool` and
`QuietLevel() int` methods.**

Rationale:
- `--quiet` is already declared as `CountP` with documented semantics (`-q`=1, `-qq`=2).
- `bindFlagsToConfig` currently collapses to `bool`, discarding the count. This is a lossy
  transformation that prevents future use of `-qq` for truly silent output.
- Changing `OutputConfig.Quiet` from `bool` to `int` is a one-field struct change with no
  downstream breakage since `cfg.Output.Quiet` is only written in `bindFlagsToConfig` and
  never read directly by any command (they all re-read flags instead, which is exactly the
  problem we are fixing).
- `IsQuiet() bool` returns `cfg.Output.Quiet > 0` -- this is the predicate used for "should
  suppress non-essential output".
- `QuietLevel() int` returns the raw count -- available for commands that want to distinguish
  `-q` vs `-qq` in the future (e.g., fully silent JSON-only output at level 2).
- The config file field `output.quiet` currently accepts `bool`. We will change it to `int`
  in the TOML struct. Callers using `output.quiet = true` in their config files will receive
  a TOML parse error (explicit, not silent). This is consistent with the project's "no silent
  repair" principle. The migration message should be clear.

  Alternative considered: Keep `Quiet bool` in TOML struct, add a separate `QuietLevel int`
  field. Rejected as it creates two fields for one concept and `bool` maps cleanly to count 1.

### Decision 2: `cli.NewOutputWriterFromConfig` convenience constructor

**Choice: Add `cli.NewOutputWriterFromConfig(out, err io.Writer, cfg *config.Config) *OutputWriter`.**

Rationale:
- Every command `RunE` function today does a two-line boilerplate block:
  ```go
  jsonMode, _ := cmd.Flags().GetBool("json")
  quietMode := cli.IsQuietMode(cmd)
  out := cli.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), jsonMode, quietMode)
  ```
  After migration this becomes:
  ```go
  cfg := cli.MustGetConfig(cmd.Context())
  out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)
  ```
- The existing `cli.NewOutputWriter(out, err, jsonMode, quietMode bool)` is kept as-is for
  callers that legitimately have booleans at hand (tests, future flexibility). The new function
  is purely additive.
- `cli` package imports `config` package. This is an existing dependency: `cli.GetActorUserID`
  already reads `config.ConfigKey` from context, and `cli.OpenDatabase` does the same. Adding
  the import here is consistent with existing usage.

### Decision 3: Migration path for commands without cfg in context

**Commands that do not currently retrieve cfg from context: `find`, `scry`, `history`.**

Looking at the code:
- `find/find.go`: Does not call `cli.GetActorUserID` or `cli.OpenDatabase` directly -- it
  calls the package-local `openDatabase(ctx)` wrapper. Has no cfg usage at all today.
- `scry/scry.go`: Uses `cli.OpenDatabase(ctx)` and `cli.ResolveItemSelector`. No cfg usage.
- `history/history.go`: Uses `cli.OpenDatabase(ctx)`. Reads `--json` via persistent flag
  correctly today but does not use cfg.

**Choice: All three commands will retrieve cfg from context using the same pattern as `config/get.go`.**

The pattern already exists in the codebase:
```go
cfgVal := cmd.Context().Value(config.ConfigKey)
cfg, ok := cfgVal.(*config.Config)
```

We will introduce a helper `cli.GetConfig(ctx context.Context) (*config.Config, bool)` and
`cli.MustGetConfig(ctx context.Context) *config.Config` (panics with a clear message if nil,
which should never happen given `initConfig` always sets it). This avoids duplicate type assertion
boilerplate across all commands.

`MustGetConfig` is safe here because:
- `initConfig` is `PersistentPreRunE` -- it always runs before any `RunE`.
- The only way cfg is nil in context is a programmer error (e.g., bypassed `PersistentPreRunE`
  in tests), which should panic, not silently misbehave.
- Commands that do not read cfg today (find, scry, history) already have the ctx passed through;
  they just need to call `cli.MustGetConfig(ctx)` to get it.

### Decision 4: Do NOT change `cli.IsQuietMode(cmd *cobra.Command)`

`cli.IsQuietMode` correctly uses `GetCount("quiet")` and is used by `add`, `move`, `loan`, `lost`.
It is not broken. We keep it as-is for backward compatibility. After the migration, commands will
use `cfg.IsQuiet()` instead (which reads from the config struct), but we do not delete
`IsQuietMode` -- it remains valid for any caller that has a `*cobra.Command` but not a `*Config`.

---

## Exact Changes by File

### Batch 1: `internal/config/config.go` (golang-developer)

**Change 1a: Modify `OutputConfig.Quiet` from `bool` to `int`**

```go
// Before:
type OutputConfig struct {
    DefaultFormat string `mapstructure:"default_format" toml:"default_format"`
    Quiet         bool   `mapstructure:"quiet"          toml:"quiet"`
}

// After:
type OutputConfig struct {
    DefaultFormat string `mapstructure:"default_format" toml:"default_format"`
    Quiet         int    `mapstructure:"quiet"          toml:"quiet"`
}
```

**Change 1b: Add accessor methods on `*Config`**

Add to `internal/config/config.go`:

```go
// IsQuiet returns true if quiet mode is enabled (at any level).
// Corresponds to the user passing -q or setting output.quiet >= 1 in config.
func (c *Config) IsQuiet() bool {
    return c.Output.Quiet > 0
}

// QuietLevel returns the quiet suppression level.
// 0 = normal output, 1 = minimal (-q), 2+ = silent (-qq).
func (c *Config) QuietLevel() int {
    return c.Output.Quiet
}

// IsJSON returns true if JSON output format is active.
// Corresponds to the user passing --json or setting output.default_format = "json" in config.
func (c *Config) IsJSON() bool {
    return c.Output.DefaultFormat == "json"
}
```

**Change 1c: Update `bindFlagsToConfig` in `cmd/root.go`**

```go
// Before:
if cmd.Flags().Changed("quiet") {
    cfg.Output.Quiet = true
}

// After:
if cmd.Flags().Changed("quiet") {
    if count, err := cmd.Flags().GetCount("quiet"); err == nil {
        cfg.Output.Quiet = count
    }
}
```

**Change 1d: Update `applyDefaults` in `internal/config/` (wherever defaults are set)**

The `Quiet` default should be 0 (zero value, no change needed if defaults aren't explicitly set).
Verify `GetDefaults()` does not explicitly set `Quiet: false` (bool) -- if it does, update to `0`.

### Batch 2: `internal/cli/` -- add helpers (golang-developer)

**Change 2a: Add `GetConfig` and `MustGetConfig` to `internal/cli/flags.go`**

```go
import (
    "context"
    "github.com/asphaltbuffet/wherehouse/internal/config"
    "github.com/spf13/cobra"
)

// GetConfig retrieves the Config from the command context.
// Returns (cfg, true) if found, (nil, false) if not present.
func GetConfig(ctx context.Context) (*config.Config, bool) {
    v := ctx.Value(config.ConfigKey)
    cfg, ok := v.(*config.Config)
    return cfg, ok
}

// MustGetConfig retrieves the Config from context, panicking if not present.
// This should never panic in production because initConfig always stores cfg
// before any RunE is called. A panic here indicates a programmer error.
func MustGetConfig(ctx context.Context) *config.Config {
    cfg, ok := GetConfig(ctx)
    if !ok {
        panic("wherehouse: Config not found in context -- was PersistentPreRunE bypassed?")
    }
    return cfg
}
```

**Change 2b: Add `NewOutputWriterFromConfig` to `internal/cli/output.go`**

```go
import "github.com/asphaltbuffet/wherehouse/internal/config"

// NewOutputWriterFromConfig creates an OutputWriter using settings from cfg.
// This is the preferred constructor for command RunE functions that have
// already retrieved cfg from context.
func NewOutputWriterFromConfig(out, err io.Writer, cfg *config.Config) *OutputWriter {
    return NewOutputWriter(out, err, cfg.IsJSON(), cfg.IsQuiet())
}
```

Note: `internal/cli` already imports `config` indirectly through the context key pattern in
`GetActorUserID` and `OpenDatabase`. Adding explicit import is safe.

### Batch 3: `cmd/` subcommand migrations (golang-ui-developer)

All changes in Batch 3 are independent of each other and can be applied in any order.
The pattern is uniform across all affected files.

**Migration pattern (applies to all commands):**

```go
// REMOVE these lines (or the equivalent flag-reading pair):
jsonMode, _ := cmd.Flags().GetBool("json")
quietMode := cli.IsQuietMode(cmd)           // OR: quietMode, _ := cmd.Flags().GetBool("quiet")
out := cli.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), jsonMode, quietMode)

// REPLACE with:
cfg := cli.MustGetConfig(cmd.Context())
out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)
```

Additionally, any local `if jsonMode {` branches that check the local variable must be updated
to use `cfg.IsJSON()`.

---

#### File: `cmd/config/init.go`

Affected lines: 53-55 (flag reading), 54 is the broken `GetBool("quiet")`.

Remove:
```go
jsonMode, _ := cmd.Flags().GetBool("json")
quietMode, _ := cmd.Flags().GetBool("quiet")
out := cli.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), jsonMode, quietMode)
```

Add after `local, _ := cmd.Flags().GetBool("local")`:
```go
cfg := cli.MustGetConfig(cmd.Context())
out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)
```

No other `jsonMode` or `quietMode` variable uses exist in `runInit` -- clean replacement.

---

#### File: `cmd/config/get.go`

Two separate issues: broken `GetBool("quiet")` at line 52, and duplicate local `--json` flag
at line 43.

Step 1: Remove local `--json` declaration in `GetGetCmd`:
```go
// REMOVE:
getCmd.Flags().Bool("json", false, "output in JSON format")
```

Step 2: In `runGet`, replace:
```go
jsonOutput, _ := cmd.Flags().GetBool("json")
showSources, _ := cmd.Flags().GetBool("sources")
quietMode, _ := cmd.Flags().GetBool("quiet")
out := cli.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), jsonOutput, quietMode)
```

With:
```go
showSources, _ := cmd.Flags().GetBool("sources")
cfg := cli.MustGetConfig(cmd.Context())
out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)
```

Step 3: Replace all `if jsonOutput {` with `if cfg.IsJSON() {` (3 occurrences in `runGet`).

Note: `runGet` already retrieves `cfg` from context further down for `config.GetValue()`.
After this change there will be two places that read cfg from context in the same function.
Deduplicate by moving the `cli.MustGetConfig(cmd.Context())` call to the top of `runGet` and
removing the later `cmd.Context().Value(config.ConfigKey)` type-assertion block:
```go
// Replace this existing block:
cfg := cmd.Context().Value(config.ConfigKey)
if cfg == nil {
    return errors.New("configuration not loaded")
}
globalConfig, ok := cfg.(*config.Config)
if !ok {
    return errors.New("invalid configuration type in context")
}

// With:
globalConfig := cli.MustGetConfig(cmd.Context())
```

This simplifies `runGet` considerably.

---

#### File: `cmd/config/set.go`

Affected lines: 52-54.

Remove:
```go
jsonMode, _ := cmd.Flags().GetBool("json")
quietMode, _ := cmd.Flags().GetBool("quiet")
out := cli.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), jsonMode, quietMode)
```

Add:
```go
cfg := cli.MustGetConfig(cmd.Context())
out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)
```

No `jsonMode` variable uses remain after removal.

---

#### File: `cmd/config/edit.go`

Affected lines: 53-55.

Remove:
```go
jsonMode, _ := cmd.Flags().GetBool("json")
quietMode, _ := cmd.Flags().GetBool("quiet")
out := cli.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), jsonMode, quietMode)
```

Add:
```go
cfg := cli.MustGetConfig(cmd.Context())
out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)
```

No `jsonMode` variable uses remain after removal.

---

#### File: `cmd/config/check.go`

Affected lines: 42-44.

Remove:
```go
jsonMode, _ := cmd.Flags().GetBool("json")
quietMode, _ := cmd.Flags().GetBool("quiet")
out := cli.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), jsonMode, quietMode)
```

Add:
```go
cfg := cli.MustGetConfig(cmd.Context())
out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)
```

No `jsonMode` variable uses remain after removal.

---

#### File: `cmd/config/path.go`

Affected lines: 117-119.

Remove:
```go
jsonMode, _ := cmd.Flags().GetBool("json")
quietMode, _ := cmd.Flags().GetBool("quiet")
out := cli.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), jsonMode, quietMode)
```

Add:
```go
cfg := cli.MustGetConfig(cmd.Context())
out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)
```

No `jsonMode` variable uses remain after removal.

---

#### File: `cmd/find/find.go`

Two issues: duplicate local `--json` at line 51, and direct `jsonMode` variable use in `runFind`.

Step 1: Remove local `--json` declaration in `GetFindCmd`:
```go
// REMOVE:
findCmd.Flags().Bool("json", false, "Output as JSON")
```

Step 2: In `runFind`, replace:
```go
limit, _ := cmd.Flags().GetInt("limit")
verbose, _ := cmd.Flags().GetBool("verbose")
jsonMode, _ := cmd.Flags().GetBool("json")
```

With:
```go
limit, _ := cmd.Flags().GetInt("limit")
verbose, _ := cmd.Flags().GetBool("verbose")
cfg := cli.MustGetConfig(ctx)
```

Step 3: Replace `if jsonMode {` with `if cfg.IsJSON() {` (1 occurrence in `runFind`).

Note: `find` does not construct an `OutputWriter` today -- it uses raw `fmt.Fprintf` and its own
`outputJSON`/`outputHuman` helpers. This plan does NOT refactor the find output layer. The only
change here is fixing the json flag source from the local shadow to `cfg.IsJSON()`.

---

#### File: `cmd/scry/scry.go`

Two issues: duplicate local `--json` at line 47, and direct `jsonMode` variable use in `runScry`.

Step 1: Remove local `--json` declaration in `GetScryCmd`:
```go
// REMOVE:
scryCmd.Flags().Bool("json", false, "Output as JSON")
```

Step 2: In `runScry`, replace:
```go
verbose, _ := cmd.Flags().GetBool("verbose")
jsonMode, _ := cmd.Flags().GetBool("json")
```

With:
```go
verbose, _ := cmd.Flags().GetBool("verbose")
cfg := cli.MustGetConfig(ctx)
```

Step 3: Replace `if jsonMode {` with `if cfg.IsJSON() {` (1 occurrence in `runScry`).

---

#### File: `cmd/move/item.go`

Affected lines: 70-72. Currently uses `cli.IsQuietMode(cmd)` correctly (not broken), but should
migrate to config for consistency so config file values are honored.

Replace:
```go
jsonMode, _ := cmd.Flags().GetBool("json")
quietMode := cli.IsQuietMode(cmd)
out := cli.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), jsonMode, quietMode)
```

With:
```go
cfg := cli.MustGetConfig(ctx)
out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)
```

Also update the `if !jsonMode {` check at line 96 and `if jsonMode {` at line 103:
```go
// Replace:
if !jsonMode {
    out.Success(...)
}
if jsonMode {
    ...
}

// With:
if !cfg.IsJSON() {
    out.Success(...)
}
if cfg.IsJSON() {
    ...
}
```

---

#### File: `cmd/add/item.go`

Affected lines: 77-79. Currently correct (not broken), migrate for consistency.

Replace:
```go
jsonMode, _ := cmd.Flags().GetBool("json")
quietMode := cli.IsQuietMode(cmd)
out := cli.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), jsonMode, quietMode)
```

With:
```go
cfg := cli.MustGetConfig(ctx)
out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)
```

---

#### File: `cmd/add/location.go`

Same pattern as `add/item.go`. Check lines ~78-80 for the flag-reading block and apply same
replacement.

---

#### File: `cmd/loan/item.go` (or equivalent RunE file)

Same pattern. Check lines ~51-53. Apply same replacement.

---

#### File: `cmd/lost/item.go` (or equivalent RunE file)

Same pattern. Check lines ~52-54. Apply same replacement.

---

#### File: `cmd/history/history.go`

Currently reads `--json` via persistent flag correctly but does not suppress quiet. Migrate for
consistency.

Replace the flag-reading lines (around line 58) with `cfg`-based approach.

---

### Batch 4: Tests (golang-tester)

**Test additions for `internal/config/config.go` methods:**

File: `internal/config/config_test.go` (or new `internal/config/output_test.go`)

Test cases needed:
- `IsQuiet()` returns `false` when `Quiet == 0`
- `IsQuiet()` returns `true` when `Quiet == 1`
- `IsQuiet()` returns `true` when `Quiet == 2`
- `QuietLevel()` returns exact count value
- `IsJSON()` returns `false` when `DefaultFormat == ""`
- `IsJSON()` returns `false` when `DefaultFormat == "text"`
- `IsJSON()` returns `true` when `DefaultFormat == "json"`

**Test additions for `internal/cli/flags.go` helpers:**

File: `internal/cli/flags_test.go`

Test cases for `MustGetConfig`:
- Panics when context has no config key
- Returns cfg when present in context

Test cases for `GetConfig`:
- Returns (nil, false) when context has no config key
- Returns (cfg, true) when present

**Test additions for `internal/cli/output.go`:**

File: `internal/cli/output_test.go`

Test `NewOutputWriterFromConfig` using table-driven tests:
- `IsJSON() = true` sets json mode
- `IsJSON() = false` clears json mode
- `IsQuiet() = true` sets quiet mode
- `IsQuiet() = false` clears quiet mode

**Regression tests for config subcommand quiet bug fix:**

File: `cmd/config/init_test.go` (note: already modified per git status)

Add test cases:
- `wherehouse config init` with `-q` flag: verify output is suppressed (success message not printed)
- `wherehouse config init` without `-q` flag: verify output is present
- Apply same pattern to `config/get_test.go`, `config/set_test.go`, etc. if they exist

---

## Implementation Batches

### Batch 1 (golang-developer) -- Foundation

**Files**: `internal/config/config.go`, `cmd/root.go`
**Changes**:
1. Change `OutputConfig.Quiet` from `bool` to `int`
2. Add `IsQuiet()`, `QuietLevel()`, `IsJSON()` methods on `*Config`
3. Update `bindFlagsToConfig` to use `GetCount("quiet")`
4. Verify `GetDefaults()` and `applyDefaults` handle `int` zero value correctly
**Estimated diff**: ~30 lines

### Batch 2 (golang-developer) -- CLI Helpers

**Files**: `internal/cli/flags.go`, `internal/cli/output.go`
**Depends on**: Batch 1 (needs `config.Config.IsQuiet()` and `IsJSON()`)
**Changes**:
1. Add `GetConfig(ctx)` and `MustGetConfig(ctx)` to `flags.go`
2. Add `NewOutputWriterFromConfig` to `output.go`
**Estimated diff**: ~25 lines

### Batch 3 (golang-ui-developer) -- Command Migrations

**Files** (all in `cmd/`):
- `cmd/config/init.go`
- `cmd/config/get.go`
- `cmd/config/set.go`
- `cmd/config/edit.go`
- `cmd/config/check.go`
- `cmd/config/path.go`
- `cmd/find/find.go`
- `cmd/scry/scry.go`
- `cmd/move/item.go`
- `cmd/add/item.go`
- `cmd/add/location.go`
- `cmd/loan/item.go` (verify RunE file name)
- `cmd/lost/item.go` (verify RunE file name)
- `cmd/history/history.go`

**Depends on**: Batch 2 (needs `cli.MustGetConfig` and `cli.NewOutputWriterFromConfig`)
**Can be parallelized**: All 14 files are independent of each other within Batch 3.
**Estimated diff**: ~80 lines removed, ~50 lines added

### Batch 4 (golang-tester) -- Tests

**Files**:
- `internal/config/config_test.go` or new `internal/config/output_test.go`
- `internal/cli/flags_test.go`
- `internal/cli/output_test.go`
- `cmd/config/init_test.go` (already modified per git status)
- Other `cmd/config/*_test.go` files if they exist

**Depends on**: Batches 1 and 2 (tests the new methods/helpers)
**Estimated additions**: ~60-80 test cases

---

## Dependency Graph

```
Batch 1 (config.go + root.go)
    └── Batch 2 (cli helpers)
            └── Batch 3 (cmd/ migrations)  ← all 14 files in parallel
                    └── Batch 4 (tests)
```

Batches 1 and 2 could technically be written in parallel by different agents, but since Batch 2
imports Batch 1's new method signatures, Batch 1 must compile first. Sequential is safer.

---

## Files NOT Changed

The following files from the flag review are deliberately left unchanged:

- `cli.IsQuietMode(cmd)` -- Kept as-is. It works correctly. Commands migrated away from it but
  it remains valid for future callers.
- `cmd/history/history.go` lines that read `--limit`, `--since`, `--oldest-first` -- These are
  local flags, not output flags. No change.
- Any command reading `--config`, `--no-config`, `--db`, `--as` -- These are not output flags
  and are already handled correctly by `bindFlagsToConfig`.
- The `history` command's JSON branching -- history does not use `OutputWriter` today. Migrate
  the `jsonMode` variable to `cfg.IsJSON()` but do not restructure the output logic.

---

## Trade-offs and Alternatives Considered

### Alternative: Keep `Quiet bool` in config, add separate `QuietLevel` field

Rejected. Two fields for one concept creates inconsistency. The TOML schema would have both
`output.quiet = true` (bool) and `output.quiet_level = 2` (int) which is confusing. Single int
field that maps `0=false, 1+=true` is clean and sufficient.

### Alternative: Read from cfg AND fall back to flag if cfg is zero-value

Rejected. The `bindFlagsToConfig` contract already handles priority correctly: it only writes to
cfg when the flag is `Changed`. Commands reading from cfg get the merged result (flag override wins
over config file, exactly as intended). Adding fallback logic in commands would duplicate what
`bindFlagsToConfig` already does correctly.

### Alternative: Add `cfg.NewOutputWriter(out, err io.Writer) *cli.OutputWriter` method on Config

Rejected. This would create a dependency from `internal/config` → `internal/cli`, which is a
circular dependency (cli already imports config). The `NewOutputWriterFromConfig` constructor on
the `cli` side is the correct direction.

### Alternative: Only fix the critical bug (`--quiet` on config subcommands) without the full migration

Valid minimal approach, but the user explicitly requested the full migration. The minimal fix is 6
one-line changes (`GetBool` → `cli.IsQuietMode`). The full migration is ~130 lines across 14 files
but achieves the stated goal of config as single source of truth.

---

## Verification Checklist

After implementation:

- [ ] `wherehouse config init -q` suppresses output (was broken, now fixed)
- [ ] `wherehouse config get -q` suppresses output (was broken, now fixed)
- [ ] `wherehouse config set -q` suppresses output (was broken, now fixed)
- [ ] `wherehouse config edit -q` suppresses output (was broken, now fixed)
- [ ] `wherehouse config check -q` suppresses output (was broken, now fixed)
- [ ] `wherehouse config path -q` suppresses output (was broken, now fixed)
- [ ] `wherehouse find --json` produces JSON (was working, still works)
- [ ] `wherehouse scry --json` produces JSON (was working, still works)
- [ ] `wherehouse --json find` (flag before subcommand) produces JSON (was ambiguous, now works cleanly via cfg)
- [ ] Config file with `output.default_format = "json"` causes JSON output (new capability)
- [ ] Config file with `output.quiet = 1` causes quiet output (new capability)
- [ ] `go test ./...` passes with zero failures
- [ ] `golangci-lint run` produces zero errors
