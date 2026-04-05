# Flag Declaration and Usage Review

**Date**: 2026-02-26
**Scope**: All commands in `/cmd/` directory
**Focus**: Persistent flag duplication, correct flag access patterns

---

## 1. Root Persistent Flags (Available to ALL Subcommands)

The root command (`/cmd/root.go:49-58`) declares these persistent flags:

| Flag | Type | Short | Default | Line |
|------|------|-------|---------|------|
| `config` | String | `-c` | `""` | 50 |
| `no-config` | Bool | | `false` | 51 |
| `db` | String | | `""` | 55 |
| `as` | String | | `""` | 56 |
| `json` | Bool | | `false` | 57 |
| `quiet` | Count | `-q` | `0` | 58 |

All six are **persistent** flags, meaning every subcommand inherits them automatically via Cobra's `cmd.Flags()` lookup chain.

The `bindFlagsToConfig` function (`root.go:75-92`) applies `--db`, `--as`, `--json`, and `--quiet` onto the `*Config` struct during `PersistentPreRunE`.

---

## 2. Subcommand Local Flag Declarations

### Flag Declaration Inventory

| Command | File:Line | Flag | Type | Scope | Issue? |
|---------|-----------|------|------|-------|--------|
| `add` | add/add.go | (none) | | | |
| `add item` | add/item.go:40 | `in` | StringP | Local | Clean |
| `add location` | add/location.go:39 | `in` | StringP | Local | Clean |
| `config` | config/config.go | (none) | | | |
| `config init` | config/init.go:40 | `local` | Bool | Local | Clean |
| `config init` | config/init.go:41 | `force` | BoolP | Local | Clean |
| `config get` | config/get.go:43 | **`json`** | Bool | Local | **DUPLICATE** |
| `config get` | config/get.go:44 | `sources` | Bool | Local | Clean |
| `config set` | config/set.go:41 | `local` | Bool | Local | Clean |
| `config edit` | config/edit.go:42 | `local` | Bool | Local | Clean |
| `config edit` | config/edit.go:43 | `global` | Bool | Local | Clean |
| `config path` | config/path.go:36 | `all` | Bool | Local | Clean |
| `config check` | config/check.go | (none) | | | |
| `find` | find/find.go:49 | `limit` | IntP | Local | Clean |
| `find` | find/find.go:50 | `verbose` | BoolP | Local | Clean |
| `find` | find/find.go:51 | **`json`** | Bool | Local | **DUPLICATE** |
| `history` | history/history.go:41 | `id` | StringP | Local | Clean |
| `history` | history/history.go:42 | `limit` | IntP | Local | Clean |
| `history` | history/history.go:43 | `since` | String | Local | Clean |
| `history` | history/history.go:44 | `oldest-first` | Bool | Local | Clean |
| `loan` | loan/loan.go:47 | `to` | String | Local | Clean |
| `loan` | loan/loan.go:51 | `note` | StringP | Local | Clean |
| `lost` | lost/lost.go:40 | `note` | StringP | Local | Clean |
| `move` | move/move.go:50 | `to` | StringP | Local | Clean |
| `move` | move/move.go:54 | `temp` | Bool | Local | Clean |
| `move` | move/move.go:57 | `project` | String | Local | Clean |
| `move` | move/move.go:58 | `keep-project` | Bool | Local | Clean |
| `move` | move/move.go:59 | `clear-project` | Bool | Local | Clean |
| `move` | move/move.go:65 | `note` | StringP | Local | Clean |
| `scry` | scry/scry.go:46 | `verbose` | BoolP | Local | Clean |
| `scry` | scry/scry.go:47 | **`json`** | Bool | Local | **DUPLICATE** |

---

## 3. Duplicated Flags

### `--json` declared locally on 3 subcommands (shadows root persistent flag)

| File | Line | Declaration |
|------|------|-------------|
| `/cmd/find/find.go` | 51 | `findCmd.Flags().Bool("json", false, "Output as JSON")` |
| `/cmd/scry/scry.go` | 47 | `scryCmd.Flags().Bool("json", false, "Output as JSON")` |
| `/cmd/config/get.go` | 43 | `getCmd.Flags().Bool("json", false, "output in JSON format")` |

**Root already declares**: `rootCmd.PersistentFlags().Bool("json", false, "machine-readable JSON output")` at `root.go:57`.

**Impact**: When a local flag shadows a persistent flag with the same name, Cobra resolves `cmd.Flags().GetBool("json")` to the **local** flag, not the persistent one. This means `bindFlagsToConfig` in `root.go` will NOT see the local flag's value -- it queries `cmd.Flags().Changed("json")` on the root command's flag set during `PersistentPreRunE`, which runs on the subcommand's `cmd`. However, since `cmd.Flags()` merges local + persistent, `Changed("json")` will actually see the local flag if set.

**Behavior analysis**: In practice this works because Cobra's `cmd.Flags()` is a merged FlagSet that includes both local and persistent. When the user passes `--json` on `find`, the local flag gets set, and `cmd.Flags().Changed("json")` returns `true`. So `bindFlagsToConfig` applies it to config. Then `runFind` also reads it via `cmd.Flags().GetBool("json")` which also returns the local flag's value. Both see the same value.

**However**, the duplication is still problematic:
1. **Inconsistent help text**: Root says "machine-readable JSON output", `find` says "Output as JSON", `config get` says "output in JSON format"
2. **Maintenance burden**: Three places to update if the flag behavior changes
3. **Confusion**: Developers may not realize the persistent flag already exists
4. **`config get` edge case**: If a user passes `--json` before `config get` (e.g., `wherehouse --json config get`), the persistent flag is set. But `config get` has its own local `--json` which would be `false`. The local flag wins for `runGet`'s `GetBool("json")`, but the persistent flag is what `bindFlagsToConfig` sees. This creates a split where config says JSON but the command handler says no-JSON.

**Severity**: MEDIUM -- works in the common case but creates subtle inconsistencies and maintenance debt.

---

## 4. Flag Reading Patterns

### Pattern A: `cmd.Flags().GetBool("json")` (reading persistent flag via merged FlagSet)

Used by: `add/item.go:77`, `add/location.go:78`, `loan/item.go:51`, `lost/item.go:52`, `move/item.go:70`, `history/history.go:58`

This is the **correct** pattern for reading persistent flags from a subcommand. Cobra merges persistent flags into `cmd.Flags()`, so this works.

### Pattern B: `cmd.Flags().GetBool("json")` where local `--json` shadows persistent

Used by: `find/find.go:66`, `scry/scry.go:57`, `config/get.go:50`

These read their own local `--json` flag, not the root persistent one.

### Pattern C: `cmd.Flags().GetBool("quiet")` / `cmd.Flags().GetCount("quiet")`

- `cli.IsQuietMode(cmd)` uses `cmd.Flags().GetCount("quiet")` -- **correct**, reads root persistent CountP flag
- `config/init.go:54`, `config/edit.go:54`, `config/check.go:43`, `config/set.go:53`, `config/get.go:52`, `config/path.go:118` all use `cmd.Flags().GetBool("quiet")` -- **BUG**

**BUG**: The root declares `quiet` as `CountP` (line 58), not `Bool`. Reading it with `GetBool("quiet")` will return `false` always because the flag type is `Count`, not `Bool`. `GetBool` on a `Count` flag returns the zero value with a type mismatch error (silently discarded by the `_` assignment).

This means **all config subcommands ignore `--quiet`**.

| File | Line | Code | Issue |
|------|------|------|-------|
| `/cmd/config/init.go` | 54 | `quietMode, _ := cmd.Flags().GetBool("quiet")` | **BUG: reads Count as Bool** |
| `/cmd/config/edit.go` | 54 | `quietMode, _ := cmd.Flags().GetBool("quiet")` | **BUG: reads Count as Bool** |
| `/cmd/config/check.go` | 43 | `quietMode, _ := cmd.Flags().GetBool("quiet")` | **BUG: reads Count as Bool** |
| `/cmd/config/set.go` | 53 | `quietMode, _ := cmd.Flags().GetBool("quiet")` | **BUG: reads Count as Bool** |
| `/cmd/config/get.go` | 52 | `quietMode, _ := cmd.Flags().GetBool("quiet")` | **BUG: reads Count as Bool** |
| `/cmd/config/path.go` | 118 | `quietMode, _ := cmd.Flags().GetBool("quiet")` | **BUG: reads Count as Bool** |

The correct approach is used by `add`, `move`, `loan`, and `lost`: they call `cli.IsQuietMode(cmd)` which correctly uses `GetCount("quiet")`.

### Pattern D: Reading `--config` and `--no-config` from subcommands

- `config/init.go:50`: `cmd.Flags().GetString("config")` -- correct (reads persistent via merged FlagSet)
- `config/path.go:114`: `cmd.Flags().GetBool("no-config")` -- correct
- `config/path.go:133`: `cmd.Flags().GetString("config")` -- correct

---

## 5. Commands Not Using Config for JSON/Quiet

After `bindFlagsToConfig` runs, `cfg.Output.DefaultFormat` and `cfg.Output.Quiet` are set. However, **no subcommand reads from config** for these values. They all re-read the flags directly.

This is not necessarily wrong -- the config object is meant for values that come from config files, and `bindFlagsToConfig` is more of a "flag wins over config" mechanism. The commands reading flags directly is the expected pattern for CLI flags that override config.

However, this means the config file's `output.default_format = "json"` setting is **never honored** by commands that read `--json` directly from flags (which is all of them). Only `bindFlagsToConfig` writes to config, but nothing reads it back. This is a design gap rather than a flag duplication issue, but worth noting.

---

## 6. Summary of Issues

### CRITICAL (Bug)

**`--quiet` broken on all config subcommands**: 6 files read `CountP` flag with `GetBool()`, which silently fails and always returns `false`.

| File | Line | Fix |
|------|------|-----|
| `cmd/config/init.go` | 54 | Replace `cmd.Flags().GetBool("quiet")` with `cli.IsQuietMode(cmd)` |
| `cmd/config/edit.go` | 54 | Replace `cmd.Flags().GetBool("quiet")` with `cli.IsQuietMode(cmd)` |
| `cmd/config/check.go` | 43 | Replace `cmd.Flags().GetBool("quiet")` with `cli.IsQuietMode(cmd)` |
| `cmd/config/set.go` | 53 | Replace `cmd.Flags().GetBool("quiet")` with `cli.IsQuietMode(cmd)` |
| `cmd/config/get.go` | 52 | Replace `cmd.Flags().GetBool("quiet")` with `cli.IsQuietMode(cmd)` |
| `cmd/config/path.go` | 118 | Replace `cmd.Flags().GetBool("quiet")` with `cli.IsQuietMode(cmd)` |

Example fix for `cmd/config/init.go`:
```go
// Before (broken):
quietMode, _ := cmd.Flags().GetBool("quiet")

// After (correct):
quietMode := cli.IsQuietMode(cmd)
```

This requires adding `"github.com/asphaltbuffet/wherehouse/internal/cli"` to the import in files that don't already have it (all config subcommands already import it).

### MEDIUM (Duplication)

**`--json` declared locally on 3 subcommands**: `find`, `scry`, `config get` redeclare `--json` as a local flag when it already exists as a persistent flag on root.

**Recommended fix**: Remove the local `--json` declarations from these 3 commands. The persistent flag is already accessible. No code changes needed in the `RunE` functions -- they already use `cmd.Flags().GetBool("json")` which works for both local and persistent flags.

| File | Line | Remove |
|------|------|--------|
| `cmd/find/find.go` | 51 | Delete `findCmd.Flags().Bool("json", false, "Output as JSON")` |
| `cmd/scry/scry.go` | 47 | Delete `scryCmd.Flags().Bool("json", false, "Output as JSON")` |
| `cmd/config/get.go` | 43 | Delete `getCmd.Flags().Bool("json", false, "output in JSON format")` |

**Caveat**: Removing local `--json` will change help output. Currently `find --help` shows `--json` under local flags. After removal, it would only appear under inherited flags (or not at all in help, depending on Cobra version). Verify help output after removal.

### LOW (Inconsistency)

**Help text inconsistency for `--json`**: Three different descriptions across the codebase:
- Root: "machine-readable JSON output"
- find: "Output as JSON"
- config get: "output in JSON format"

Resolved by removing the duplicates (MEDIUM fix above).

---

## 7. Flag Access Pattern Summary Table

| Command | `--json` | `--quiet` | Method |
|---------|----------|-----------|--------|
| `add item` | `cmd.Flags().GetBool` (persistent) | `cli.IsQuietMode` | Correct |
| `add location` | `cmd.Flags().GetBool` (persistent) | `cli.IsQuietMode` | Correct |
| `config init` | `cmd.Flags().GetBool` (persistent) | `cmd.Flags().GetBool` | **quiet BROKEN** |
| `config get` | `cmd.Flags().GetBool` (local shadow) | `cmd.Flags().GetBool` | **quiet BROKEN, json duplicated** |
| `config set` | `cmd.Flags().GetBool` (persistent) | `cmd.Flags().GetBool` | **quiet BROKEN** |
| `config edit` | `cmd.Flags().GetBool` (persistent) | `cmd.Flags().GetBool` | **quiet BROKEN** |
| `config check` | `cmd.Flags().GetBool` (persistent) | `cmd.Flags().GetBool` | **quiet BROKEN** |
| `config path` | `cmd.Flags().GetBool` (persistent) | `cmd.Flags().GetBool` | **quiet BROKEN** |
| `find` | `cmd.Flags().GetBool` (local shadow) | N/A | **json duplicated** |
| `history` | `cmd.Flags().GetBool` (persistent) | N/A | Correct |
| `loan` | `cmd.Flags().GetBool` (persistent) | `cli.IsQuietMode` | Correct |
| `lost` | `cmd.Flags().GetBool` (persistent) | `cli.IsQuietMode` | Correct |
| `move` | `cmd.Flags().GetBool` (persistent) | `cli.IsQuietMode` | Correct |
| `scry` | `cmd.Flags().GetBool` (local shadow) | N/A | **json duplicated** |

---

## 8. Recommended Priority

1. **Fix `--quiet` bug in all config subcommands** (6 files, replace `GetBool("quiet")` with `cli.IsQuietMode(cmd)`)
2. **Remove duplicate `--json` declarations** (3 files, delete local flag registration)
3. **Verify help output** after removing local `--json` flags
