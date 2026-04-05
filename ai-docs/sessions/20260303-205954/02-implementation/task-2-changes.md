# Task 2 Implementation: DI Standardization Across All Commands

## Summary

Applied the move-pattern DI (db.go + New*Cmd constructor + NewDefault*Cmd) to all remaining cmd packages.

---

## Files Created

| File | Description |
|------|-------------|
| `/home/grue/dev/wherehouse/cmd/list/db.go` | `listDB` interface with all methods used by list command |
| `/home/grue/dev/wherehouse/cmd/find/db.go` | `findDB` interface with SearchByName + GetItemLoanedInfo |
| `/home/grue/dev/wherehouse/cmd/scry/db.go` | `scryDB` interface with item/location/scry methods |
| `/home/grue/dev/wherehouse/cmd/history/db.go` | `historyDB` interface with item/location/event methods |

## Files Modified

| File | Changes |
|------|---------|
| `/home/grue/dev/wherehouse/cmd/add/add.go` | Replaced `GetAddCmd()` singleton with `NewAddCmd()` + `NewDefaultAddCmd()` + deprecated alias |
| `/home/grue/dev/wherehouse/cmd/list/list.go` | Removed `testOpenDatabase`/`testMustGetConfig` hooks; replaced with `NewListCmd(db listDB)` + `NewDefaultListCmd()`; changed all internal functions to accept `listDB` instead of `*database.Database` |
| `/home/grue/dev/wherehouse/cmd/list/list_test.go` | Updated `TestRunList_*` integration tests to use `NewListCmd(f.db)` + `config.ConfigKey` context; removed package-level hook usage |
| `/home/grue/dev/wherehouse/cmd/find/find.go` | Replaced `GetFindCmd()` singleton with `NewFindCmd(db findDB)` + `NewDefaultFindCmd()`; changed `prefetchLoanedInfo` to accept `findDB`; removed unused `openDatabase` wrapper; removed unused `runFind` wrapper |
| `/home/grue/dev/wherehouse/cmd/find/find_test.go` | Updated to test `NewDefaultFindCmd()` instead of singleton |
| `/home/grue/dev/wherehouse/cmd/scry/scry.go` | Replaced `GetScryCmd()` singleton with `NewScryCmd(db scryDB)` + `NewDefaultScryCmd()`; changed `validateItemIsMissing` to accept `scryDB` |
| `/home/grue/dev/wherehouse/cmd/history/history.go` | Replaced `GetHistoryCmd()` singleton with `NewHistoryCmd(db historyDB)` + `NewDefaultHistoryCmd()`; inlined `resolveItemSelector` (was in resolver.go); changed internal calls to use `historyDB` |
| `/home/grue/dev/wherehouse/cmd/history/output.go` | Changed all functions accepting `*database.Database` to accept `historyDB` interface |
| `/home/grue/dev/wherehouse/cmd/history/history_test.go` | Updated to test `NewDefaultHistoryCmd()` instead of singleton |
| `/home/grue/dev/wherehouse/cmd/root.go` | Wired all commands to `NewDefault*Cmd()` constructors: add, find, found, history, list, loan, lost, scry |
| `/home/grue/dev/wherehouse/cmd/add/add_test.go` | Updated singleton test to non-singleton pattern |
| `/home/grue/dev/wherehouse/cmd/found/found_test.go` | Updated singleton test to non-singleton pattern |
| `/home/grue/dev/wherehouse/cmd/lost/item.go` | Added `resolveItemSelector(ctx, lostDB, selector)` function (moved from deleted helpers.go) |

### Deprecated comment formatting (separate paragraph) — also fixed in previously-modified files:
- `/home/grue/dev/wherehouse/cmd/found/found.go`
- `/home/grue/dev/wherehouse/cmd/loan/loan.go`
- `/home/grue/dev/wherehouse/cmd/lost/lost.go`

## Files Deleted

| File | Reason |
|------|--------|
| `/home/grue/dev/wherehouse/cmd/add/helpers.go` | Dead code — `openDatabase`/`resolveLocation` wrappers unused by any code in the package |
| `/home/grue/dev/wherehouse/cmd/list/helpers.go` | Dead code — `openDatabase`/`resolveLocation` wrappers replaced by interface injection |
| `/home/grue/dev/wherehouse/cmd/loan/helpers.go` | Dead code — `openDatabase` wrapper unused after DI adoption |
| `/home/grue/dev/wherehouse/cmd/lost/helpers.go` | Dead code — `openDatabase` wrapper unused; `resolveItemSelector` moved to item.go with lostDB type |
| `/home/grue/dev/wherehouse/cmd/history/resolver.go` | Thin wrapper inlined into history.go as `resolveItemSelector` function |

## Key Design Decisions

1. **`cmd/add/db.go` not created**: The `add` package's subcommands delegate entirely to `internal/cli.AddItems` and `internal/cli.AddLocations` which open their own DB internally. The parent `add` command has no direct DB access.

2. **`listDB` includes `GetItem`/`GetItemsByCanonicalName`**: Required because `cli.ResolveLocation` accepts `LocationItemQuerier` which includes these methods.

3. **`historyDB` uses actual `GetEventsByEntity` signature**: The plan had `opts ...any` but the real method has no variadic args parameter.

4. **`resolveItemSelector` in `cmd/lost/item.go`**: Preserved for the `helpers_test.go` tests that test resolver behavior. Changed to accept `lostDB` — `*database.Database` satisfies this.

5. **Pre-existing test failure**: `cmd/migrate/TestGetDatabaseCmd_ShortHelp` was already failing before this task (case-sensitive check for "migrate" in a string that starts with capital "Migrate"). Not modified.

## Test Results

- All modified packages: PASS
- Pre-existing failure: `cmd/migrate/TestGetDatabaseCmd_ShortHelp` (unrelated to this task)
- Linting: CLEAN (0 issues)
