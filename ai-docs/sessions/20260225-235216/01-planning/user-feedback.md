# User Feedback on Plan

## Change 1: Remove the golden file approach completely

The user wants to remove the golden file approach for `config init` testing.
Replace with programmatic validation via round-trip testing.

## Change 2: Viper usage approach

The user wants:
1. **Viper-backed config struct**: Load config into a `*Config` struct once via viper, use the struct everywhere in app code.

2. **Bind viper to persistent/config flags**: Most persistent flags (e.g., `--format`, `--quiet`, `--user`) should be bound to viper so that:
   - Config file values serve as defaults
   - Flag values override config values
   - EXCLUDE transient flags that are unique per-invocation (e.g., `--to`, `--from`, `--id`)

3. **Use viper's native write capability**: Use `viper.WriteConfig()` and `viper.WriteConfigAs(path)` for writing config files. Eliminate custom TOML serialization.

## Change 3: Remove `config unset` command entirely

Since viper has no key-delete API, the `Unset` implementation is awkward.
The user has decided to REMOVE the `config unset` command entirely.

- Delete `cmd/config/unset.go`
- Delete `cmd/config/unset_test.go`
- Remove `Unset` function from any writer.go plan
- Remove the `unset` subcommand from the config command registration
- No replacement needed - users who want to "unset" a value can use `config set` with the default value
