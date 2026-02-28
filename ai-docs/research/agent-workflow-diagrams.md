# Wherehouse Agent System - Complete Workflow Diagrams

**Date**: 2026-02-19
**Version**: 1.0

This document visualizes the complete agent system and common development workflows.

---

## Agent Ecosystem Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        WHEREHOUSE AGENT SYSTEM                          │
│                         6 Specialized Agents                            │
└─────────────────────────────────────────────────────────────────────────┘

                    ┌──────────────────────────┐
                    │  🔵 golang-architect     │
                    │  Model: opus             │
                    │  Color: blue             │
                    │  Role: Plans architecture│
                    │  Scope: All Go code      │
                    └────────────┬─────────────┘
                                 │
                    Designs architecture for ↓
                                 │
        ┌────────────────────────┼────────────────────┬─────────────┐
        ▼                        ▼                    ▼             ▼
┌───────────────┐      ┌───────────────┐    ┌───────────────┐  ┌──────────────┐
│🟡 golang-     │      │🟢 db-         │    │🔵 golang-ui-  │  │🟣 golang-    │
│   developer   │      │   developer   │    │   developer   │  │   tester     │
│Model: opus    │      │Model: opus    │    │Model: haiku   │  │Model: opus   │
│Color: yellow  │      │Color: green   │    │Color: cyan    │  │Color: magenta│
│               │      │               │    │               │  │              │
│Core Logic     │      │Database Code  │    │CLI/TUI Code   │  │All Testing   │
│/pkg/          │      │/internal/     │    │/cmd/          │  │*_test.go     │
│/internal/     │◄────►│ database/     │    │/internal/tui/ │  │              │
│ events/       │ Calls│               │    │               │  │Writes tests  │
│ projections/  │  DB  │• Queries      │    │• Commands     │  │Runs tests    │
│ models/       │ funcs│• Migrations   │    │• Flags        │  │Runs linting  │
│ validation/   │      │• Connections  │    │• Output fmt   │  │              │
│               │      │               │    │               │  │              │
│Tests: Own     │      │Tests: Own     │    │Tests: Own     │  │Tests: All    │
│Lint: Own      │      │Lint: Own      │    │Lint: Own      │  │Lint: All     │
└───────┬───────┘      └───────┬───────┘    └───────┬───────┘  └──────┬───────┘
        │                      │                    │                 │
        └──────────────────────┴────────────────────┴─────────────────┘
                                       │
                            All code reviewed by ↓
                                       │
                          ┌────────────┴─────────────┐
                          │  🔴 code-reviewer        │
                          │  Model: opus             │
                          │  Color: red              │
                          │  Role: Reviews all code  │
                          │  Scope: All Go code      │
                          │                          │
                          │  • Event-sourcing check  │
                          │  • Security analysis     │
                          │  • Performance review    │
                          │  • Code quality assess   │
                          │                          │
                          │  Identifies issues ──────┼──> Back to
                          └──────────────────────────┘    implementation
                                                          agents for fixes
```

---

## Complete Feature Development Workflow

### Scenario: "Implement item.moved event handler with TDD"

```
┌─────────────────────────────────────────────────────────────────────────┐
│ START: User Request                                                     │
│ "Implement item.moved event handler to track item movements"            │
└─────────────────────────────────────────────────────────────────────────┘
                                    ↓

┌─────────────────────────────────────────────────────────────────────────┐
│ DECISION: Is architecture needed?                                       │
│ • New feature? → Yes, need architecture                                 │
│ • Modifying existing? → Maybe, depends on scope                         │
│ • Bug fix? → No, skip to implementation                                 │
└─────────────────────────────────────────────────────────────────────────┘
                                    ↓ YES

┌═════════════════════════════════════════════════════════════════════════┐
║ PHASE 1: ARCHITECTURE DESIGN                                            ║
║ Agent: 🔵 golang-architect (opus, blue)                                 ║
╠═════════════════════════════════════════════════════════════════════════╣
║ Tasks:                                                                  ║
║ 1. Read DESIGN.md and knowledge docs                                    ║
║ 2. Design package structure                                             ║
║    • Event handler in /internal/events/                                 ║
║    • Validation in /internal/validation/                                ║
║ 3. Define interfaces                                                    ║
║    • MoveItem(itemID, fromLoc, toLoc, opts) error                       ║
║ 4. Plan event flow                                                      ║
║    • Validate from_location                                             ║
║    • Create item.moved event                                            ║
║    • Update projection (atomic transaction)                             ║
║ 5. Identify dependencies                                                ║
║    • Needs db-developer for query functions                             ║
║    • Needs golang-developer for event handler                           ║
║ 6. Write architecture plan to file                                      ║
║                                                                         ║
║ Output:                                                                 ║
║ • File: ai-docs/sessions/20260219-143000/01-planning/architecture.md   ║
║ • Summary: "Architecture plan complete. 3-phase approach. Details: ..." ║
╠═════════════════════════════════════════════════════════════════════════╣
║ User Reviews: Approves architecture plan                                ║
╚═════════════════════════════════════════════════════════════════════════╝
                                    ↓

┌═════════════════════════════════════════════════════════════════════════┐
║ PHASE 2: TDD - WRITE TESTS FIRST                                        ║
║ Agent: 🟣 golang-tester (opus, magenta)                                 ║
╠═════════════════════════════════════════════════════════════════════════╣
║ Tasks:                                                                  ║
║ 1. Read architecture plan                                               ║
║ 2. Write comprehensive tests BEFORE implementation                      ║
║                                                                         ║
║    File: internal/events/item_moved_test.go                             ║
║    Tests:                                                               ║
║    • TestItemMove_Success_UpdatesLocation                               ║
║    • TestItemMove_LocationMismatch_ReturnsError                         ║
║    • TestItemMove_LocationNotFound_ReturnsError                         ║
║    • TestItemMove_TemporaryUse_PreservesOrigin                          ║
║    • TestItemMove_Rehome_ClearsTemporaryState                           ║
║    • TestItemMove_WithProject_UpdatesProjectID                          ║
║    • TestItemMove_ClearProject_RemovesProjectID                         ║
║    • TestItemMove_TransactionRollback_OnError                           ║
║    • TestItemMove_CreatesEvent_BeforeProjection                         ║
║    • TestItemMove_AtomicTransaction_EventAndProjection                  ║
║                                                                         ║
║ 3. Run tests (expected: all fail, no implementation)                    ║
║    go test ./internal/events/ -v                                        ║
║    Result: 0/10 passing (EXPECTED - TDD RED phase)                      ║
║                                                                         ║
║ 4. Write test details to file                                           ║
║                                                                         ║
║ Output:                                                                 ║
║ • File: ai-docs/sessions/20260219-143000/02-testing/tests-written.md   ║
║ • Summary: "10 tests written, all failing (expected). Ready for impl." ║
╠═════════════════════════════════════════════════════════════════════════╣
║ User: Proceeds to implementation phase                                  ║
╚═════════════════════════════════════════════════════════════════════════╝
                                    ↓
                        ┌───────────┴───────────┐
                        ▼                       ▼
┌═══════════════════════════════════┐  ┌═══════════════════════════════════┐
║ PHASE 3A: DATABASE IMPLEMENTATION ║  ║ PHASE 3B: CORE IMPLEMENTATION     ║
║ Agent: 🟢 db-developer (opus)     ║  ║ Agent: 🟡 golang-developer (opus) ║
║                                   ║  ║                                   ║
║ (Can run in PARALLEL)             ║  ║ (Can run in PARALLEL)             ║
╠═══════════════════════════════════╣  ╠═══════════════════════════════════╣
║ Tasks:                            ║  ║ Tasks:                            ║
║ 1. Implement query functions      ║  ║ 1. Read architecture + tests      ║
║    in /internal/database/         ║  ║ 2. Implement event handler        ║
║                                   ║  ║    File: internal/events/         ║
║    • GetItemByID(id) (*Item, err) ║  ║          item_moved.go            ║
║    • GetLocationByID(id) (...)    ║  ║                                   ║
║    • UpdateItemLocation(...)      ║  ║    func MoveItem(...) error {     ║
║    • CreateItemMovedEvent(...)    ║  ║      // 1. Get item               ║
║                                   ║  ║      item := db.GetItemByID(id)   ║
║ 2. Write migration if needed      ║  ║                                   ║
║                                   ║  ║      // 2. Validate from_location ║
║ 3. Write database tests           ║  ║      if item.LocationID !=        ║
║    • TestGetItemByID_Exists       ║  ║         fromLocationID {          ║
║    • TestUpdateItemLocation_OK    ║  ║        return ErrLocationMismatch ║
║    • TestTransaction_Rollback     ║  ║      }                            ║
║                                   ║  ║                                   ║
║ 4. Run tests + linting            ║  ║      // 3. Validate target exists ║
║    go test ./internal/database/   ║  ║      if !db.LocationExists(toLoc) ║
║    golangci-lint run              ║  ║        return ErrLocationNotFound ║
║                                   ║  ║      }                            ║
║ Output:                           ║  ║                                   ║
║ • Tests: 15/15 passing            ║  ║      // 4. Create event + update  ║
║ • Linting: Clean                  ║  ║      //    (atomic transaction)   ║
║ • File: internal/database/        ║  ║      tx := db.Begin()             ║
║         items.go                  ║  ║      defer tx.Rollback()          ║
║ • Summary: "DB implementation     ║  ║      event := CreateEvent(...)    ║
║   complete. Tests passing."       ║  ║      tx.InsertEvent(event)        ║
╚═══════════════════════════════════╝  ║      tx.UpdateProjection(...)     ║
                                       ║      return tx.Commit()            ║
                                       ║    }                               ║
                                       ║                                   ║
                                       ║ 3. Write unit tests                ║
                                       ║ 4. Run tests + linting             ║
                                       ║    go test ./internal/events/      ║
                                       ║    golangci-lint run               ║
                                       ║                                   ║
                                       ║ Output:                            ║
                                       ║ • Tests: 25/25 passing             ║
                                       ║   (10 TDD + 15 unit)               ║
                                       ║ • Linting: Clean                   ║
                                       ║ • Summary: "Event handler complete ║
                                       ║   Tests: 25/25 | Linting: Clean"   ║
                                       ╚═══════════════════════════════════╝
                        └───────────┬───────────┘
                                    ↓

┌═════════════════════════════════════════════════════════════════════════┐
║ PHASE 4: COMPREHENSIVE VERIFICATION                                     ║
║ Agent: 🟣 golang-tester (opus, magenta)                                 ║
╠═════════════════════════════════════════════════════════════════════════╣
║ Tasks:                                                                  ║
║ 1. Run ENTIRE test suite (not just new tests)                           ║
║    go test ./... -v -race -coverprofile=coverage.out                    ║
║                                                                         ║
║    Result:                                                              ║
║    • internal/events: 25/25 passing ✅                                  ║
║    • internal/database: 15/15 passing ✅                                ║
║    • internal/validation: 20/20 passing ✅                              ║
║    • cmd/wherehouse: 30/30 passing ✅                                   ║
║    • pkg/: 10/10 passing ✅                                             ║
║    TOTAL: 100/100 passing ✅                                            ║
║                                                                         ║
║ 2. Check for race conditions                                            ║
║    -race flag: No races detected ✅                                     ║
║                                                                         ║
║ 3. Analyze coverage                                                     ║
║    go tool cover -func=coverage.out                                     ║
║    internal/events/item_moved.go: 95.2% ✅                              ║
║                                                                         ║
║ 4. Run linting on ENTIRE codebase                                       ║
║    golangci-lint run                                                    ║
║    Result: No errors ✅                                                 ║
║                                                                         ║
║ 5. Write verification report to file                                    ║
║                                                                         ║
║ Output:                                                                 ║
║ • File: ai-docs/sessions/20260219-143000/03-verification/              ║
║         test-results.md                                                 ║
║ • Summary: "100/100 tests passing. Linting clean. Verified."           ║
╠═════════════════════════════════════════════════════════════════════════╣
║ User: Proceeds to code review                                           ║
╚═════════════════════════════════════════════════════════════════════════╝
                                    ↓

┌═════════════════════════════════════════════════════════════════════════┐
║ PHASE 5: CODE REVIEW                                                    ║
║ Agent: 🔴 code-reviewer (opus, red)                                     ║
╠═════════════════════════════════════════════════════════════════════════╣
║ Tasks:                                                                  ║
║ 1. Read implemented code                                                ║
║    • internal/events/item_moved.go                                      ║
║    • internal/database/items.go                                         ║
║    • Tests for both                                                     ║
║                                                                         ║
║ 2. Check Event-Sourcing Correctness                                     ║
║    ✅ from_location validated before event creation                     ║
║    ✅ Event created before projection update                            ║
║    ✅ Atomic transaction (event + projection)                           ║
║    ✅ No modification of events after creation                          ║
║    ✅ Proper error handling (no silent repair)                          ║
║                                                                         ║
║ 3. Check Business Rules                                                 ║
║    ✅ System location checks (can't move to/from Missing incorrectly)   ║
║    ✅ Temporary use semantics preserved                                 ║
║    ✅ Project association handled correctly                             ║
║                                                                         ║
║ 4. Check Security                                                       ║
║    ✅ All queries use prepared statements                               ║
║    ✅ No user input in SQL strings                                      ║
║    ✅ Error messages don't leak sensitive data                          ║
║                                                                         ║
║ 5. Check Performance                                                    ║
║    ✅ Indexes used (location_id, item_id)                               ║
║    ✅ No N+1 query patterns                                             ║
║    ✅ Transaction properly scoped                                       ║
║                                                                         ║
║ 6. Check Code Quality                                                   ║
║    ✅ Functions under 50 lines                                          ║
║    ✅ Clear naming (MoveItem, not ProcessItemChange)                    ║
║    ✅ Proper error wrapping with context                                ║
║    ✅ Good test coverage                                                ║
║                                                                         ║
║ 7. Write review to file                                                 ║
║                                                                         ║
║ Output:                                                                 ║
║ • File: ai-docs/sessions/20260219-143000/04-review/code-review.md      ║
║ • Summary: "Assessment: Ready to Merge. No critical issues. Risk: Low."║
╠═════════════════════════════════════════════════════════════════════════╣
║ Review Result: ✅ APPROVED                                              ║
╚═════════════════════════════════════════════════════════════════════════╝
                                    ↓

┌─────────────────────────────────────────────────────────────────────────┐
│ SUCCESS: Feature Complete                                               │
│ • Architecture: Designed ✅                                             │
│ • Tests: Written first (TDD) ✅                                         │
│ • Implementation: Complete ✅                                           │
│ • Verification: All tests pass + linting clean ✅                       │
│ • Code Review: Approved ✅                                              │
│                                                                         │
│ Ready to merge! 🎉                                                      │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Workflow with Review Feedback Loop

### Scenario: Code review finds critical issues

```
┌═════════════════════════════════════════════════════════════════════════┐
║ PHASE 5: CODE REVIEW                                                    ║
║ Agent: 🔴 code-reviewer (opus, red)                                     ║
╠═════════════════════════════════════════════════════════════════════════╣
║ Review finds issues:                                                    ║
║                                                                         ║
║ 🔴 CRITICAL:                                                            ║
║ 1. Missing from_location validation (line 45)                           ║
║    Code: moveItem(itemID, toLocationID)                                 ║
║    Issue: No check that item.LocationID matches expected fromLocation   ║
║    Risk: Can corrupt projection state                                   ║
║                                                                         ║
║ 🟡 HIGH:                                                                ║
║ 2. Transaction not atomic (lines 50-60)                                 ║
║    Code: Creates event, THEN updates projection (separate operations)   ║
║    Issue: If projection update fails, event exists but projection       ║
║           doesn't match                                                 ║
║    Risk: Event log and projection diverge                               ║
║                                                                         ║
║ Output:                                                                 ║
║ • Summary: "Assessment: Needs Changes. Critical: 1 | High: 1.          ║
║             Risk: High. Details: ..."                                   ║
╠═════════════════════════════════════════════════════════════════════════╣
║ Review Result: ❌ NEEDS CHANGES                                         ║
╚═════════════════════════════════════════════════════════════════════════╝
                                    ↓
                        Issues sent back to implementation
                                    ↓

┌═════════════════════════════════════════════════════════════════════════┐
║ PHASE 6: FIX ISSUES                                                     ║
║ Agent: 🟡 golang-developer (opus, yellow)                               ║
╠═════════════════════════════════════════════════════════════════════════╣
║ Tasks:                                                                  ║
║ 1. Read review feedback                                                 ║
║    • Critical issue #1: Add from_location validation                    ║
║    • High issue #2: Make transaction atomic                             ║
║                                                                         ║
║ 2. Fix critical issue #1                                                ║
║    Before:                                                              ║
║      func MoveItem(itemID, toLocationID string) error {                 ║
║        item, _ := db.GetItemByID(itemID)                                ║
║        // Missing validation!                                           ║
║        return db.UpdateItemLocation(itemID, toLocationID)               ║
║      }                                                                  ║
║                                                                         ║
║    After:                                                               ║
║      func MoveItem(itemID, fromLocationID, toLocationID string) error { ║
║        item, err := db.GetItemByID(itemID)                              ║
║        if err != nil {                                                  ║
║          return err                                                     ║
║        }                                                                ║
║        // ADD: Validation                                               ║
║        if item.LocationID != fromLocationID {                           ║
║          return fmt.Errorf("location mismatch: "+                       ║
║            "projection has %s, event expects %s",                       ║
║            item.LocationID, fromLocationID)                             ║
║        }                                                                ║
║        // ... rest of function                                          ║
║      }                                                                  ║
║                                                                         ║
║ 3. Fix high issue #2                                                    ║
║    Before:                                                              ║
║      event := createEvent(...)                                          ║
║      db.InsertEvent(event)       // Operation 1                         ║
║      db.UpdateProjection(event)  // Operation 2 (separate!)            ║
║                                                                         ║
║    After:                                                               ║
║      tx, err := db.Begin()                                              ║
║      if err != nil {                                                    ║
║        return err                                                       ║
║      }                                                                  ║
║      defer tx.Rollback()  // Safe cleanup                               ║
║                                                                         ║
║      event := createEvent(...)                                          ║
║      if err := tx.InsertEvent(event); err != nil {                      ║
║        return err  // Rollback happens                                  ║
║      }                                                                  ║
║      if err := tx.UpdateProjection(event); err != nil {                 ║
║        return err  // Rollback happens                                  ║
║      }                                                                  ║
║      return tx.Commit()  // Atomic!                                     ║
║                                                                         ║
║ 4. Run tests + linting                                                  ║
║    go test ./internal/events/ -v                                        ║
║    golangci-lint run                                                    ║
║                                                                         ║
║    Result: All tests pass ✅, Linting clean ✅                          ║
║                                                                         ║
║ Output:                                                                 ║
║ • Summary: "Fixes complete. Tests: 25/25 | Linting: Clean"             ║
╚═════════════════════════════════════════════════════════════════════════╝
                                    ↓

┌═════════════════════════════════════════════════════════════════════════┐
║ PHASE 7: RE-VERIFICATION                                                ║
║ Agent: 🟣 golang-tester (opus, magenta)                                 ║
╠═════════════════════════════════════════════════════════════════════════╣
║ Tasks:                                                                  ║
║ 1. Run full test suite again                                            ║
║    go test ./... -v -race                                               ║
║    Result: 100/100 passing ✅                                           ║
║                                                                         ║
║ 2. Run linting                                                          ║
║    golangci-lint run                                                    ║
║    Result: Clean ✅                                                     ║
║                                                                         ║
║ Output:                                                                 ║
║ • Summary: "Tests passing. Linting clean. Ready for re-review."        ║
╚═════════════════════════════════════════════════════════════════════════╝
                                    ↓

┌═════════════════════════════════════════════════════════════════════════┐
║ PHASE 8: RE-REVIEW                                                      ║
║ Agent: 🔴 code-reviewer (opus, red)                                     ║
╠═════════════════════════════════════════════════════════════════════════╣
║ Tasks:                                                                  ║
║ 1. Review fixes                                                         ║
║    ✅ Issue #1 resolved: from_location validation added                 ║
║    ✅ Issue #2 resolved: Transaction is atomic                          ║
║                                                                         ║
║ 2. Check no new issues introduced                                       ║
║    ✅ No new problems found                                             ║
║                                                                         ║
║ Output:                                                                 ║
║ • Summary: "Assessment: Ready to Merge. Issues resolved. Risk: Low."   ║
╠═════════════════════════════════════════════════════════════════════════╣
║ Review Result: ✅ APPROVED                                              ║
╚═════════════════════════════════════════════════════════════════════════╝
                                    ↓

┌─────────────────────────────────────────────────────────────────────────┐
│ SUCCESS: Feature Complete (after fixes)                                 │
│ Ready to merge! 🎉                                                      │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Parallel Implementation Workflow

### Multiple agents working simultaneously

```
┌─────────────────────────────────────────────────────────────────────────┐
│ Scenario: Implement complete feature with database, core logic, and CLI│
│ "Add 'wherehouse borrow' command to track borrowed items"               │
└─────────────────────────────────────────────────────────────────────────┘
                                    ↓
┌═════════════════════════════════════════════════════════════════════════┐
║ PHASE 1: Architecture + TDD                                             ║
║ Sequential (must complete before parallel work)                         ║
╠═════════════════════════════════════════════════════════════════════════╣
║ 1. golang-architect: Design system                                      ║
║ 2. golang-tester: Write tests first (TDD)                               ║
╚═════════════════════════════════════════════════════════════════════════╝
                                    ↓
                    ┌───────────────┴───────────────┐
                    ▼                               ▼
┌═══════════════════════════════════┐  ┌═══════════════════════════════════┐
║ PARALLEL IMPLEMENTATION - Part 1  ║  ║ PARALLEL IMPLEMENTATION - Part 2  ║
╠═══════════════════════════════════╣  ╠═══════════════════════════════════╣
║ 🟢 db-developer                   ║  ║ 🟡 golang-developer               ║
║ Database work                     ║  ║ Core logic                        ║
║                                   ║  ║                                   ║
║ Implements:                       ║  ║ Implements:                       ║
║ • item.borrowed event storage     ║  ║ • Event handler logic             ║
║ • BorrowItem query function       ║  ║ • Validation (item exists,        ║
║ • Update projection SQL           ║  ║   borrower name not empty)        ║
║ • Move item to "Borrowed" loc     ║  ║ • Business rule enforcement       ║
║                                   ║  ║                                   ║
║ Duration: ~30 minutes             ║  ║ Duration: ~30 minutes             ║
║                                   ║  ║ (Runs at SAME TIME as db work)    ║
║ Tests: 12/12 passing ✅           ║  ║ Tests: 18/18 passing ✅           ║
║ Linting: Clean ✅                 ║  ║ Linting: Clean ✅                 ║
╚═══════════════════════════════════╝  ╚═══════════════════════════════════╝
                    └───────────────┬───────────────┘
                                    ↓
                    Both complete simultaneously
                                    ↓
┌═════════════════════════════════════════════════════════════════════════┐
║ PARALLEL IMPLEMENTATION - Part 3                                        ║
║ (Depends on Parts 1 & 2 completing)                                     ║
╠═════════════════════════════════════════════════════════════════════════╣
║ 🔵 golang-ui-developer                                                  ║
║ CLI command                                                             ║
║                                                                         ║
║ Implements:                                                             ║
║ • 'wherehouse borrow' cobra command                                     ║
║ • Flag parsing (--borrowed-by, --note)                                  ║
║ • Call golang-developer's BorrowItem()                                  ║
║ • Call db-developer's query functions                                   ║
║ • Output formatting (human/JSON)                                        ║
║                                                                         ║
║ Duration: ~20 minutes                                                   ║
║                                                                         ║
║ Tests: 8/8 passing ✅                                                   ║
║ Linting: Clean ✅                                                       ║
╚═════════════════════════════════════════════════════════════════════════╝
                                    ↓
┌═════════════════════════════════════════════════════════════════════════┐
║ VERIFICATION + REVIEW                                                   ║
║ Sequential (full codebase check)                                        ║
╠═════════════════════════════════════════════════════════════════════════╣
║ 1. golang-tester: Full test suite + linting                             ║
║    Result: 150/150 tests passing ✅                                     ║
║                                                                         ║
║ 2. code-reviewer: Comprehensive review                                  ║
║    Result: Ready to Merge ✅                                            ║
╚═════════════════════════════════════════════════════════════════════════╝
                                    ↓
                              ✅ SUCCESS
                    Total time saved: ~30 minutes
              (Would be ~80 mins sequential, only ~50 mins parallel)
```

---

## Decision Tree: Which Agent to Use?

```
User Request Received
        │
        ├─ Is this architecture/design work?
        │  ├─ YES → 🔵 golang-architect
        │  └─ NO → Continue
        │
        ├─ Is this implementing tests?
        │  ├─ YES → 🟣 golang-tester
        │  └─ NO → Continue
        │
        ├─ Is this reviewing existing code?
        │  ├─ YES → 🔴 code-reviewer
        │  └─ NO → Continue
        │
        ├─ Is this implementing code?
        │  ├─ YES → Which type?
        │  │   ├─ Database code (/internal/database/)
        │  │   │  └─ 🟢 db-developer
        │  │   │
        │  │   ├─ CLI/TUI code (/cmd/, /internal/tui/)
        │  │   │  └─ 🔵 golang-ui-developer
        │  │   │
        │  │   └─ Core logic (/pkg/, /internal/events/, etc.)
        │  │      └─ 🟡 golang-developer
        │  │
        │  └─ NO → Ask user for clarification
        │
        └─ Is this verification (tests + linting)?
           ├─ YES → 🟣 golang-tester
           └─ NO → Ask user for clarification
```

---

## Agent Communication Patterns

### Pattern 1: Sequential Delegation

```
Main Chat
    ↓ "Implement item.moved"
    │
    ├─> golang-architect: Design architecture
    │   Returns: "Architecture complete. Details: file.md"
    │
    ├─> golang-tester: Write tests first (TDD)
    │   Returns: "Tests written, failing. Ready for impl."
    │
    ├─> golang-developer: Implement code
    │   Returns: "Implementation complete. Tests: 25/25"
    │
    ├─> golang-tester: Verify all tests
    │   Returns: "100/100 passing. Linting clean."
    │
    └─> code-reviewer: Review code
        Returns: "Ready to merge. No critical issues."
```

### Pattern 2: Parallel Delegation

```
Main Chat
    ↓ "Implement borrow feature"
    │
    ├─> golang-architect: Design (sequential first)
    │   Returns: "Architecture complete"
    │
    ├─> golang-tester: Write tests (sequential second)
    │   Returns: "Tests written"
    │
    ├─────────────┬─────────────┐
    │             │             │
    ▼             ▼             ▼
db-developer  golang-dev  (waits for db + core)
(parallel)    (parallel)
    │             │
    └──────┬──────┘
           ▼
    golang-ui-developer
    (sequential after dependencies)
           │
           ├─> golang-tester: Verify
           │   Returns: "All tests pass"
           │
           └─> code-reviewer: Review
               Returns: "Ready to merge"
```

### Pattern 3: Review Feedback Loop

```
code-reviewer
    ↓ Finds issues
    │
    ├─> Critical issues → golang-developer (fix core logic)
    │   ├─> Runs own tests + linting
    │   └─> Returns: "Fixed"
    │
    ├─> Database issues → db-developer (fix queries)
    │   ├─> Runs own tests + linting
    │   └─> Returns: "Fixed"
    │
    ├─> CLI issues → golang-ui-developer (fix commands)
    │   ├─> Runs own tests + linting
    │   └─> Returns: "Fixed"
    │
    ├─> All fixes complete
    │   ↓
    │   golang-tester: Re-verify everything
    │   ↓
    │   code-reviewer: Re-review
    │   ↓
    │   ✅ APPROVED
```

---

## Time Savings: Parallel vs Sequential

### Example: Complete Feature Implementation

**Sequential (traditional) approach:**
```
1. Architecture:       30 minutes
2. Write tests:        20 minutes
3. DB implementation:  30 minutes
4. Core impl:          30 minutes (waits for DB)
5. CLI impl:           20 minutes (waits for core)
6. Verify:             10 minutes
7. Review:             15 minutes
─────────────────────────────────
Total:                155 minutes (2h 35m)
```

**Parallel (agent-based) approach:**
```
1. Architecture:       30 minutes
2. Write tests:        20 minutes
3. Implementation:     30 minutes (DB + Core in parallel!)
4. CLI impl:           20 minutes
5. Verify:             10 minutes
6. Review:             15 minutes
─────────────────────────────────
Total:                125 minutes (2h 5m)

Time saved: 30 minutes (19% faster)
```

---

## Summary

The wherehouse agent system provides:

1. **Specialized Expertise**: Each agent focuses on what it does best
2. **Clear Boundaries**: No overlap, no confusion about responsibility
3. **Parallel Execution**: Independent tasks run simultaneously
4. **Quality Gates**: Verification and review before merge
5. **Feedback Loops**: Issues caught early, fixed quickly
6. **Consistency**: All code meets same standards (tests + linting)

**Result**: Faster development with higher quality code.
