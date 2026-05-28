# CLI error rendering uses OutputWriter throughout, including at the root Execute level

All CLI error output — including errors returned by cobra before any subcommand config is loaded — is rendered via `internal/cli.OutputWriter.Error()` (red bold text to stderr via `internal/styles`). At the root `Execute()` level, the writer is constructed with `jsonMode: false, quietMode: false` hardcoded, because cobra-level errors (unknown flag, unknown command) fire before flag parsing completes and JSON mode is not meaningful for them.

The alternative was to call `styles.DefaultStyles().Error().Render(...)` inline, which is simpler at the call site. We chose `OutputWriter` instead so that error rendering has exactly one implementation in the codebase — if the format changes (e.g. adding a prefix, switching to stderr-only JSON), it changes in one place.
