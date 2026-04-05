# Task A Notes

## Decisions

- Interface placed immediately before `ResolveLocation` (the first function that uses it), which is the idiomatic Go location for interface definitions near their first use point.
- The interface uses `context.Context` in method signatures matching the actual `*database.Database` method signatures exactly, ensuring implicit satisfaction without any adapter layer.
- No import changes were required; the `database` package import was already present and is still needed for `database.Location`, `database.Item`, `database.CanonicalizeString`, and error sentinel values.

## Verification

- `go build ./internal/cli/...` — clean
- `go build ./...` — clean (no regressions in callers)
- `go test ./internal/cli/...` — all tests pass (existing tests pass `*database.Database` which satisfies the interface)
