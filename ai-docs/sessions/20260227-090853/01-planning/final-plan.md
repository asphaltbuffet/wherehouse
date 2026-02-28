# Final Implementation Plan: `wherehouse found` Command

**Date**: 2026-02-27
**Session**: 20260227-090853
**Status**: Final (incorporates user clarifications)

---

## 1. Summary of Changes from Initial Plan

The user clarifications resolved three open questions:

1. **Non-Missing items**: Warn and proceed (do NOT error). Same behavior for Borrowed items.
2. **--return no-op** (found location == home location): Fire `item.found`, skip `item.moved`, print note "already at home location".
3. **Home location for --return**: Use `temp_origin_location_id` only. If `temp_origin_location_id` is NULL, skip the move and print a warning "unable to determine home location". No `--home` flag is needed.
4. **No project association flags**: `found` has no `--project`/`--keep-project` flags (simpler interface).

The `--home` flag from the initial plan is **dropped**. Home is always inferred from `TempOriginLocationID`.

---

## 2. Event Sequence

### 2.1 `found` (without `--return`)

Single event fired:

```
item.found {
  item_id:           <resolved item UUID>
  found_location_id: <resolved --in location UUID>
  home_location_id:  <inferred from TempOriginLocationID, or found_location_id if NULL>
}
```

**Projection result**: `location_id = found_location_id`, `in_temporary_use = true`, `temp_origin_location_id = home_location_id`.

When `TempOriginLocationID` is NULL, `home_location_id` in the event is set to `found_location_id`. This records the found location as the item's home, establishing a baseline for future return tracking.

### 2.2 `found --return` (normal case: TempOriginLocationID is set, home != found)

Two events fired in sequence:

```
EVENT 1: item.found {
  item_id:           <item UUID>
  found_location_id: <--in location UUID>
  home_location_id:  <item.TempOriginLocationID>
}

EVENT 2: item.moved {
  item_id:          <item UUID>
  from_location_id: <--in location UUID>   -- current after event 1
  to_location_id:   <item.TempOriginLocationID>
  move_type:        "rehome"
  project_action:   "clear"
}
```

**Projection result after both events**: item is at `home_location_id`, `in_temporary_use = false`, `temp_origin_location_id = NULL`.

### 2.3 `found --return` (no-op: found location == home location)

Single event fired, return skipped with note:

```
EVENT 1: item.found {
  item_id:           <item UUID>
  found_location_id: <--in location UUID>
  home_location_id:  <item.TempOriginLocationID>
}

-- item.moved is SKIPPED because foundLocationID == homeLocationID
```

Output note: `"already at home location - return skipped"`

### 2.4 `found --return` (NULL TempOriginLocationID)

Single event fired, return skipped with warning:

```
EVENT 1: item.found {
  item_id:           <item UUID>
  found_location_id: <--in location UUID>
  home_location_id:  <found_location_id>   -- fallback: found IS home
}

-- item.moved is SKIPPED because home cannot be determined
```

Output warning: `"home location unknown - could not return item"`

---

## 3. Files to Create/Modify

### 3.1 New Files

```
cmd/found/
├── doc.go        -- package doc comment
├── found.go      -- GetFoundCmd(), cobra command, flags
├── helpers.go    -- thin wrappers to internal/cli
└── item.go       -- runFoundItem(), foundItem(), Result type
```

### 3.2 Modified Files

```
cmd/root.go       -- add found.GetFoundCmd() registration
```

No database layer changes required. All needed DB methods exist.

---

## 4. File-by-File Specification

### 4.1 `cmd/found/doc.go`

```go
// Package found implements the wherehouse found command,
// which records that a previously missing or lost item has been found.
package found
```

### 4.2 `cmd/found/found.go`

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
        Short: "Record that a lost or missing item has been found",
        Long: `Record that one or more items have been found at a specific location.

The item's home location is NOT changed by default. Use --return to also
move the item back to its home location immediately.

Selector types:
  - UUID:          550e8400-e29b-41d4-a716-446655440001
  - LOCATION:ITEM: garage:socket (both canonical names)
  - Canonical:     "10mm socket" (must match exactly 1 item)

Examples:
  wherehouse found "10mm socket" --in garage
  wherehouse found "10mm socket" --in garage --return
  wherehouse found garage:screwdriver --in shed --note "behind workbench"`,
        Args: cobra.MinimumNArgs(1), //nolint:mnd // 1 is the minimum required arg count
        RunE: runFoundItem,
    }

    foundCmd.Flags().StringP("in", "i", "", "location where item was found (required)")
    _ = foundCmd.MarkFlagRequired("in")

    foundCmd.Flags().BoolP("return", "r", false, "also return item to its home location")
    foundCmd.Flags().StringP("note", "n", "", "optional note for event")

    return foundCmd
}
```

**Key flag decisions**:
- `--in` / `-i`: required, the found location
- `--return` / `-r`: optional bool, triggers second event
- `--note` / `-n`: optional, applied to both events when `--return`
- No `--home` flag (dropped from initial plan per user clarification)
- No project flags (per user clarification: keep interface simple)

### 4.3 `cmd/found/helpers.go`

```go
package found

import (
    "context"

    "github.com/asphaltbuffet/wherehouse/internal/cli"
    "github.com/asphaltbuffet/wherehouse/internal/database"
)

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

### 4.4 `cmd/found/item.go`

#### Result type

```go
type Result struct {
    ItemID          string  `json:"item_id"`
    DisplayName     string  `json:"display_name"`
    FoundAt         string  `json:"found_at"`
    HomeLocation    string  `json:"home_location"`
    Returned        bool    `json:"returned"`
    FoundEventID    int64   `json:"found_event_id"`
    ReturnEventID   *int64  `json:"return_event_id,omitempty"`
    Warnings        []string `json:"warnings,omitempty"`
}
```

#### `runFoundItem` (main cobra handler)

```go
func runFoundItem(cmd *cobra.Command, args []string) error {
    ctx := cmd.Context()

    foundLocationStr, _ := cmd.Flags().GetString("in")
    returnToHome, _      := cmd.Flags().GetBool("return")
    note, _              := cmd.Flags().GetString("note")

    db, err := openDatabase(ctx)
    if err != nil {
        return fmt.Errorf("failed to open database: %w", err)
    }
    defer db.Close()

    actorUserID := cli.GetActorUserID(ctx)

    foundLocationID, err := resolveLocation(ctx, db, foundLocationStr)
    if err != nil {
        return fmt.Errorf("found location not found: %w", err)
    }

    // Validate --in is not a system location
    if sysErr := validateNotSystemLocation(ctx, db, foundLocationID); sysErr != nil {
        return sysErr
    }

    cfg := cli.MustGetConfig(ctx)
    out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

    var results []Result

    for _, selector := range args {
        itemID, itemErr := resolveItemSelector(ctx, db, selector)
        if itemErr != nil {
            return fmt.Errorf("failed to resolve %q: %w", selector, itemErr)
        }

        result, foundErr := foundItem(ctx, db, itemID, foundLocationID, returnToHome, actorUserID, note)
        if foundErr != nil {
            return fmt.Errorf("failed to record found for %q: %w", selector, foundErr)
        }

        results = append(results, *result)

        if !cfg.IsJSON() {
            // Print primary success message
            msg := formatSuccessMessage(result)
            out.Success(msg)
            // Print any warnings
            for _, w := range result.Warnings {
                out.Warning(w)
            }
        }
    }

    if cfg.IsJSON() {
        output := map[string]any{"found": results}
        if jsonErr := out.JSON(output); jsonErr != nil {
            return fmt.Errorf("failed to encode JSON output: %w", jsonErr)
        }
    }

    return nil
}
```

#### `foundItem` (core logic)

```go
func foundItem(
    ctx context.Context,
    db *database.Database,
    itemID, foundLocationID string,
    returnToHome bool,
    actorUserID, note string,
) (*Result, error) {
    // 1. Get current item state
    item, err := db.GetItem(ctx, itemID)
    if err != nil {
        return nil, fmt.Errorf("item not found: %w", err)
    }

    // 2. Get current item location for warning check
    currentLoc, err := db.GetLocation(ctx, item.LocationID)
    if err != nil {
        return nil, fmt.Errorf("current location not found: %w", err)
    }

    // 3. Collect warnings (non-fatal)
    var warnings []string
    if currentLoc.IsSystem && currentLoc.CanonicalName != "missing" {
        // Item is at Borrowed or Loaned - unusual for "found"
        warnings = append(warnings, fmt.Sprintf(
            "item is currently at system location %q (not Missing)", currentLoc.DisplayName))
    } else if !currentLoc.IsSystem {
        // Item is at a normal location - unusual for "found"
        warnings = append(warnings, fmt.Sprintf(
            "item is not currently missing (currently at %q)", currentLoc.DisplayName))
    }
    // No warning if currentLoc.CanonicalName == "missing" - that's the normal case

    // 4. Determine home location for the item.found event
    homeLocationID := foundLocationID // fallback: found location IS home
    if item.TempOriginLocationID != nil {
        homeLocationID = *item.TempOriginLocationID
    }

    // 5. Get location display names for result
    foundLoc, err := db.GetLocation(ctx, foundLocationID)
    if err != nil {
        return nil, fmt.Errorf("found location details not found: %w", err)
    }
    homeLoc, err := db.GetLocation(ctx, homeLocationID)
    if err != nil {
        return nil, fmt.Errorf("home location details not found: %w", err)
    }

    // 6. Fire item.found event
    payload := map[string]any{
        "item_id":           itemID,
        "found_location_id": foundLocationID,
        "home_location_id":  homeLocationID,
    }
    foundEventID, err := db.AppendEvent(ctx, "item.found", actorUserID, payload, note)
    if err != nil {
        return nil, fmt.Errorf("failed to create found event: %w", err)
    }

    result := &Result{
        ItemID:       itemID,
        DisplayName:  item.DisplayName,
        FoundAt:      foundLoc.DisplayName,
        HomeLocation: homeLoc.DisplayName,
        Returned:     false,
        FoundEventID: foundEventID,
        Warnings:     warnings,
    }

    // 7. Handle --return
    if returnToHome {
        if item.TempOriginLocationID == nil {
            // Home unknown - skip move, warn
            result.Warnings = append(result.Warnings,
                "home location unknown - could not return item (use move command to return manually)")
        } else if foundLocationID == homeLocationID {
            // Already at home - skip move, note
            result.Warnings = append(result.Warnings,
                "already at home location - return skipped")
        } else {
            // Fire item.moved rehome event
            movePayload := map[string]any{
                "item_id":          itemID,
                "from_location_id": foundLocationID,
                "to_location_id":   homeLocationID,
                "move_type":        "rehome",
                "project_action":   "clear",
            }
            returnEventID, moveErr := db.AppendEvent(ctx, "item.moved", actorUserID, movePayload, note)
            if moveErr != nil {
                return nil, fmt.Errorf("failed to create return event: %w", moveErr)
            }
            result.Returned = true
            result.ReturnEventID = &returnEventID
        }
    }

    return result, nil
}
```

#### `validateNotSystemLocation` helper

```go
func validateNotSystemLocation(ctx context.Context, db *database.Database, locationID string) error {
    loc, err := db.GetLocation(ctx, locationID)
    if err != nil {
        return fmt.Errorf("failed to get location: %w", err)
    }
    if loc.IsSystem {
        return fmt.Errorf(
            "cannot record item as found at system location %q\nUse a real location for --in",
            loc.DisplayName,
        )
    }
    return nil
}
```

#### `formatSuccessMessage` helper

```go
func formatSuccessMessage(r *Result) string {
    if r.Returned {
        return fmt.Sprintf("Found %q at %s, returned to %s", r.DisplayName, r.FoundAt, r.HomeLocation)
    }
    return fmt.Sprintf("Found %q at %s (home: %s)", r.DisplayName, r.FoundAt, r.HomeLocation)
}
```

### 4.5 `cmd/root.go` (modification)

Add import and registration:

```go
import "github.com/asphaltbuffet/wherehouse/cmd/found"

// In command registration section:
rootCmd.AddCommand(found.GetFoundCmd())
```

---

## 5. Validation Logic

### 5.1 Pre-event Validation Table

| Check | Condition | Action |
|-------|-----------|--------|
| Item exists | `db.GetItem` returns error | Fail: item not found |
| Found location exists | `db.GetLocation` returns error | Fail: location not found |
| Found location is system | `loc.IsSystem == true` | Fail: cannot find at system location |
| Current location is Missing | `currentLoc.CanonicalName == "missing"` | No warning (normal case) |
| Current location is non-Missing system | `currentLoc.IsSystem && canonical != "missing"` | Warn, proceed |
| Current location is normal (not system) | `!currentLoc.IsSystem` | Warn, proceed |
| TempOriginLocationID is NULL | `item.TempOriginLocationID == nil` | home = foundLocationID (fallback) |
| TempOriginLocationID is set | `item.TempOriginLocationID != nil` | home = *TempOriginLocationID |

### 5.2 `--return` Decision Tree

```
IF returnToHome == true:
  IF item.TempOriginLocationID == nil:
    SKIP move
    ADD warning: "home location unknown - could not return item"
  ELSE IF foundLocationID == homeLocationID:
    SKIP move
    ADD warning: "already at home location - return skipped"
  ELSE:
    FIRE item.moved (rehome, clear project)
    SET result.Returned = true
    SET result.ReturnEventID = &returnEventID
```

### 5.3 What is NOT validated (intentional)

- No `from_location_id` check in `item.found` (unlike `item.moved`). The `item.found` event handler does not require from-location validation. This is correct: the item's current location (Missing, Borrowed, normal) is irrelevant to the found event.
- The `item.moved` event for `--return` uses `foundLocationID` as `from_location_id`. After `item.found` fires, the projection is updated to `location_id = foundLocationID`, so the subsequent `item.moved` from-location will match.

---

## 6. Output Formats

### 6.1 Human-readable (default)

Without `--return`:
```
Found "10mm socket" at Garage (home: Tote F)
```

With `--return` (returned):
```
Found "10mm socket" at Garage, returned to Tote F
```

With `--return` (already at home - skipped):
```
Found "10mm socket" at Garage (home: Garage)
warning: already at home location - return skipped
```

With `--return` (home unknown - skipped):
```
Found "10mm socket" at Garage (home: Garage)
warning: home location unknown - could not return item (use move command to return manually)
```

Non-missing item (warning only):
```
warning: item is not currently missing (currently at "Tote F")
Found "10mm socket" at Garage (home: Tote F)
```

### 6.2 JSON (`--json`)

Without `--return`:
```json
{
  "found": [
    {
      "item_id": "019532ab-...",
      "display_name": "10mm socket",
      "found_at": "Garage",
      "home_location": "Tote F",
      "returned": false,
      "found_event_id": 42
    }
  ]
}
```

With `--return` (returned):
```json
{
  "found": [
    {
      "item_id": "019532ab-...",
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

With warnings:
```json
{
  "found": [
    {
      "item_id": "019532ab-...",
      "display_name": "10mm socket",
      "found_at": "Garage",
      "home_location": "Garage",
      "returned": false,
      "found_event_id": 42,
      "warnings": ["home location unknown - could not return item (use move command to return manually)"]
    }
  ]
}
```

### 6.3 Quiet (`--quiet`)

Produces no output on success. Warnings are suppressed. Errors still print to stderr.

---

## 7. Test Cases

### 7.1 Core Happy Path Tests

| # | Scenario | Setup | Expected |
|---|----------|-------|----------|
| 1 | Basic found, item at Missing | Item at Missing, TempOriginLocationID=Tote F | item.found event, location=Garage, in_temporary_use=true, temp_origin=Tote F |
| 2 | Found + return, item at Missing with known home | Item at Missing, TempOriginLocationID=Tote F, --in=Garage --return | item.found event + item.moved event, final location=Tote F, in_temporary_use=false |
| 3 | Found + return, already at home | Item at Missing, TempOriginLocationID=Garage, --in=Garage --return | item.found only, warning "already at home" |
| 4 | Found + return, NULL home | Item at Missing, TempOriginLocationID=NULL, --in=Garage --return | item.found only, home=Garage (fallback), warning "home unknown" |
| 5 | Multiple items, all succeed | Items A, B at Missing | 2x item.found events, both reported |

### 7.2 Warning Cases (warn and proceed)

| # | Scenario | Expected |
|---|----------|----------|
| 6 | Item at normal location (not missing) | Warning "item is not currently missing", proceeds, item.found fires |
| 7 | Item at Borrowed (system, non-missing) | Warning "item is currently at system location Borrowed", proceeds |
| 8 | Item at Loaned (system, non-missing) | Warning "item is currently at system location Loaned", proceeds |

### 7.3 Error Cases (hard fail)

| # | Scenario | Expected |
|---|----------|----------|
| 9 | Item selector not found | Error: "failed to resolve..." |
| 10 | --in location not found | Error: "found location not found" |
| 11 | --in is system location (Missing) | Error: "cannot record item as found at system location" |
| 12 | --in is system location (Borrowed) | Error: "cannot record item as found at system location" |
| 13 | Multiple items, second fails | First item committed, second returns error (fail-fast) |

### 7.4 Event State Verification Tests

| # | Scenario | Verify |
|---|----------|--------|
| 14 | item.found sets in_temporary_use | `items_current.in_temporary_use = 1` after found |
| 15 | item.found sets temp_origin | `items_current.temp_origin_location_id = homeLocationID` |
| 16 | item.found + item.moved (rehome) clears temp state | `in_temporary_use = 0`, `temp_origin = NULL` |
| 17 | Event log has correct count | found (no return) = 1 event; found+return = 2 events |
| 18 | item.moved from_location_id matches post-found location | Projection consistency: from_location_id in move event = foundLocationID |

### 7.5 Edge Cases

| # | Scenario | Expected |
|---|----------|----------|
| 19 | Item already at found location (not missing) | Warning "not currently missing", item.found fires (sets temp tracking even if location unchanged) |
| 20 | UUID selector (exact item ID) | Resolves correctly, same as name selector |
| 21 | LOCATION:ITEM selector | Resolves correctly via canonical names |
| 22 | Ambiguous item name | Error from resolveItemSelector (existing behavior) |

### 7.6 Output Format Tests

| # | Scenario | Expected |
|---|----------|----------|
| 23 | Human output without --return | `Found "X" at Y (home: Z)` |
| 24 | Human output with --return (returned) | `Found "X" at Y, returned to Z` |
| 25 | JSON output without --return | `{"found":[{..., "returned": false}]}` |
| 26 | JSON output with --return | `{"found":[{..., "returned": true, "return_event_id": N}]}` |
| 27 | Quiet mode | No stdout output |
| 28 | JSON with warnings | warnings array in result object |

---

## 8. Integration with Existing Code

### 8.1 Database Methods Used (all existing, no new methods needed)

| Method | Used for |
|--------|----------|
| `db.GetItem(ctx, itemID)` | Get current item state, TempOriginLocationID, LocationID |
| `db.GetLocation(ctx, locationID)` | Get IsSystem, DisplayName, CanonicalName for both current and found locations |
| `db.AppendEvent(ctx, "item.found", ...)` | Fire found event |
| `db.AppendEvent(ctx, "item.moved", ...)` | Fire return move event (--return only) |

### 8.2 CLI Package Methods Used (all existing)

| Method | Used for |
|--------|----------|
| `cli.OpenDatabase(ctx)` | Open DB from config |
| `cli.GetActorUserID(ctx)` | Get current user for event attribution |
| `cli.ResolveLocation(ctx, db, input)` | Resolve --in location string to UUID |
| `cli.ResolveItemSelector(ctx, db, selector, cmd)` | Resolve item selector to UUID |
| `cli.MustGetConfig(ctx)` | Get config for output mode |
| `cli.NewOutputWriterFromConfig(...)` | Create output writer |

### 8.3 Event Type in Router

The event type string is `"item.found"` (not `"item.marked_found"` as in the knowledge docs). This matches the existing router entry in `internal/database/eventHandler.go` line 42 and the handler `handleItemFound` in `itemEventHandler.go` line 188.

### 8.4 `item.found` Handler Behavior (no changes needed)

The existing `handleItemFound` executes:
```sql
UPDATE items_current
SET location_id = ?,        -- found_location_id
    in_temporary_use = 1,
    temp_origin_location_id = ?,  -- home_location_id
    last_event_id = ?,
    updated_at = ?
WHERE item_id = ?
```

This is correct. When `home_location_id == found_location_id` (NULL TempOriginLocationID fallback case), `temp_origin_location_id` is set to `found_location_id`. This is a reasonable default that makes the state internally consistent.

---

## 9. Root Command Registration

In `cmd/root.go`, after existing command registrations:

```go
import (
    // existing imports...
    "github.com/asphaltbuffet/wherehouse/cmd/found"
)

// in init() or command setup:
rootCmd.AddCommand(found.GetFoundCmd())
```

---

## 10. Key Design Decisions (Final)

### Decision 1: No `--home` flag

**Rationale**: User clarification confirmed: use `TempOriginLocationID` only. If NULL, skip return and warn. This keeps the interface simple and avoids the complexity of requiring users to know their item's home UUID/name.

### Decision 2: `home_location_id = found_location_id` when TempOriginLocationID is NULL

**Rationale**: The `item.found` event requires a `home_location_id` field (handler validates it). When we cannot determine home, we use `found_location_id` as a safe fallback. This means the item's `temp_origin_location_id` will point to where it was found, which is logically sound: "I found it here, so this is where it lives for now."

### Decision 3: Two separate events for `found --return` (not one atomic transaction)

**Rationale**: Consistent with existing multi-event operations in `move`. Each event is a distinct domain fact. `AppendEvent` uses per-call transactions (existing design). Partial failure (found succeeds, return fails) leaves item in "found but not returned" state, which is valid - user can run `move` to return manually.

### Decision 4: Fail-fast for multiple items

**Rationale**: Consistent with `move` command. Partial success is visible (events in log). Users re-run for remaining items.

### Decision 5: Warn for non-Missing items, do not error

**Rationale**: User clarification. The command records what happened ("I found it here") regardless of whether the item was formally marked missing. This is useful for general tracking, not just formal missing-item workflows.

### Decision 6: `--note` applies to both `item.found` and `item.moved` events

**Rationale**: Simplicity. A single note field covers the whole operation. The note is contextual to the "finding" event, and when an item is also returned, the note is relevant to that action too.

---

## 11. Linting Notes

- `cobra.MinimumNArgs(1)` triggers `mnd` linter - suppress with `//nolint:mnd // 1 is the minimum required arg count`
- Use `err =` (not `:=`) inside loops that already have `err` in scope to avoid `govet shadow`
- Verify with `mise run lint` after implementation

---

**Version**: 1.0 (final)
**Last Updated**: 2026-02-27
