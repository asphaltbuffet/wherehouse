# Wherehouse MVP Feature Development Plan

> **Status:** Feature Roadmap (High-Level)
> **Created:** 2026-02-20
> **Purpose:** Define MVP scope, feature sequence, and dependencies

**Goal:** Build a working event-sourced CLI inventory tracker that answers "Where did I put my 10mm socket?"

**Architecture:** Event log → Projections → CLI → TUI wrapper

**Core Principle:** Events are immutable source of truth, projections are disposable derived state

---

## Phase 0: Foundation (Prerequisites for all features)

**Features:**
- Database schema (events table, projection tables)
- SQLite connection management with WAL mode
- Configuration system (TOML via viper)
- Basic CLI framework (cobra)
- UUID generation (v7)
- Name canonicalization utilities

**Dependencies:** None (starting point)

**Deliverable:** `wherehouse --version` works, database initializes

---

## Phase 1: Location Management (Required before items)

### 1.1: Core Location Operations
**Features:**
- `location.created` event handler
- `locations_current` projection updates
- `wherehouse location create <name>` command
- `wherehouse location list` command
- Location path resolution (`:` separator)
- Parent validation (no cycles, parent exists)
- Canonical name uniqueness enforcement

**Dependencies:** Phase 0

**Validation Rules:**
- No `:` in location names
- Canonical names globally unique
- Parent must exist (or null for root)
- No cycles allowed

### 1.2: System Locations
**Features:**
- Create `Missing` location with `is_system=true`
- Create `Borrowed` location with `is_system=true`
- Block rename/delete/reparent of system locations

**Dependencies:** 1.1

### 1.3: Location Hierarchy Operations
**Features:**
- `location.reparented` event handler
- `wherehouse location move <location> --parent <new-parent>` command
- `location.deleted` event handler
- `wherehouse location delete <location>` command
- Validation: only delete empty locations

**Dependencies:** 1.1, 1.2

---

## Phase 2: Item Management (Core inventory tracking)

### 2.1: Item Creation and Lookup
**Features:**
- `item.created` event handler
- `items_current` projection updates
- `wherehouse item create <name> --location <location>` command
- `wherehouse where <item>` command
- Selector syntax parsing (`LOCATION:ITEM`)
- Duplicate canonical name warnings

**Dependencies:** Phase 1 (requires locations)

**Validation Rules:**
- No `:` in item names
- Location must exist at creation
- Duplicate canonical names allowed but warned

### 2.2: Item Movement
**Features:**
- `item.moved` event handler with move_type
- `wherehouse move <item> <to-location>` command
- `from_location_id` validation against projection
- `--temporary-use` flag handling
- `--rehome` flag (default)
- Temporary origin tracking

**Dependencies:** 2.1

**Business Rules:**
- First temporary use sets `temp_origin_location_id`
- Subsequent temporary uses preserve origin
- Rehome clears temporary state
- Default move type: rehome

### 2.3: Borrowed Items
**Features:**
- `item.borrowed` event handler
- `wherehouse borrow <item> --by <person>` command
- Automatic move to `Borrowed` location
- `borrowed_by` field (required, non-blank)
- Return = normal move from `Borrowed`

**Dependencies:** 2.2, 1.2 (Borrowed location)

### 2.4: Missing/Found Workflow
**Features:**
- `item.marked_missing` event handler
- `wherehouse mark-missing <item>` command
- Automatic move to `Missing` location
- `item.marked_found` event handler
- `wherehouse mark-found <item> --at <location> --home <location>` command
- Sets temporary use state on found

**Dependencies:** 2.2, 1.2 (Missing location)

### 2.5: Item Deletion
**Features:**
- `item.deleted` event handler
- `wherehouse item delete <item>` command
- Permanent removal from projections
- Record previous location

**Dependencies:** 2.1

---

## Phase 3: Project Context (Optional item associations)

### 3.1: Project Lifecycle
**Features:**
- `project.created` event handler
- `projects_current` projection
- `wherehouse project create <slug>` command
- `wherehouse project list` command
- `project.completed` event handler
- `wherehouse project complete <slug>` command
- `project.reopened` event handler
- `wherehouse project reopen <slug>` command

**Dependencies:** Phase 0 (no item dependency yet)

**Validation Rules:**
- No `:` in project slugs
- Globally unique slugs
- Must be active to associate with items

### 3.2: Project-Item Association
**Features:**
- Project field in `items_current`
- `--project <slug>` flag on move command
- `--keep-project` flag on move command
- `--clear-project` flag on move command
- Default behavior: clear project on move
- Show project in `where` output

**Dependencies:** 3.1, 2.2 (move command)

### 3.3: Project Deletion
**Features:**
- `project.deleted` event handler
- `wherehouse project delete <slug>` command
- Validation: only delete if no items associated

**Dependencies:** 3.2

---

## Phase 4: User Attribution (Multi-user support)

**Features:**
- User identity resolution (OS username)
- Config: `[users.*]` and `[user_identity.os_username_map]`
- `--as <user>` flag on all commands
- `actor_user_id` in all events
- Default to OS username
- Warning on unmapped users

**Dependencies:** Phase 0 (config system)

**Integration:** Add `actor_user_id` to all event handlers

---

## Phase 5: History and Auditing

**Features:**
- `wherehouse history <item>` command
- Semantic event summaries (default)
- Event icons (⁇ 󰍉 󱞬 󱞤 󰙴  󰆴 󱁤)
- `--raw` flag for full event details
- Chronological event display

**Dependencies:** Phase 2 (items), Phase 3 (projects optional)

---

## Phase 6: Validation and Integrity

### 6.1: Doctor Command
**Features:**
- `wherehouse doctor` command
- Structural integrity checks
- Event stream validation
- Projection consistency verification
- Transition validation
- `--rebuild` flag for projection rebuild
- Non-zero exit on failure

**Dependencies:** All phases (validates everything)

### 6.2: Replay System
**Features:**
- Projection rebuild from events
- Strict `event_id` ordering
- `from_location_id` validation during replay
- Deterministic replay guarantee
- Fail on inconsistency (no silent repair)

**Dependencies:** Phase 1, 2, 3 event handlers

---

## Phase 7: Export/Import (Backup and portability)

**Features:**
- `wherehouse export` command
- Export format: events (ordered) + projection snapshot + schema version + checksums
- Future: `wherehouse import` command
- Future: `--fast` flag to trust snapshot

**Dependencies:** Phase 6 (doctor for validation)

---

## Phase 8: Output Formats and Usability

**Features:**
- `--json` flag on all commands
- `-q` / `-qq` quiet modes
- `-v` / `-vv` verbose modes
- Structured JSON output
- Human-readable default output
- Exit code conventions

**Dependencies:** All command phases (cross-cutting)

---

## Phase 9: TUI (Terminal User Interface)

**Features:**
- TUI framework setup
- Thin wrapper over CLI layer
- No direct event writes
- No duplicated domain logic
- Interactive location browser
- Interactive item browser
- Command palette

**Architecture:** `TUI -> CLI Layer -> Domain Engine`

**Dependencies:** Phases 1-8 (requires working CLI)

---

## MVP Scope Definition

**Minimum Viable Product includes:**
- Phase 0: Foundation
- Phase 1: Location Management (1.1, 1.2, 1.3)
- Phase 2: Item Management (2.1, 2.2, 2.4)
- Phase 5: History (basic)
- Phase 6: Doctor (6.1 basic validation)
- Phase 8: Output Formats (basic)

**Post-MVP (can defer):**
- Phase 2.3: Borrowed Items
- Phase 2.5: Item Deletion
- Phase 3: Projects (entire phase)
- Phase 4: User Attribution (use default only)
- Phase 6.2: Replay System (manual rebuild)
- Phase 7: Export/Import
- Phase 9: TUI

**Critical Path Dependencies:**
```
Phase 0 (Foundation)
  ↓
Phase 1.1 (Locations) → 1.2 (System Locations) → 1.3 (Location Operations)
  ↓
Phase 2.1 (Items) → 2.2 (Movement) → 2.4 (Missing/Found)
  ↓
Phase 5 (History)
  ↓
Phase 6.1 (Doctor)
  ↓
Phase 8 (Output Formats)
```

**Parallel Development Possible:**
- Phase 4 (User Attribution) can develop alongside Phase 2
- Phase 8 (Output Formats) can be added incrementally
- Phase 3 (Projects) independent of Phase 2.3-2.5

---

## Testing Strategy

**Per Phase:**
- Unit tests for event handlers
- Unit tests for projection updates
- Integration tests for CLI commands
- Validation tests for business rules
- Edge case tests (cycles, duplicates, concurrent writes)

**System Tests:**
- End-to-end workflows (create → move → find)
- Doctor validation after operations
- Event replay determinism
- Concurrent operation safety

---

## Success Criteria

**MVP is complete when:**
1. ✓ User can create locations in hierarchy
2. ✓ User can add items at locations
3. ✓ User can move items (temporary and rehome)
4. ✓ User can mark items missing/found
5. ✓ User can query "where is X?"
6. ✓ User can see item history
7. ✓ `wherehouse doctor` validates consistency
8. ✓ All operations are atomic and safe
9. ✓ JSON output available for scripting
10. ✓ Test coverage >80% on critical paths

---

**Next Step:** Begin Phase 0 (Foundation) implementation with detailed plan
