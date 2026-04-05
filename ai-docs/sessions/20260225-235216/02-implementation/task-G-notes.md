# Task G Notes

## Implementation Decisions

### Changed-check pattern
Using `cmd.Flags().Changed("flagname")` before reading flag values ensures that flag
default values (empty string for `--db`/`--as`, false for `--json`, 0 for `--quiet`)
never silently overwrite values that were intentionally set in the config file.

### Non-empty guard for string flags
For `--db` and `--as`, an additional `val != ""` guard is applied even when `Changed`
is true. This is belt-and-suspenders: cobra won't normally mark a flag Changed if the
user passed an empty string explicitly, but the guard avoids accidentally clearing a
path if called programmatically.

### quiet is a count flag, treated as bool override
`--quiet` is defined with `CountP`. `Changed` becomes true when the flag appears at
least once. We set `cfg.Output.Quiet = true` regardless of the count value because the
Config struct uses a bool field. Callers that need the count level (e.g., `-qq`) read
the count directly from the flag rather than from `cfg`.

### Placement in initConfig
`bindFlagsToConfig` is called after `loadConfigOrDefaults` and before `globalConfig = cfg`,
so flags take highest priority: defaults < config file < flags.

### No structural changes
No imports were added or removed. No existing function signatures changed. The function
is purely additive.

## Verification

`go build ./cmd/...` completed with no errors after the change.
