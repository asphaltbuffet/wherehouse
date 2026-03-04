# Task 1b Changes

## Files Created

### `/home/grue/dev/wherehouse/internal/cli/found.go`
- New file implementing `FoundItem` function
- Defines unexported `foundDB` interface (minimal: `GetItem`, `GetLocation`, `ValidateFromLocation`, `AppendEvent`)
- Defines `FoundItemResult` struct mirroring `cmd/found.Result` fields (without JSON tags, for library use)
- Extracts all domain logic from `cmd/found/found.go:foundItem` (~115 lines)
- Handles: current state warnings, home location fallback, item.found event, optional item.moved rehome event
- `*database.Database` satisfies `foundDB` implicitly

## Files Modified

### `/home/grue/dev/wherehouse/cmd/found/found.go`
- Replaced `foundItem` body (~115 lines of domain logic) with thin wrapper (~20 lines)
- Wrapper calls `cli.FoundItem(ctx, db, itemID, foundLocationID, returnToHome, actorUserID, note)`
- Maps `cli.FoundItemResult` fields back to local `Result` type (for JSON tags and output formatting)
- No imports added or removed (both `database` and `cli` were already imported)
- All other functions (`runFoundItem`, `validateNotSystemLocation`, `formatSuccessMessage`) unchanged

## Verification

- `go build ./internal/cli/...` — clean
- `go build ./cmd/found/...` — clean
- `go test ./internal/cli/... ./cmd/found/...` — all pass
- `golangci-lint run ./internal/cli/ ./cmd/found/` — 0 issues in changed files (1 pre-existing issue in `output.go` line 137, unrelated to this task)
