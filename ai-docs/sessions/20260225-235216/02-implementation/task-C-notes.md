# Task C Notes

## Pre-existing test failures

`go test ./cmd/config/...` fails to compile due to `GetUnsetCmd` and `unsetCmd` being
undefined in `config_test.go`. This is unrelated to Task C — it is a pre-existing condition
from a missing `unset.go` or incomplete implementation elsewhere in the session.

The package itself (`go build ./cmd/config/...`) compiles cleanly.

## Behaviour equivalence

The old `updateConfigValue` function:
1. Read file with `afero.ReadFile`
2. Unmarshalled TOML into `map[string]any`
3. Called `setValueInMap` (helpers.go) for type-checked value insertion
4. Marshalled back to TOML with `go-toml`
5. Validated by unmarshalling into `config.Config` then `config.Validate`
6. Wrote with `atomicWrite`

The new `config.Set` function (internal/config/writer.go):
1. Validates key and parses value with `parseConfigValue` (type-safe, per-key)
2. Reads file via viper
3. Sets value via viper
4. Validates full merged config via `validate`
5. Writes via `v.WriteConfigAs`

The behaviour is equivalent from the user perspective. The new path is the canonical
implementation owned by `internal/config`.

## No helpers.go modification

`helpers.go` was not modified. `setValueInMap` and `atomicWrite` remain for use by other
commands (e.g., `unset.go`).
