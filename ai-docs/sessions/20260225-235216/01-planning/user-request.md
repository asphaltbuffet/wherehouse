# User Request: Configuration Refactoring

## Summary
Refactor the wherehouse configuration system to use viper effectively.

## Requirements

1. **No direct TOML editing**: Remove all direct go-toml file manipulation. Configuration should only be read/written through the viper singleton.

2. **Viper wrapper**: `internal/config/` should be a proper wrapper around spf13/viper that enforces business rules (validation, defaults, etc.)

3. **Preserve configuration hierarchy**: The existing config hierarchy (env vars, config file, defaults) should remain the same.

4. **`get`/`set` subcommands via viper**: The `cmd/config/get.go` and `cmd/config/set.go` commands should interact with the viper singleton rather than editing TOML fields directly.

5. **Consolidate business logic**: The business logic currently scattered in `cmd/config/` files should be moved into `internal/config/`.

6. **Golden file for `config init`**: Strong preference to create a golden file for `config init` output and use it for read validation (ensures config file format stays consistent).

## Context
- Current config files: `cmd/config/` (check.go, config.go, edit.go, get.go, helpers.go, init.go, path.go, set.go, unset.go)
- Internal config: `internal/config/` (config.go, database.go, defaults.go, loader.go, log.go, validation.go)
- Tech stack: spf13/viper, spf13/cobra, go-toml
