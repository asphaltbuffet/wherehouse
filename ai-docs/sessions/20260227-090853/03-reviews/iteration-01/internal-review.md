# Code Review: `wherehouse found` Command

**Date**: 2026-02-27
**Reviewer**: code-reviewer agent
**Files Reviewed**:
- `/home/grue/dev/wherehouse/cmd/found/doc.go`
- `/home/grue/dev/wherehouse/cmd/found/found.go`
- `/home/grue/dev/wherehouse/cmd/found/helpers.go`
- `/home/grue/dev/wherehouse/cmd/found/item.go`

**Reference Files**:
- `/home/grue/dev/wherehouse/cmd/move/` (pattern reference)
- `/home/grue/dev/wherehouse/.claude/knowledge/events.md`
- `/home/grue/dev/wherehouse/.claude/knowledge/business-rules.md`
- `/home/grue/dev/wherehouse/internal/database/itemEventHandler.go` (event handler)
- `/home/grue/dev/wherehouse/ai-docs/sessions/20260227-090853/01-planning/final-plan.md`

---

## Pre-Review: Automated Linting

**Result**: PASS -- `mise run lint` reports 0 issues. No linting concerns.

---

## Strengths

1. **Faithful to plan**: The implementation matches the final plan spec almost line-for-line. Event payloads, decision trees, warning logic, and output formats all align.

2. **Correct event payload field names**: The `item.found` payload uses `item_id`, `found_location_id`, `home_location_id` -- matching exactly what `handleItemFound` in `internal/database/itemEventHandler.go:189-193` expects.

3. **Correct `item.moved` rehome payload**: The move payload includes `item_id`, `from_location_id`, `to_location_id`, `move_type`, `project_action` -- matching what `handleItemMoved` expects at `internal/database/itemEventHandler.go:44-51`.

4. **Pattern consistency with `cmd/move/`**: File structure (`doc.go`, `found.go`, `helpers.go`, `item.go`) mirrors `cmd/move/`. Helper wrappers delegate to `internal/cli`. Command registration pattern is identical. Output handling (JSON/human/quiet) follows the same structure.

5. **Correct `from_location_id` for rehome event**: The rehome `item.moved` event uses `foundLocationID` as `from_location_id` (line 181 of `item.go`). Since the preceding `item.found` event updates the projection to set `location_id = foundLocationID`, this will match the projection state when `handleItemMoved` executes. This is correct.

6. **Good warning logic**: The three-way switch on current location state (Missing = normal, other system = warn, non-system = warn) is clean and correct per the plan's specification.

7. **System location validation**: Correctly blocks finding items at system locations (Missing, Borrowed) via `validateNotSystemLocation`.

8. **Proper error wrapping**: All errors use `fmt.Errorf("context: %w", err)` pattern consistently.

---

## Concerns

### IMPORTANT (should fix before merge)

**1. Missing `from_location` validation before rehome `item.moved` event**

**File**: `/home/grue/dev/wherehouse/cmd/found/item.go`, lines 178-191

The `cmd/move/item.go` calls `db.ValidateFromLocation(ctx, itemID, item.LocationID)` at line 155 before creating the `item.moved` event. The `found` command does not call this validation before its rehome `item.moved` event.

While technically safe in the current flow (the preceding `item.found` event sets `location_id = foundLocationID` within the same sequential execution, and SQLite `MaxOpenConns(1)` prevents concurrent writes), this is a deviation from the established pattern. The `move` command validates explicitly as a defense-in-depth measure against projection corruption.

The counter-argument (stated in the plan section 5.3) is that the `item.found` event just updated the projection, so validation is redundant. This is a valid engineering trade-off. However, since `AppendEvent` calls are NOT in the same database transaction (each `AppendEvent` is its own transaction), there is a theoretical window between the two events where the projection state could be inconsistent if an error occurs between them.

**Recommendation**: Add `db.ValidateFromLocation(ctx, itemID, foundLocationID)` before the rehome `item.moved` `AppendEvent` call. This aligns with the move command pattern and adds defense-in-depth. The cost is one extra query; the benefit is consistency with the codebase pattern and protection against future changes.

**Severity**: IMPORTANT -- not a current bug, but a pattern violation and defense-in-depth gap.

---

### MINOR (consider fixing)

**2. Two separate transactions for `found --return` instead of one atomic operation**

**File**: `/home/grue/dev/wherehouse/cmd/found/item.go`, lines 150-191

The `item.found` and `item.moved` events are each created via separate `AppendEvent` calls, which means separate database transactions. If the first succeeds but the second fails, the item is left in a "found but not returned" state with `in_temporary_use = true`.

The plan explicitly acknowledges this (Decision 3, section 10): "Partial failure leaves item in found but not returned state, which is valid."

This is acceptable given:
- The intermediate state is valid (user can manually `move`)
- The pattern matches existing multi-event operations
- `AppendEvent` does not currently support multi-event transactions

**Recommendation**: No change needed now. If a future `AppendEvents` batch method is added, this could be tightened. Document the partial-failure behavior in the command help text or in a comment.

**3. Event type naming: `item.found` vs `item.marked_found` in knowledge docs**

The knowledge docs (`events.md`) use `item.marked_found` as the event type name, but the actual database handler uses `item.found` (confirmed in `eventHandler.go:42`). The implementation correctly uses `"item.found"`, matching the handler.

**Recommendation**: No code change needed. The knowledge docs (`events.md`) should be updated to use `item.found` to match the actual implementation. This is a documentation issue, not a code issue.

**4. Redundant `db.GetLocation` call for system location check**

**File**: `/home/grue/dev/wherehouse/cmd/found/item.go`, lines 133-135

`validateNotSystemLocation` (called at line 46) already fetched the location for `foundLocationID`. Then `foundItem` fetches it again at line 133. This results in two `GetLocation` calls for the same location ID.

**Recommendation**: This is minor (one extra query) and matches the move command's pattern (which also does redundant location fetches). Not worth changing for consistency. Note for awareness only.

---

## Questions

1. **Is quiet mode (`--quiet`) handled correctly?** The `out.Success()` and `out.Warning()` calls appear to be gated by `!cfg.IsJSON()`, but quiet mode suppression would depend on the `OutputWriter` implementation. The move command uses the same pattern, so this is likely correct, but worth verifying in testing.

2. **Should the `--return` flag's "already at home" case produce a different output format?** Currently it adds a warning and leaves `result.Returned = false`. The human output says `Found "X" at Y (home: Y)` followed by `warning: already at home location - return skipped`. This seems reasonable.

---

## Checklist Verification

### Event Handler Review
1. Validates from_location matches projection? -- N/A for `item.found` (no from_location). See IMPORTANT #1 for the rehome `item.moved`.
2. Creates event before updating projection? -- Yes, `AppendEvent` handles this atomically.
3. Event + projection in same transaction? -- Yes, per `AppendEvent` design.
4. No modification of events after creation? -- Correct, events are append-only.
5. Proper error handling (no silent repair)? -- Yes, all errors propagated.
6. Tests for validation failures? -- Deferred to tester agent.
7. Tests for projection consistency? -- Deferred to tester agent.

### CLI Code Review
1. Calls core logic (doesn't duplicate it)? -- Yes, delegates to `internal/cli` and `internal/database`.
2. Proper flag parsing and validation? -- Yes, `--in` is required, `--return` and `--note` are optional.
3. `--json` output format works? -- Yes, follows move command pattern.
4. Error messages are user-friendly? -- Yes, clear context in all error messages.
5. Exit codes used correctly? -- Yes, returns errors (non-zero) on failure.
6. Help text comprehensive? -- Yes, includes selector types and examples.
7. Tests for flag parsing? -- Deferred to tester agent.
8. Tests for output formatting? -- Deferred to tester agent.

---

## Summary

**Assessment**: APPROVED with one important recommendation

**Priority Fixes**:
1. (IMPORTANT) Add `ValidateFromLocation` call before the rehome `item.moved` event for defense-in-depth consistency with the move command pattern.

**Issue Counts**: CRITICAL: 0 | IMPORTANT: 1 | MINOR: 3

**Estimated Risk**: Low -- the implementation is correct, clean, and follows established patterns. The one important issue is a defense-in-depth gap, not a current bug.

**Testability Score**: Good -- clear separation of concerns, injectable dependencies via helpers, pure logic in `foundItem`.

**Pattern Compliance**: High -- faithfully follows `cmd/move/` conventions for file structure, error handling, output formatting, and fail-fast batch processing.
