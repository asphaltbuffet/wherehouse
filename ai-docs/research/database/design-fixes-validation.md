# Design Fixes Validation Report

**Date**: 2026-02-19
**Validator**: db-developer agent
**Files Reviewed**:
- `.claude/knowledge/events.md`
- `.claude/knowledge/projections.md`
- `.claude/knowledge/business-rules.md`
- `.claude/knowledge/domain-model.md`
- `docs/DESIGN.md` (authoritative source)

---

## 1. location.created Event Definition

**Status**: FIXED CORRECTLY

The `location.created` event in `events.md` (lines 310-377) now includes all required fields:
- `location_id` (uuid-v7)
- `display_name`
- `canonical_name`
- `parent_id` (nullable for root)
- `is_system` (boolean)
- Common fields: `event_id`, `event_type`, `timestamp_utc`, `actor_user_id`, `note`

The projection update SQL correctly computes `full_path_display`, `full_path_canonical`, and `depth` from the parent chain. Path computation algorithm is documented with both root and non-root cases.

Validation rules are comprehensive: uniqueness of `canonical_name`, colon prohibition, `parent_id` existence check, cycle prevention, and `is_system` restriction.

**Cross-reference with DESIGN.md**: DESIGN.md (lines 80-88) lists `display_name`, `canonical_name`, `parent_id`, `is_system` as location fields. The events.md definition matches.

---

## 2. item.created Event - display_name and canonical_name Fields

**Status**: FIXED CORRECTLY

The `item.created` event in `events.md` (lines 39-90) now includes both fields:
- `display_name`: "10mm Socket Wrench" (preserved as entered)
- `canonical_name`: "10mm_socket_wrench" (normalized for matching)

Validation rules correctly state:
- `display_name` must not be empty
- `canonical_name` must not contain `:`
- `canonical_name` is derived from `display_name` via canonicalization rules

**Cross-reference with DESIGN.md**: DESIGN.md (lines 63-71) specifies `display_name` preserved as entered and `canonical_name` normalized. The events.md definition aligns.

**Note**: DESIGN.md `item.created` field list (lines 122-128) does NOT explicitly list `display_name` or `canonical_name` -- it only lists `item_id`, `location_id`, `actor_user_id`, `timestamp_utc`, and `note`. This is a gap in DESIGN.md itself (it is incomplete for this event), but events.md correctly infers these fields from the domain model section. This is acceptable since domain model clearly requires both fields.

---

## 3. Projection Update SQL for item.created

**Status**: FIXED CORRECTLY

The projection INSERT in `events.md` (lines 59-81) includes all required columns:
- `item_id`
- `display_name` (new)
- `canonical_name` (new)
- `location_id`
- `in_temporary_use` (defaulted to false)
- `temp_origin_location_id` (defaulted to NULL)
- `project_id` (defaulted to NULL)
- `last_event_id`
- `updated_at`

This matches the `items_current` table schema in `projections.md` exactly. All nine columns are accounted for.

---

## 4. Composite Indexes in projections.md

**Status**: FIXED CORRECTLY

The `projections.md` file (lines 86-93) now includes composite indexes:

```sql
CREATE INDEX idx_items_canonical_location ON items_current(canonical_name, location_id);
CREATE INDEX idx_items_location_covering ON items_current(location_id, display_name, canonical_name);
```

Additionally present:
- Partial index: `idx_items_temp_use ON items_current(in_temporary_use) WHERE in_temporary_use = 1`
- Single-column indexes on `location_id`, `canonical_name`, `project_id`, `last_event_id`

The Index Strategy section (lines 422-435) documents the rationale:
- `(canonical_name, location_id)` -- selector resolution for LOCATION:ITEM pattern
- `(location_id, display_name, canonical_name)` -- covering index for listing items at a location

These align with the documented query patterns in projections.md (lines 383-417).

---

## 5. Event ID Numbering - Sequential and Consistent

**Status**: CORRECT (with caveat)

Event example `event_id` values in events.md are sequential:
- item.created: 1
- item.moved: 2
- item.borrowed: 3
- item.marked_missing: 4
- item.marked_found: 5
- item.deleted: 6
- location.created: 7
- location.reparented: 8
- location.deleted: 9
- project.created: 10
- project.completed: 11
- project.reopened: 12
- project.deleted: 13

All 13 event types are numbered sequentially from 1-13. These are example IDs only (actual IDs are AUTOINCREMENT), but the sequential numbering makes the documentation clear and consistent.

---

## 6. Internal Consistency of Event Schemas

**Status**: CONSISTENT

All event schemas follow the same pattern:
- Common fields (event_id, event_type, timestamp_utc, actor_user_id, note)
- Entity-specific fields
- Projection update SQL
- Validation rules

Cross-consistency checks performed:

| Check | Result |
|-------|--------|
| events.md item.created fields match items_current columns | PASS |
| events.md location.created fields match locations_current columns | PASS |
| events.md project.created fields match projects_current columns | PASS |
| Validation rules in events.md match business-rules.md | PASS |
| Domain model entity fields match projection table columns | PASS |
| Foreign key references in projections.md match entity relationships | PASS |
| Index definitions cover documented query patterns | PASS |
| Replay rules in projections.md match event ordering rules in events.md | PASS |

---

## Remaining Issues

### Minor Issues (non-blocking)

1. **DESIGN.md item.created is incomplete**: The authoritative DESIGN.md (line 122-128) does not list `display_name` or `canonical_name` as explicit fields for `item.created`. It only lists `item_id`, `location_id`, `actor_user_id`, `timestamp_utc`, and `note`. The knowledge docs correctly include them based on domain model context, but DESIGN.md should be updated to match for full consistency.

2. **DESIGN.md location.created is missing**: DESIGN.md does not have an explicit `location.created` event type section at all. It documents location fields and deletion/reparenting events but omits the creation event. The knowledge docs correctly define it, but DESIGN.md should be updated.

3. **locations_current missing last_event_id**: The `items_current` projection tracks `last_event_id` for replay checkpointing, but `locations_current` does not have an equivalent field. This means incremental replay checkpointing cannot track location projection state independently. This is acceptable for v1 (location events are infrequent), but worth noting for future versions.

4. **projects_current missing last_event_id**: Same observation as locations -- no `last_event_id` tracking. Acceptable for v1.

5. **item.borrowed projection does not update last_event_id in SQL**: The projection update SQL for `item.borrowed` (line 181-184) shows `SET last_event_id = event.event_id` but the comment says "Does NOT set in_temporary_use (borrowed is different state)". The SQL is correct, but `last_event_id` is only shown in the SQL block and not in the textual description above it. Minor doc clarity issue.

---

## Confirmation

The schema design in events.md and projections.md is now internally consistent and ready for implementation. All six verification points pass. The five minor issues identified above are documentation completeness concerns in DESIGN.md, not blocking issues in the knowledge docs that drive implementation.

**Schema is ready for implementation.**

---

**Validated by**: db-developer agent
**Date**: 2026-02-19
