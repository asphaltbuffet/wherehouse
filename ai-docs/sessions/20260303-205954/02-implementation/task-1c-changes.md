# Task 1c: Changes

## Files Created

### `/home/grue/dev/wherehouse/internal/cli/loan.go`
New file. Extracts loan business logic from `cmd/loan/item.go` into `cli.LoanItem`.
- Defines `LoanItemResult` struct (outcome of loaning an item)
- Defines `LoanItemOptions` struct (Borrower, Note)
- Defines `loanDB` interface (unexported, embeds `LocationItemQuerier`, adds `GetItemLoanedInfo`, `ValidateItemLoaned`, `AppendEvent`)
- `LoanItem(ctx, db, itemSelector, actorUserID, opts)` handles: selector resolution, item/location lookup, re-loan detection, validation, event creation, result assembly

### `/home/grue/dev/wherehouse/internal/cli/lost.go`
New file. Extracts lost/missing business logic from `cmd/lost/item.go` into `cli.LostItem`.
- Defines `LostItemResult` struct (outcome of marking item as lost)
- Defines `LostItemOptions` struct (Note)
- Defines `lostDB` interface (unexported, embeds `LocationItemQuerier`, adds `ValidateFromLocation`, `AppendEvent`)
- `LostItem(ctx, db, itemSelector, actorUserID, opts)` handles: selector resolution, item/location lookup, already-missing guard, projection validation, event creation, result assembly

## Files Modified

### `/home/grue/dev/wherehouse/cmd/loan/item.go`
Replaced `runLoanItem` handler with thin wrapper that calls `cli.LoanItem` per selector.
- Removed: `validateItemForLoan`, `loanItem` functions (business logic extracted to `internal/cli`)
- Kept: `Result` struct (used for JSON output), `runLoanItem` (now thin wrapper)
- Loop now calls `cli.LoanItem` for each selector; output logic unchanged

### `/home/grue/dev/wherehouse/cmd/lost/item.go`
Replaced `markItemLost` body with delegation to `cli.LostItem`.
- `runLostItem` is a thin wrapper calling `markItemLost`
- `markItemLost` is a thin wrapper around `cli.LostItem` (preserved for existing test compatibility)
- All domain logic removed from package; selector resolution now happens inside `cli.LostItem`

### `/home/grue/dev/wherehouse/cmd/loan/helpers.go`
Removed unused `resolveItemSelector` helper (selector resolution now handled inside `cli.LoanItem`).

### `/home/grue/dev/wherehouse/cmd/lost/item_test.go`
Updated `TestMarkItemLost_ItemNotFound_Error` assertion from "item not found" to "not found" to match new error message format from `ResolveItemSelector` ("item with ID ... not found").

### `/home/grue/dev/wherehouse/internal/cli/output.go`
Fixed pre-existing godoclint issue: `io.Writer` → `[io.Writer]` in `Writer()` doc comment.

## Pre-existing Issues Not Introduced by Task 1c

- `cmd/add/helpers.go`: unused `openDatabase` and `resolveLocation` (introduced by task 1a; to be deleted in step 2)
- `cmd/migrate`: test failure in `TestGetDatabaseCmd_ShortHelp` (introduced by task 5)
