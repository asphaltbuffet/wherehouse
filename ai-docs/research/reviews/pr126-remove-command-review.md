# Code Review: PR #126 - Remove Command & Project Cleanup

**Branch**: implement-removal-functionality -> main
**Reviewer**: code-reviewer agent
**Date**: 2026-04-06

---

## Pre-Review Automated Checks

- Linting: PASS (0 issues)
- Tests: PASS (685 tests, 5 skipped)
- Stringer generation: VERIFIED (ItemRemovedEvent, LocationRemovedEvent present in eventtype_string.go)

---

## Strengths

1. **Correct constructor pattern**: `NewRemoveCmd(db removeDB)` + `NewDefaultRemoveCmd()` follows project conventions perfectly.
2. **Per-command DB interface**: `removeDB` in `cmd/remove/db.go` is minimal and has `//go:generate mockery` directive.
3. **Event-sourcing integrity**: `handleItemRemoved` correctly looks up the Removed system location by canonical name within the transaction, matching the pattern used by `handleItemMissing`, `handleItemBorrowed`, etc.
4. **Atomic event+projection**: `AppendEvent` wraps both event insertion and `processEventInTx` in a single transaction. Correct.
5. **Exhaustive switch coverage**: `eventHandler.go` has cases for all 11 event types including `ItemRemovedEvent` and `LocationRemovedEvent`.
6. **Location removal guards**: `removeLocation` correctly checks `IsSystem`, items in location, and child locations before allowing removal.
7. **Removed items hidden from views**: `GetItemsByLocation` joins with `locations_current` and filters `canonical_name != 'removed'`. `GetRootLocations` excludes the Removed location. `SearchByName` excludes removed items from the item half of the UNION.
8. **Migration ordering**: 000004 (add Removed location) before 000005 (remove project tables) is correct -- the Removed location must exist before any item removal events.
9. **System location seeding**: `seedSystemLocations` in `schema_metadata.go` includes the Removed location with deterministic ID `sys0000004`, matching the migration.
10. **Test coverage is solid**: Tests cover happy path, already-removed rejection, system location protection, non-empty location rejection, not-found errors, JSON marshaling, search visibility, and location listing visibility.

---

## Concerns

### CRITICAL (must fix before merge)

**[C1] `location.removed` event payload missing `previous_parent_id` -- violates event schema**

File: `/home/grue/dev/wherehouse/cmd/remove/location.go` lines 53-55

The `location.removed` payload only includes `location_id`:

```go
payload := map[string]any{
    "location_id": locationID,
}
```

Per `ai-docs/knowledge/events.md` (line 427) and `ai-docs/knowledge/business-rules.md` (line 284), the `location.removed` event MUST include `previous_parent_id` to match the projection's current parent. This field is critical for replay validation -- without it, replay cannot verify projection integrity.

The `handleLocationRemoved` handler in `locationEventHandler.go` (line 165) does not validate `previous_parent_id` either, meaning this gap exists on both sides.

**Fix**: Include `previous_parent_id` in the payload (nullable, from `loc.ParentID`) and add validation in the handler.

---

### HIGH (should fix before merge)

**[H1] `GetItemsByCanonicalName` does not exclude removed items**

File: `/home/grue/dev/wherehouse/internal/database/item.go` lines 127-150

This query returns all items matching a canonical name, including those in the Removed location. This function is used by `cli.ResolveItemSelector` for selector resolution. If a user tries to interact with an item by canonical name, a removed item could be returned as a match, potentially causing confusing behavior.

`GetItemsByLocation` correctly excludes removed items via a JOIN, but `GetItemsByCanonicalName` does not.

**[H2] Orphaned project_id reference in history output**

File: `/home/grue/dev/wherehouse/cmd/history/output.go` line 244

```go
if projectID, ok := payload["project_id"].(string); ok && projectID != "" {
    details = append(details, fmt.Sprintf("Project: %s", projectID))
}
```

This code references `project_id` which was removed in this PR. While it is harmless (the field will never be present in new events), it is dead code that contradicts the intent of the project removal. Old events that had `project_id` in their payload would still render it, which is inconsistent with removing all project functionality.

**[H3] `item.removed` event payload includes `to_location_id` but handler ignores it**

File: `/home/grue/dev/wherehouse/cmd/remove/item.go` lines 61-65

The command-side payload includes `to_location_id`:
```go
payload := map[string]any{
    "item_id":              itemID,
    "previous_location_id": item.LocationID,
    "to_location_id":       removedLoc.LocationID,
}
```

But the handler in `itemEventHandler.go` (line 238) only unmarshals `item_id` and `previous_location_id`, then independently looks up the Removed location. The `to_location_id` field is stored in the event but never used during replay.

This creates a discrepancy: the event claims a specific `to_location_id`, but replay ignores it and always uses whatever the current Removed system location ID is. Per the event schema in `events.md`, `item.removed` only specifies `previous_location_id` -- the `to_location_id` is not part of the schema. Remove it from the payload to avoid confusion.

---

### MEDIUM (consider fixing)

**[M1] ORDER BY clauses missing tiebreakers in item/location queries**

Files: `internal/database/item.go` lines 113, 140, 234; `internal/database/location.go` lines 289, 316, 408

Per project rules (CLAUDE.md: "every query that could tie must include `event_id ASC/DESC`"), these queries use `ORDER BY display_name` or `ORDER BY i.display_name` without a tiebreaker. Items can share the same display name (canonical names are not unique for items), so ordering is non-deterministic.

Note: This is a pre-existing issue not introduced by this PR, but the PR touched `item.go` and `location.go` and should have addressed it.

**[M2] `removeItem` does not check all system locations -- only checks "removed"**

File: `/home/grue/dev/wherehouse/cmd/remove/item.go` lines 46-48

The current check is:
```go
if location.IsSystem && location.CanonicalName == "removed" {
    return nil, fmt.Errorf("item %q is already removed", item.DisplayName)
}
```

This correctly allows removing items from Missing/Borrowed/Loaned (confirmed by tests). However, the decision to allow removal from any system location other than Removed should be documented in the code or at least noted in the design. The tests cover this well, so this is a minor documentation concern.

**[M3] `extractLocationFromEvent` does not handle `item.removed` event type**

File: `/home/grue/dev/wherehouse/internal/database/search.go` lines 317-342

The function maps event types to location fields for enrichment. The `item.removed` type is not listed, so if a removed item's last location resolution is attempted via this code path, it would fall through to the default (return empty). This function is only used by the enrichment path which already handles `IsRemoved`, so the impact is low, but it would be more correct to handle it.

---

## Questions

1. **Intent**: Should `GetAllItems` (used by migrations) also exclude removed items, or is it correct to include them for migration purposes? Currently it includes all items, which seems correct for rebuild scenarios but could cause confusion.

2. **Replay validation gap**: The `handleItemRemoved` handler does not validate that `previous_location_id` matches the current projection location. Other handlers like `handleItemMoved` also skip this validation at the handler level (it is done pre-event in the command). Is this intentional? During replay, if events were corrupted, this would not be caught for `item.removed`.

3. **Search behavior**: `SearchByName` excludes removed items from the item results but still includes the "Removed" location itself in location results. Is this intentional? A search for "removed" would show the system location.

---

## Summary

```
Assessment: Needs Changes

Priority Fixes:
1. [C1] Add previous_parent_id to location.removed event payload and handler validation
2. [H3] Remove to_location_id from item.removed payload (not in event schema)
3. [H2] Remove orphaned project_id reference in history output
4. [H1] Filter removed items from GetItemsByCanonicalName

Estimated Risk: Medium
Testability Score: Good
```

The core architecture is sound. Event-sourcing patterns are followed correctly for `item.removed` (append event, update projection atomically). The location removal correctly enforces empty-location and non-system constraints. The move command correctly blocks moves to/from system locations (including Removed). Test coverage is good with both happy paths and error paths exercised.

The critical issue (C1) is the missing `previous_parent_id` in the `location.removed` event, which breaks the replay validation contract documented in the event schema. The high issues are cleanup items that prevent the PR from being fully coherent with the project removal.
