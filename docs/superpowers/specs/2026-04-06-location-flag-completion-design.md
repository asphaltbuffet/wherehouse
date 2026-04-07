# Design: Shell Completion for `--in` / `--to` Location Flags

**Date:** 2026-04-06
**Status:** Approved

## Summary

Add shell tab-completion for location-accepting flags across four commands. When the user presses `<Tab>` after `--in` or `--to`, the shell presents all non-system locations as full canonical paths (e.g. `garage/shelf-a`).

## Scope

Commands receiving completion:

| Command | Flag |
|---|---|
| `add item` | `--in` |
| `add location` | `--in` |
| `found` | `--in` |
| `move` | `--to` |

System locations (`IsSystem == true`: Missing, Borrowed, Loaned, Removed) are excluded from all completion lists.

## Architecture

### Shared helper: `internal/cli/completions.go`

Single exported function:

```go
func LocationCompletions(ctx context.Context) ([]string, cobra.ShellCompDirective)
```

Behavior:
1. Opens database via `cli.OpenDatabase(ctx)`
2. Calls `db.GetAllLocations(ctx)`
3. Filters out entries where `IsSystem == true`
4. Returns `loc.FullPathCanonical` for each remaining location
5. Returns `cobra.ShellCompDirectiveNoFileComp` on success
6. Returns `nil, cobra.ShellCompDirectiveError` on any error (silent — completion offers nothing)

No new DB interfaces. No changes to `foundDB`, `moveDB`, or any per-command interface. The completion function manages its own DB connection, matching the pattern used by `NewDefaultXxxCmd`.

### Wiring in each command

`cmd.RegisterFlagCompletionFunc` is called immediately after each flag definition:

```go
_ = cmd.RegisterFlagCompletionFunc("in", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
    return cli.LocationCompletions(cmd.Context())
})
```

- `cmd/found/found.go` — in `registerFoundFlags`, for `"in"`
- `cmd/add/item.go` — in `GetItemCmd`, for `"in"`
- `cmd/add/location.go` — in `GetLocationCmd`, for `"in"`
- `cmd/move/move.go` — in `registerMoveFlags`, for `"to"`

## Data

Completion values are `FullPathCanonical` (e.g. `garage`, `garage/shelf-a`, `toolbox`). These match what users type at the command line. Display names are not used for completions.

## Error Handling

Completion errors are silent (`cobra.ShellCompDirectiveError`). A DB open failure or query failure results in an empty completion list — the command still runs normally; the user just gets no suggestions. This is consistent with how cobra handles completion failures.

## Testing

One test file: `internal/cli/completions_test.go`

- Uses a real in-memory test DB (consistent with other `internal/cli` tests)
- Verifies system locations are excluded from results
- Verifies `FullPathCanonical` values are returned
- Verifies error path returns `cobra.ShellCompDirectiveError`

## Out of Scope

- fzf interactive picker (Option B/C from brainstorm — not requested)
- Completion for item selectors / `--note` / `--return` flags
- Shell completion script generation command (cobra provides `wherehouse completion <shell>` automatically)
