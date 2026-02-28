# Task B Changes

## Files Created

### `/home/grue/dev/wherehouse/cmd/move/mover.go` (new)
- Defines the `moveDB` unexported interface with all 8 methods required by the move command
- `AppendEvent` payload type is `any` (matches `*database.Database` actual signature, not `map[string]any`)
- Includes `//go:generate mockery --name=moveDB` directive

## Files Modified

### `/home/grue/dev/wherehouse/cmd/move/helpers.go`
- Changed `resolveLocation` signature: `*database.Database` → `moveDB`
- Changed `resolveItemSelector` signature: `*database.Database` → `moveDB`
- Removed `database` import (no longer needed after type change)
- `cli.ResolveLocation` and `cli.ResolveItemSelector` already accept `LocationItemQuerier` (Task A), and `moveDB` satisfies that interface

### `/home/grue/dev/wherehouse/cmd/move/item.go`
- Removed `runMoveItem(cmd, args)` function
- Added `runMoveItemCore(cmd, args, db moveDB)` — same logic, DB passed in; no `openDatabase` call; no `defer db.Close()` (caller owns lifecycle)
- Changed `moveItem` signature: `*database.Database` → `moveDB`
- Changed `validateDestinationNotSystem` signature: `*database.Database` → `moveDB`
- Removed `database` import (no longer needed)

### `/home/grue/dev/wherehouse/cmd/move/move.go`
- Removed `var moveCmd *cobra.Command` singleton
- Removed `GetMoveCmd()` function
- Added `const moveLongDescription` for shared Long description
- Added `NewMoveCmd(db moveDB) *cobra.Command` — injects pre-opened DB; RunE defers Close then calls `runMoveItemCore`
- Added `NewDefaultMoveCmd() *cobra.Command` — opens DB from context in RunE; defers Close then calls `runMoveItemCore`
- Added `registerMoveFlags(cmd *cobra.Command)` — extracted flag registration, called by both constructors
- Both constructors use identical Use/Short/Long/Args fields

### `/home/grue/dev/wherehouse/cmd/root.go`
- Line 72: `move.GetMoveCmd()` → `move.NewDefaultMoveCmd()`

### `/home/grue/dev/wherehouse/cmd/move/item_test.go`
- Line 391: `GetMoveCmd()` → `NewDefaultMoveCmd()` in `TestGetMoveCmd_Structure`
