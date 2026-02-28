# Wherehouse - AI Agent Instructions

## Project Overview

Wherehouse is an **event-sourced** CLI/TUI inventory tracker that answers "Where did I put my 10mm socket?". Built with Go + SQLite, it uses events as source of truth with disposable projections for fast queries. Multi-user attribution only (no permissions).

**Architecture**: Event log → Projections → CLI → TUI

**Implementation Status**: Database foundation complete (see Implementation Status section below)

---

## ⚠️ Critical First Steps

**Before ANY implementation work:**

1. **Read** `.claude/knowledge/business-rules.md` "Critical Invariants" section
2. **Understand** this is event-sourced (NOT CRUD - events are immutable)
3. **Check** `.claude/knowledge/README.md` to find the right document for your task

**Absolute rules (never violate):**
- Events are immutable (never modify, only append)
- Ordering by `event_id` only (never timestamps)
- No silent repair (fail explicitly on validation errors)
- No auto-creation of locations
- No colons (`:`) in item/project names (reserved for selectors)

---

## Documentation Map

**Start here:** `.claude/knowledge/README.md` - Which file for which task

### By Task Type

**Implementing features:**
1. `.claude/knowledge/domain-model.md` - Entities, relationships, selectors
2. `.claude/knowledge/events.md` - Event schemas and handlers
3. `.claude/knowledge/projections.md` - Projection updates
4. `.claude/knowledge/business-rules.md` - Validation requirements

**Understanding design:**
- `.claude/knowledge/architecture.md` - Why event-sourcing, trade-offs, philosophy
- `.claude/knowledge/event-sourcing-libraries.md` - Library evaluation (custom vs frameworks)
- `docs/DESIGN.md` - **Authoritative source** (full specification)

**CLI work:**
- `.claude/knowledge/cli-contract.md` - Commands, flags, output formats

**Validation/debugging:**
- `.claude/knowledge/business-rules.md` - All constraints and invariants

### File Sizes (Context Planning)
- Total knowledge base: ~25K tokens (includes event-sourcing library research)
- Typical task needs: 2-3 files (~8K tokens)
- Load selectively based on task

---

## Workflow Protocol

### Feature Development

```
1. Check docs/DESIGN.md for existing design
2. Load relevant .claude/knowledge/ files
3. Identify which events needed
4. Implement: validation → event creation → projection update
5. Verify against business-rules.md constraints
```

### Bug Fixes

```
1. Load .claude/knowledge/business-rules.md for correct behavior
2. Load specific file (events.md or projections.md)
3. Fix validation/logic
4. Verify no invariants broken
```

### Database Schema Work

```
1. Load .claude/knowledge/projections.md for table schemas
2. Load .claude/knowledge/events.md for event storage
3. Implement with indexes from business-rules.md
4. Ensure foreign keys enabled (PRAGMA foreign_keys=ON)
```

---

## Project Structure

```
wherehouse/
├── cmd/                    # CLI commands (cobra)
├── internal/
│   ├── models/            # Domain entities
│   ├── events/            # Event types and handlers
│   ├── projections/       # Projection builders
│   ├── database/          # SQLite access
│   ├── validation/        # Business rule enforcement
│   └── cli/               # CLI command implementations
├── docs/
│   └── DESIGN.md          # Authoritative design spec
├── .claude/
│   └── knowledge/         # AI agent reference docs
├── ai-docs/
│   └── sessions/          # Development orchestrator session artifacts
├── dist/                  # Build artifacts (gitignored)
└── main.go
```

---

## Implementation Status

### ✅ Completed (Session 20260221-213913)

**Database Foundation** - Production-ready, fully tested
- Migration framework (golang-migrate with embedded SQL)
- Event storage with 8 event type handlers
- Projection operations (locations, items, projects - 32 functions)
- Validation (from_location verification, cycle detection, uniqueness)
- Replay & rebuild (deterministic event replay)
- Testing infrastructure (100+ test cases, zero linter errors)
- XDG-compliant configuration paths

**Files**: `/internal/database/*`, `/internal/config/database.go`, migrations SQL

**Session Artifacts**: `ai-docs/sessions/20260221-213913/`

### ✅ Completed (Session 20260224-224828)

**Move Command** - Production-ready, fully tested
- Complete CLI implementation with selector resolution (UUID, LOCATION:ITEM, canonical)
- System location restrictions (cannot move FROM or TO Missing/Borrowed)
- Exact match requirement (ambiguous names fail with ID list)
- Fail-fast batch operations
- Event-sourcing validation (from_location verification)
- Output formats (human-readable, JSON, quiet modes)
- Project association handling (set/keep/clear)
- Temporary move support (--temp flag)
- 170+ tests passing, zero linter errors, 61.4% coverage

**Files**: `/cmd/move/*`, `/internal/database/location.go` (GetLocationByCanonicalName)

**Session Artifacts**: `ai-docs/sessions/20260224-224828/`

### ✅ Completed (Session 20260225-235216)

**Config Refactoring** - Production-ready, fully tested
- Viper-native config writes — no direct TOML editing (`WriteConfigAs` replaces custom serialization)
- `internal/config/writer.go`: `WriteDefault`, `Set`, `Check`, `GetValue` (all business logic here)
- `config unset` command removed — viper v1.21 has no key-delete API
- Flag binding: `--db`/`--as`/`--json`/`--quiet` applied to `*Config` struct post-load via `bindFlagsToConfig`
- 712 tests passing, zero linter errors

**Files**: `/internal/config/writer.go`, `/internal/config/writer_test.go`, `/cmd/config/*`, `/cmd/root.go`

**Session Artifacts**: `ai-docs/sessions/20260225-235216/`

### 🚧 Planned

**CLI Layer** (`/cmd/`, `/internal/cli/`)
- Additional command implementations (where, borrow, return, etc.)
- Shared output formatting utilities

**Business Rules** (`/internal/validation/`)
- Domain-specific validation logic
- Business constraint enforcement
- Cross-entity rule validation

**TUI Layer** (`/internal/tui/`)
- Interactive terminal interface
- Screen layouts and navigation
- Integration with CLI layer

**Domain Models** (`/internal/models/`)
- Entity types and interfaces
- Canonicalization utilities
- Selector parsing

**Event Handlers** (`/internal/events/`)
- Event creation helpers
- Event type definitions
- Payload validation

---

## Key Concepts (Quick Reference)

### Event Sourcing
- **Events** are source of truth (append-only log)
- **Projections** are derived state (rebuildable)
- **Replay** by `event_id` order (deterministic)
- **No undo** - use compensating events

### Entity Identifiers
- Items/Locations: UUID (v7 preferred)
- Projects: User-provided slug (no colons)
- Events: Integer `event_id` (autoincrement)

### Name Canonicalization
```
"10mm Socket Wrench" → "10mm_socket_wrench"
- Lowercase
- Trim whitespace
- Collapse runs to '_'
- Normalize separators to '_'
```

### Selector Syntax
```
LOCATION:ITEM           (both canonical names)
"Garage:10mm Socket"    → "garage:10mm_socket"
--id UUID               (exact ID reference)
```

### Special Locations
- `Missing` - Lost items (system location, `is_system=true`)
- `Borrowed` - Borrowed items (system location, `is_system=true`)
- Cannot be renamed, deleted, or reparented

### Validation Pattern
```go
// ALWAYS validate before creating event
1. Check entity exists
2. Validate from_location matches projection (critical!)
3. Check constraints (cycles, uniqueness, etc)
4. Create event + update projection (atomic transaction)
```

---

## Development Commands

```bash
# Database
wherehouse doctor              # Validate projection consistency
wherehouse doctor --rebuild    # Rebuild projections from events

# Build & Test (mise task automation)
mise run build                 # Build with version injection to dist/
mise run test                  # Run test suite with gotestsum and coverage
mise run lint                  # Run golangci-lint with HTML output
mise run ci                    # Full CI pipeline (test, lint, build, verify)
mise run dev                   # Development workflow (generate, mock, lint, test)
mise run snapshot              # goreleaser snapshot build

# Development
go test ./...                  # Run tests
go run main.go <cmd>          # Run locally
```

---

## Technology Stack

- **Language**: Go 1.21+
- **CLI**: spf13/cobra
- **Database**: SQLite (modernc.org/sqlite or mattn/go-sqlite3)
- **Styling**: charmbracelet/lipgloss (terminal styling), charmbracelet/fang (help styling)
- **Config**: spf13/viper
- **UUID**: google/uuid (v7)

**SQLite Configuration:**
```sql
PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;
PRAGMA synchronous=NORMAL;
PRAGMA busy_timeout=30000;
```

**Build Output:**
- Binaries: `dist/` directory (gitignored)
- Version injection via ldflags (handled by mise tasks)

---

## Development Orchestration

**Using the `/dev` skill:**
- Coordinates multiple specialist agents (architect, developer, ui-developer, tester, reviewer)
- Automatically routes work based on file paths:
  - `/cmd/`, `/internal/tui` → golang-ui-developer
  - `/internal/database/` → db-developer
  - `/internal/events/`, `/internal/projections/`, `/pkg/` → golang-developer
  - Testing → golang-tester
  - Reviews → code-reviewer
- Executes independent tasks in parallel for faster development
- Creates session artifacts in `ai-docs/sessions/{timestamp}/` for audit trail

**Completed Sessions:**
- **20260221-213913**: Database foundation (migrations, events, projections, validation, replay)
  - 4 batches, 100+ tests, zero linter errors
  - See `ai-docs/sessions/20260221-213913/SESSION-COMPLETE.md` for details
- **20260224-224828**: Move command implementation (CLI, selectors, validation, testing)
  - 4 batches (database, CLI, testing, review), 2 review iterations
  - 170+ tests passing, zero linter errors, 61.4% coverage
  - See `ai-docs/sessions/20260224-224828/` for planning, implementation, and review artifacts
- **20260225-092110**: CLI code refactoring (eliminate duplication in /cmd/)
  - 3 batches (3 parallel, 2 sequential), 5 phases, 1 code review
  - Consolidated 11 functions from 5 commands into /internal/cli/
  - Eliminated ~300 lines of duplicate code
  - 372 tests passing, zero linter errors, 94-100% coverage on new files
  - See `ai-docs/sessions/20260225-092110/` for refactoring plan and results
- **20260225-235216**: Config refactoring (viper-native writes, remove direct TOML editing)
  - 4 batches (9 tasks), 2 review iterations, 1 test run
  - Created `internal/config/writer.go`; thinned all `cmd/config/` commands; removed `config unset`
  - 712 tests passing, zero linter errors
  - See `ai-docs/sessions/20260225-235216/` for planning, implementation, and review artifacts

**Architecture Decisions:**
- **Event-sourcing library**: Custom implementation chosen over libraries (see `.claude/knowledge/event-sourcing-libraries.md`)
  - Rationale: Simplicity, SQLite alignment, transparency, control (~300 lines vs framework overhead)
  - Libraries evaluated: Event Horizon, hallgren/eventsourcing, goes, thefabric-io, eventhus, quintans

---

## Configuration Architecture (Quick Reference)

- `internal/config/writer.go` — all config write ops; use `WriteDefault`/`Set`/`Check`/`GetValue`
- `cmd/config/helpers.go` — only `cmdFS`, `SetFilesystem`, `fileExists`, `ensureDir` remain
- Config context key is typed (`configKeyType` in `cmd/root.go`) — use `config.ConfigKey`, not string `"config"`
- `bindFlagsToConfig` in `cmd/root.go` applies persistent flag overrides after config load
- viper v1.21 has no key-delete API — `config unset` is intentionally absent

## Linting Gotchas

- `cobra.ExactArgs(N)` triggers `mnd` — suppress with `//nolint:mnd // N is the exact arg count`
- `if err :=` inside a scope with existing `err` triggers `govet shadow` — use `err =` to reuse
- `mise run lint` is authoritative (runs mnd, govet shadow, and all configured linters)
- LSP/system-reminder diagnostics are often stale during parallel agent execution — verify with `go build ./...`

## Database Gotchas

- `MaxOpenConns(1)`: any loop over `*sql.Rows` that calls another DB query will deadlock — collapse nested lookups into SQL using subquery JOINs
- `json_extract(payload, '$.field')` works directly on event payloads in SQL — avoids Go-level JSON unmarshaling loops
- Test pattern for event data: insert all events first, then `ProcessEvent` in order; track `processedCount` to process only new events after setup

---

## Anti-Patterns (Never Do This)

❌ Modify events after creation
❌ Use timestamps for ordering
❌ Auto-repair projections on validation failure
❌ Auto-create locations
❌ Allow colons in item/project names
❌ Skip validation before event creation
❌ Implement domain logic in TUI (use CLI layer)

---

## When in Doubt

1. Check `docs/DESIGN.md` (authoritative)
2. Review `.claude/knowledge/business-rules.md` invariants
3. Ask user for clarification on ambiguous requirements
4. Prefer explicit over implicit (design philosophy)

---

**Version**: 1.5
**Last Updated**: 2026-02-26
**Authoritative Source**: docs/DESIGN.md
**Change Log**:
- v1.5: Config refactoring - viper-native writes, linting gotchas, config architecture (session 20260225-235216)
- v1.4: CLI code refactoring - eliminated duplication in /cmd/ (session 20260225-092110)
- v1.3: Added move command implementation (session 20260224-224828)
- v1.2: Added implementation status tracking, library research reference, session history
- v1.1: Initial version with development orchestration
