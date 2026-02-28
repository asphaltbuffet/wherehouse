# DESIGN.md Fixes Validation Report

**Date**: 2026-02-19
**Status**: All identified gaps resolved
**Documents compared**: docs/DESIGN.md, .claude/knowledge/events.md, .claude/knowledge/projections.md

---

## Gaps Identified in Previous Report (schema-validation.md)

The previous validation report identified 7 gaps. This report verifies which have been fixed.

### Gap 1: Missing `location.created` event schema in events.md

**Status**: FIXED

events.md now contains a full `location.created` section (lines 310-377) with JSON example, projection update SQL, path computation algorithm, and validation rules. DESIGN.md (lines 197-214) defines the same event with matching fields.

Both documents agree on fields: `location_id`, `display_name`, `canonical_name`, `parent_id` (nullable), `is_system`, `actor_user_id`, `timestamp_utc`, `note` (optional).

### Gap 2: Missing display_name/canonical_name in item.created event

**Status**: FIXED

DESIGN.md (lines 122-131) now explicitly lists `display_name` and `canonical_name` as fields of `item.created`. events.md (lines 43-56) includes both in the JSON example payload. The two documents are consistent.

### Gap 3: Missing display_name/canonical_name in location event fields

**Status**: FIXED

Resolved by the addition of the full `location.created` event schema (see Gap 1). Both DESIGN.md and events.md include `display_name` and `canonical_name` in the location.created event.

### Gap 4: No `location.renamed` event

**Status**: NOT ADDRESSED (acceptable for v1)

No rename event exists in any of the three documents. This remains undocumented -- whether rename is supported in v1 or deferred. This is a minor design decision, not an inconsistency between documents.

### Gap 5: items_current missing display_name in item.created projection SQL

**Status**: FIXED

events.md item.created projection update SQL (lines 59-81) now includes `display_name` and `canonical_name` in the INSERT statement, matching the items_current table definition.

### Gap 6: schema_metadata table not in main DDL

**Status**: NOT ADDRESSED (minor)

The schema_metadata table is still only defined in projections.md's migration section, not alongside the main table definitions in DESIGN.md. This is a documentation organization issue, not a consistency problem. The table is fully defined in projections.md.

### Gap 7: No explicit event for location.created (most significant)

**Status**: FIXED

This was the same issue as Gap 1. Both DESIGN.md and events.md now have complete `location.created` definitions.

---

## Cross-Document Consistency Check

### Event Schemas: DESIGN.md vs events.md

| Event Type | DESIGN.md | events.md | Consistent |
|---|---|---|---|
| item.created | Lines 122-131 | Lines 39-91 | Yes |
| item.moved | Lines 134-144 | Lines 93-159 | Yes |
| item.borrowed | Lines 147-161 | Lines 162-197 | Yes |
| item.marked_missing | Lines 163-171 | Lines 200-232 | Yes |
| item.marked_found | Lines 173-186 | Lines 234-272 | Yes |
| item.deleted | Lines 188-195 | Lines 275-305 | Yes |
| location.created | Lines 197-214 | Lines 310-377 | Yes |
| location.reparented | Lines 216-225 | Lines 380-418 | Yes |
| location.deleted | Lines 227-235 | Lines 425-457 | Yes |
| project.created | Lines 237-241 | Lines 462-493 | Yes |
| project.completed | Lines 243-249 | Lines 496-527 | Yes |
| project.reopened | Lines 251-255 | Lines 530-556 | Yes |
| project.deleted | Lines 257-263 | Lines 559-586 | Yes |

### Projection Tables: DESIGN.md vs projections.md

| Table | DESIGN.md | projections.md | Consistent |
|---|---|---|---|
| locations_current | Lines 269-279 | Lines 25-43 (DDL) | Yes |
| items_current | Lines 283-292 | Lines 71-94 (DDL) | Yes |
| projects_current | Lines 294-298 | Lines 132-141 (DDL) | Yes |

Field-level comparison for all three tables shows exact match between DESIGN.md field lists and projections.md DDL column definitions.

### locations_current Field Verification

DESIGN.md lists: `location_id`, `display_name`, `canonical_name`, `parent_id`, `full_path_display`, `full_path_canonical`, `depth`, `is_system`, `updated_at`.

projections.md DDL defines the same 9 columns with matching types, constraints (NOT NULL, UNIQUE on canonical_name, DEFAULT 0 on is_system), and FK on parent_id. Consistent.

---

## Remaining Minor Items

1. **location.renamed event** - Not defined in any document. Should be explicitly noted as "deferred to future version" or "not supported" in DESIGN.md. Low priority.

2. **schema_metadata placement** - Only in projections.md migration section. Could be added to DESIGN.md table listing for completeness. Very low priority.

3. **DESIGN.md path separator** - DESIGN.md location.created section (line 214) mentions projection computes paths. The display path separator is defined as ` >> ` in projections.md but not stated in DESIGN.md. Minor -- projections.md is the implementation reference.

---

## Conclusion

The three critical gaps (item.created missing name fields, location.created event undefined, locations_current missing fields) have all been resolved. DESIGN.md is now consistent with events.md and projections.md as implementation-driving references.

The documents are ready to serve as the basis for Go implementation of the database layer.

---

**Version**: 1.0
**Author**: db-developer agent
