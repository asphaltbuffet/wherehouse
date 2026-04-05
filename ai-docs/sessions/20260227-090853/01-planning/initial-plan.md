# Architecture Plan: `wherehouse found` Command

**Date**: 2026-02-27
**Session**: 20260227-090853
**Status**: Draft

---

## 1. Executive Summary

The `found` command answers the scenario: "I know I found this item - let me record where it is now." It uses the existing `item.found` event type (already implemented in `internal/database/itemEventHandler.go` as `handleItemFound`). The `--return` flag fires a second `item.moved` (rehome) event to move the item from the found location back to its home.

This command does NOT introduce a new event type. It reuses `item.found` and, when `--return` is supplied, chains it with `item.moved`.

---

## 2. Event Analysis

### 2.1 Existing `item.found` Event (already in codebase)

The event handler `handleItemFound` already exists in `internal/database/itemEventHandler.go`:

```go
func (d *Database) handleItemFound(ctx context.Context, tx *sql.Tx, event *Event) error {
    // payload: item_id, found_location_id, home_location_id
    // SET location_id = found_location_id
    // SET in_temporary_use = 1
    // SET temp_origin_location_id = home_location_id
}
```

The event handler:
- Sets `location_id = found_location_id`
- Sets `in_temporary_use = true`
- Sets `temp_origin_location_id = home_location_id` (user-specified or inferred)
- Does NOT clear project association

The router in `eventHandler.go` maps `"item.found"` to this handler. The event type name in the dispatcher is `"item.found"` (not `"item.marked_found"` as in the knowledge docs - implementation uses the shorter form).

### 2.2 `--return` Chains a Second Event

When `--return` is specified, after `item.found` succeeds, the command fires `item.moved` with:
- `from_location_id = found_location_id`
- `to_location_id = home_location_id`
- `move_type = "rehome"` (permanent - item is back home)
- `project_action = "clear"` (default)

This produces two events in the log, which is correct event-sourcing behavior. Both are applied atomically... but because `AppendEvent` uses its own transaction per call, they are two separate transactions. This is intentional and consistent with existing multi-event operations (like the move command doesn't batch either).

### 2.3 When `--return` fires: home_location_id must be known

The `item.found` event requires `home_location_id`. The user does NOT pass this - it comes from `item.TempOriginLocationID` in the projection when the item is already in temporary use, or there's no established "home" concept when the item is in a normal location.

**Key insight**: `home_location_id` is a concept for the `item.found` event specifically. When the item is at a regular (non-Missing, non-Borrowed) location, it is considered "at home." The `found` command must ask: what is the item's home?

**Resolution**: The `home_location_id` in the `item.found` event payload is the location the item should return to. Since there is no explicit `home_location` field in the item schema, the command must derive it:
- If `item.InTemporaryUse == true`: home is `item.TempOriginLocationID`
- If `item.InTemporaryUse == false` and item is at Missing: previous location is unknown without event log scan - use `--home` flag or fail with a helpful error
- If item is at a normal location (not system): current location IS home, and the found location IS current, this is a no-op situation

See Section 5 for the gap analysis on this.

---

## 3. Command Architecture

### 3.1 Package Structure

```
cmd/found/
├── found.go        # GetFoundCmd(), cobra command definition, flags
├── item.go         # runFoundItem(), foundItem(), returnItem()
├── helpers.go      # openDatabase(), resolveLocation(), resolveItemSelector() (thin wrappers to cli/)
└── doc.go          # package doc comment
```

This mirrors the `cmd/move/` package structure exactly.

### 3.2 Command Signature

```
wherehouse found <item-selector>... --in <location> [--return] [--home <location>] [--note <text>]
```

**Flags**:

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--in` / `-i` | string | yes | Location where item was found |
| `--return` / `-r` | bool | no | Also move item back to its home location |
| `--home` | string | no | Override home location for `--return` (used when home cannot be inferred) |
| `--note` / `-n` | string | no | Optional note for event(s) |

**Args**: `<item-selector>...` (variadic, minimum 1)

### 3.3 Flag Constraints

- `--home` is only meaningful with `--return`; warn if `--home` supplied without `--return`
- `--home` and `--return` are companion flags, not mutually exclusive
- If `--return` is specified but home cannot be inferred and `--home` is not given, fail with clear error

### 3.4 Selector Support

Item selectors follow the same three-tier resolution as `move`:
1. UUID (exact ID)
2. `LOCATION:ITEM` (both canonical names)
3. Canonical name (must resolve to exactly 1 item)

The `--in` location resolves via `cli.ResolveLocation()`.

---

## 4. Implementation Walkthrough

### 4.1 `found.go` - Command Definition

```go
func GetFoundCmd() *cobra.Command {
    // cobra command with Use, Short, Long, Args: cobra.MinimumNArgs(1)
    // Flags: --in (required), --return, --home, --note
    // MarkFlagRequired("in")
}
```

### 4.2 `item.go` - Core Logic

```go
func runFoundItem(cmd *cobra.Command, args []string) error {
    // 1. Parse flags: foundLocation, returnToHome, homeLocation, note
    // 2. openDatabase
    // 3. Get actorUserID
    // 4. Resolve --in location ID (call resolveLocation)
    // 5. Validate --in is NOT a system location (cannot "find" at Missing or Borrowed)
    // 6. If --home specified, resolve home location ID
    // 7. Set up OutputWriter
    // 8. For each item selector (fail-fast):
    //    a. resolveItemSelector
    //    b. foundItem(...)
    //    c. Print success
    // 9. Output JSON if --json
}

func foundItem(ctx, db, itemID, foundLocationID, homeLocationID, actorUserID, note string, returnToHome bool) (*Result, error) {
    // 1. db.GetItem to get current state
    // 2. db.GetLocation for current location (for system location check)
    // 3. If item is already at foundLocationID AND !returnToHome: return error/warning "item already here"
    // 4. Infer homeLocationID if not provided:
    //    a. If item.InTemporaryUse: homeLocationID = *item.TempOriginLocationID
    //    b. Else if item is at Missing: fail, require --home
    //    c. Else: homeLocationID = item.LocationID (current is home)
    // 5. Build item.found payload:
    //    { item_id, found_location_id, home_location_id }
    // 6. db.AppendEvent("item.found", actorUserID, payload, note)
    // 7. If returnToHome:
    //    a. Validate foundLocationID != homeLocationID (no-op check)
    //    b. Build item.moved payload:
    //       { item_id, from_location_id: foundLocationID, to_location_id: homeLocationID,
    //         move_type: "rehome", project_action: "clear" }
    //    c. db.AppendEvent("item.moved", actorUserID, payload, note)
    // 8. Return Result
}
```

### 4.3 Result Type

```go
type Result struct {
    ItemID        string  `json:"item_id"`
    DisplayName   string  `json:"display_name"`
    FoundAt       string  `json:"found_at"`
    HomeLocation  string  `json:"home_location"`
    Returned      bool    `json:"returned"`
    FoundEventID  int64   `json:"found_event_id"`
    ReturnEventID *int64  `json:"return_event_id,omitempty"`
}
```

### 4.4 Human Output

Without `--return`:
```
Found "10mm socket" at Garage (home: Tote F)
```

With `--return`:
```
Found "10mm socket" at Garage, returned to Tote F
```

With `--json`:
```json
{
  "found": [
    {
      "item_id": "...",
      "display_name": "10mm socket",
      "found_at": "Garage",
      "home_location": "Tote F",
      "returned": true,
      "found_event_id": 42,
      "return_event_id": 43
    }
  ]
}
```

---

## 5. Validation Rules

### 5.1 Pre-event Validation

| Check | Error condition | Behavior |
|-------|----------------|---------|
| Item exists | Not in projection | Fail with ErrItemNotFound |
| Found location exists | Not in projection | Fail with ErrLocationNotFound |
| Found location is system | is_system=true | Fail: "cannot find item at system location" |
| Home location exists (if --home) | Not in projection | Fail with ErrLocationNotFound |
| Item already at found location (no --return) | location_id == foundLocationID | Warn or error (see gap) |

### 5.2 Home Location Inference Logic

```
IF --home provided:
    use resolved home location ID

ELSE IF item.InTemporaryUse == true:
    home = *item.TempOriginLocationID

ELSE IF item is at Missing location (is_system=true, name="missing"):
    FAIL: "cannot infer home for item at Missing - use --home to specify"

ELSE:
    home = item.LocationID  (current non-system location = home)
```

### 5.3 No-op Detection for `--return`

If after resolution `foundLocationID == homeLocationID`, the `--return` flag would be a no-op (item would move to itself). Options:
- Fire `item.found` but skip `item.moved` (with a warning)
- Fail with error

Recommendation: Fire `item.found` (which records the discovery), skip `item.moved` because `from_location_id == to_location_id` is rejected by validation, and print a note: "item already at home location - skipping return move".

### 5.4 System Location Restrictions

- Cannot "find" an item AT a system location (Missing, Borrowed)
- The item can currently be at Missing (that's the normal use case for `found`)
- `item.found` does NOT require the item to currently be at Missing (warn if not, but allow) - consistent with knowledge docs

---

## 6. Integration with Existing Database Layer

### 6.1 No New Database Methods Required

All required database operations already exist:
- `db.GetItem(ctx, itemID)` - get current item state
- `db.GetLocation(ctx, locationID)` - get location details (for system check)
- `db.AppendEvent(ctx, "item.found", ...)` - fire found event
- `db.AppendEvent(ctx, "item.moved", ...)` - fire return move event
- `cli.ResolveLocation(ctx, db, input)` - resolve location
- `cli.ResolveItemSelector(ctx, db, selector, "wherehouse found")` - resolve item

### 6.2 The `item.found` Handler Already Works

`handleItemFound` in `itemEventHandler.go` correctly:
- Sets `location_id = found_location_id`
- Sets `in_temporary_use = 1`
- Sets `temp_origin_location_id = home_location_id`

No changes needed to the database layer for the basic `found` operation.

### 6.3 The `item.moved` Handler Handles `--return`

`handleItemMoved` with `move_type = "rehome"` correctly:
- Sets `location_id = to_location_id` (homeLocationID)
- Sets `in_temporary_use = false`
- Sets `temp_origin_location_id = NULL`
- Applies `project_action = "clear"`

This is exactly the right semantic for returning an item home.

---

## 7. Registration in Root Command

The `found` command must be registered in the root cobra command (likely `cmd/root.go`):

```go
rootCmd.AddCommand(found.GetFoundCmd())
```

---

## 8. Differences from `move`

| Aspect | `move` | `found` |
|--------|--------|---------|
| Event type | `item.moved` | `item.found` (+ optionally `item.moved`) |
| Home location changed? | Yes (rehome) or no (temp) | Never - found preserves home |
| Source location restriction | Cannot FROM system | Can FROM system (item was Missing) |
| Destination restriction | Cannot TO system | Cannot TO system |
| `--return` flag | No | Yes (chains second event) |
| Home inference | N/A | Yes (from TempOriginLocationID or current) |
| `in_temporary_use` semantics | Set/clear based on move_type | Always set to true after found |

---

## 9. Edge Cases

### 9.1 Item Currently at Missing

Normal use case. The `item.found` event:
- Moves item out of Missing to `found_location_id`
- Sets `in_temporary_use = true`
- Sets `temp_origin_location_id = home_location_id` (inferred or specified)

Home inference: since the item is at Missing (system location), we cannot use current location as home. We need:
- Either `--home` flag
- Or the item was previously in temporary use (check `TempOriginLocationID`)

Note: When `item.marked_missing` fires, it preserves `in_temporary_use` and `temp_origin_location_id`. So if the item was in temporary use before going missing, we can still infer home from `TempOriginLocationID` even while it's at Missing.

### 9.2 Item Currently at Normal Location (Not Missing)

Unusual case. User ran `found` on an item that wasn't marked missing. The knowledge docs say: warn if current location is not Missing, but allow. Proceed normally.

### 9.3 Item Currently at Borrowed

Should we allow `found` on a borrowed item? The `item.found` handler doesn't check current location. However, this is semantically odd - if an item is borrowed, it's not "lost." This could be a warning case.

### 9.4 Multiple Items, Partial Failure

Fail-fast behavior like `move`: if item 2 fails, items 0 and 1 are already committed. This is consistent with the existing `move` command design.

### 9.5 `found_location_id == current_location_id`

The item is already at the location it was "found" at. The `item.found` event would be a no-op state-wise (location unchanged) but `in_temporary_use` and `home_location_id` would be set. This is valid - the user may want to set the home tracking even if location doesn't change.

---

## 10. File-by-File Implementation Guide

### `cmd/found/found.go`

```go
package found

import "github.com/spf13/cobra"

var foundCmd *cobra.Command

func GetFoundCmd() *cobra.Command {
    if foundCmd != nil {
        return foundCmd
    }

    foundCmd = &cobra.Command{
        Use:   "found <item-selector>... --in <location>",
        Short: "Record that a previously lost or missing item has been found",
        Long: `Record that one or more items have been found at a specific location.

The item's home location is NOT changed - it remains where it was expected.
Use --return to also move the item back to its home location.

Selector types:
  - UUID: 550e8400-e29b-41d4-a716-446655440001 (exact ID)
  - LOCATION:ITEM: garage:socket (both canonical names)
  - Canonical name: "10mm socket" (must match exactly 1 item)

Examples:
  wherehouse found "10mm socket" --in garage
  wherehouse found "10mm socket" --in garage --return
  wherehouse found "10mm socket" --in garage --return --home toolbox
  wherehouse found garage:screwdriver --in shed --note "was behind workbench"`,
        Args: cobra.MinimumNArgs(1), //nolint:mnd // 1 is the exact minimum arg count
        RunE: runFoundItem,
    }

    foundCmd.Flags().StringP("in", "i", "", "location where item was found (required)")
    _ = foundCmd.MarkFlagRequired("in")

    foundCmd.Flags().BoolP("return", "r", false, "also return item to its home location")
    foundCmd.Flags().String("home", "", "override home location for --return (inferred if not set)")
    foundCmd.Flags().StringP("note", "n", "", "optional note for event")

    return foundCmd
}
```

### `cmd/found/item.go`

Core logic following the `move/item.go` pattern. Key function `foundItem` handles:
1. Get item state
2. Infer home location (see 4.2)
3. Fire `item.found` event
4. If `--return`: fire `item.moved` (rehome) event
5. Return `Result`

### `cmd/found/helpers.go`

Thin wrappers delegating to `internal/cli`:
```go
func openDatabase(ctx context.Context) (*database.Database, error) {
    return cli.OpenDatabase(ctx)
}
func resolveLocation(ctx context.Context, db *database.Database, input string) (string, error) {
    return cli.ResolveLocation(ctx, db, input)
}
func resolveItemSelector(ctx context.Context, db *database.Database, selector string) (string, error) {
    return cli.ResolveItemSelector(ctx, db, selector, "wherehouse found")
}
```

### `cmd/found/doc.go`

```go
// Package found implements the wherehouse found command,
// which records that a previously missing or lost item has been found.
package found
```

---

## 11. Testing Strategy

### Unit Tests (`cmd/found/item_test.go`)

Test coverage should include:
1. Basic found - item at Missing, no --return
2. Found with --return - item at Missing, home inferred from TempOriginLocationID
3. Found with --return and explicit --home
4. Found with --return when foundLocation == homeLocation (no-op return)
5. Item not at Missing (warn but succeed)
6. Item not found (selector error)
7. Found location not found
8. Found location is system (error)
9. Multiple items - all succeed
10. Multiple items - second fails (fail-fast, first committed)

### Integration Tests

Follow `cmd/move/item_test.go` pattern using in-memory SQLite. Tests should:
- Create locations and items
- Mark item missing (or use direct state)
- Run `found` command
- Verify projection state after
- Verify event log entries

---

## 12. Root Command Registration

In `cmd/root.go` (or wherever commands are registered), add:

```go
import "github.com/asphaltbuffet/wherehouse/cmd/found"
// ...
rootCmd.AddCommand(found.GetFoundCmd())
```

---

## 13. Key Design Decisions

### Decision 1: Reuse `item.found` event type, not a new event

**Rationale**: The event already exists in the dispatcher and has a handler. Creating `item.located` or similar would add complexity for no gain. The existing semantics match the command's intent.

### Decision 2: `--return` fires two separate events (not one combined event)

**Rationale**: Event sourcing requires atomic, meaningful events. "Item was found" and "Item was moved home" are two distinct domain facts. Combining them into a single event would obscure history and complicate replay. Two events in sequence is correct.

### Decision 3: Home location inference over mandatory `--home` flag

**Rationale**: In the common case (item was in temporary use), home is already tracked in `TempOriginLocationID`. Requiring `--home` always would be friction. We infer when possible and fail with a clear error when not.

### Decision 4: Fail-fast for multiple items (consistent with `move`)

**Rationale**: Consistency with existing command behavior. Users can re-run the command for remaining items. Partial success with some committed is transparent (events are visible in the log).

### Decision 5: Cannot "find" at system locations

**Rationale**: Finding an item "at Missing" is nonsensical - Missing IS the not-found state. Finding "at Borrowed" is semantically wrong. The `--in` location must be a real user-defined location.
