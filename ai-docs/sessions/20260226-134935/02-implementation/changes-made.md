# Changes Made

## Task A - DB: GetRootLocations
- `internal/database/location.go` - Added `GetRootLocations` function
- `internal/database/location_test.go` - Added 5 tests for GetRootLocations

## Task B - CLI: list command
- `cmd/list/doc.go` - Package doc
- `cmd/list/list.go` - Cobra command + buildNodes/buildLocationNodeFlat/buildLocationNodeRecursive
- `cmd/list/output.go` - LocationNode, JSON types, renderTree (via xlab/treeprint), toJSON
- `cmd/list/helpers.go` - openDatabase, resolveLocation wrappers
- `cmd/list/list_test.go` - 23 tests
- `cmd/root.go` - Registered list command

## Misc Fix
- `internal/database/search.go` - Removed unused `resultTypeLocation` const; used `resultTypeItem` consistently

## Dependencies Added
- `github.com/xlab/treeprint` - tree rendering
