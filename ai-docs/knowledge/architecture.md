# Architecture & Design Philosophy

**Source**: docs/DESIGN.md
**Purpose**: Key decisions, trade-offs, and architectural principles

---

## Core Philosophy

### Wherehouse Is

```
✓ Event-sourced        Events are source of truth
✓ Deterministic        Replay order by event_id, not timestamps
✓ Multi-user           Attribution only, no permissions
✓ SQLite-backed        Single file, network-mount compatible
✓ Projection-driven    Fast reads from derived state
✓ Strict               Strong invariants over convenience
✓ Explicit             No magic, no silent behavior
✓ CLI-first            TUI wraps CLI, no duplicated logic
```

### Wherehouse Favors

```
✓ Determinism over magic
✓ Explicit commands over silent behavior
✓ Strong invariants over convenience
✓ Human-readable defaults with machine-readable output options
```

### Wherehouse Is NOT

```
✗ A distributed system
✗ A permissions system
✗ A cloud service
✗ Trying to be everything
```

**Goal**: Be correct, be simple, be explicit.

---

## Event Sourcing Architecture

### Why Event Sourcing?

**Benefits**:
- Complete audit trail (who, what, when)
- Projections rebuildable from events
- Multiple read models possible
- No lost context (notes, reasons preserved)
- Time-travel queries possible (future)
- Debugging via replay

**Trade-offs**:
- More complex than CRUD
- Cannot "undo" (compensating events only)
- Projection rebuild can be slow (acceptable for v1)
- Schema evolution requires migration strategy

### Event Authority

```
PRINCIPLE: event_id defines truth ordering
  - event_id = INTEGER AUTOINCREMENT
  - Timestamps are informational only
  - Clock skew doesn't affect correctness
  - Replay is deterministic
```

**Why?**:
- Network storage may have clock sync issues
- Multi-user scenarios can have time drift
- Event order must be unambiguous
- Integer sequence provides total ordering

### Projections Are Disposable

```
PRINCIPLE: Current state derived from events
  - Projections can be deleted and rebuilt
  - Rebuild must match incremental updates
  - Mismatch = corruption = failure
  - "doctor" command validates consistency
```

**Why?**:
- Simplifies schema evolution
- Debugging via rebuild
- Confidence in correctness
- Clear source of truth

---

## Domain Design Decisions

### UUIDs for Entities, Integers for Events

```
DECISION: Use UUIDs for item_id, location_id
REASON: Portability, no coordination required, future-proof

DECISION: Use INTEGER for event_id
REASON: Deterministic ordering, efficient, no gaps
```

### Global Canonical Location Names

```
DECISION: location.canonical_name globally unique
REASON: Simplifies matching, no ambiguous paths

TRADE-OFF: Slightly less filesystem-like
BENEFIT: No "which Toolbox did you mean?" confusion
```

**Example**:
```
Cannot have: Garage:Toolbox AND Basement:Toolbox
Must be: Garage:Toolbox1 AND Basement:Toolbox2
  OR: Garage:Red_Toolbox AND Basement:Green_Toolbox
```

### Projects as Ephemeral Context

```
DECISION: Default movement clears project association
REASON: Projects are temporary context, not permanent tags

PRINCIPLE: Explicit carry-forward required
FLAGS: --project, --keep-project, --clear-project
```

**Why?**:
- Reduces clutter (items don't accumulate old projects)
- Forces intentional project tracking
- Completed projects don't auto-disassociate items
- User controls when items "return" from projects

### Temporary Use Origin Tracking

```
DECISION: Track original location before temporary use
FIELD: temp_origin_location_id

BEHAVIOR:
  - First temporary_use sets origin
  - Subsequent temporary moves preserve original origin
  - Rehome clears temporary state
```

**Why?**:
- Item can move through multiple temporary locations
- Always know where it belongs
- "Return to origin" workflow natural

---

## No Silent Magic

### Explicit Over Implicit

```
✓ No auto-repair of projections
✓ No implicit retries on validation failure
✓ No implicit project carry-over
✓ No auto-creation of locations
✓ No auto-return of items on project completion
```

**Philosophy**: User should always understand what happened and why.

### Fail Loud

```
PRINCIPLE: Validation failures stop replay immediately
  - Report event_id and error
  - Do not skip events
  - Do not "best guess" repairs
  - Require manual intervention
```

**Why?**:
- Silent failures hide bugs
- Corruption should be obvious
- User trust requires transparency

---

## Strict Integrity

### Validation Before Persistence

```
PRINCIPLE: Events validated before writing
  - Invalid events rejected
  - Atomic: event write + projection update
  - Transaction rollback on failure
```

### Projection Consistency

```
PRINCIPLE: from_location_id stored and validated
  - item.moved stores from_location_id
  - Replay checks: projection.location_id = event.from_location_id
  - Mismatch = corruption or concurrent write

PURPOSE:
  - Detects projection drift
  - Validates event ordering
  - Prevents "move from wrong location"
```

### Per-Item Write Serialization

```
PRINCIPLE: Concurrent writes to same item serialized
  - SQLite row-level locking (WAL mode)
  - Retry with backoff on SQLITE_BUSY
  - Ensures event ordering integrity
```

---

## Technology Choices

### SQLite

**Why SQLite?**
```
✓ Serverless (no setup)
✓ Single file (easy backup/move)
✓ Network storage compatible (WAL mode)
✓ Excellent concurrency (WAL mode)
✓ Sufficient for personal use (10K+ items)
✓ Cross-platform
```

**Configuration**:
```sql
PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;  -- faster on network storage
PRAGMA foreign_keys=ON;
PRAGMA busy_timeout=30000;  -- 30s for network mounts
```

### Go

**Why Go?**
```
✓ Single binary distribution
✓ Cross-platform (Linux, macOS, Windows)
✓ Excellent CLI library ecosystem (cobra)
✓ Strong SQLite support
✓ Fast compilation
✓ Good concurrency primitives (if needed later)
```

### CLI-First Architecture

```
ARCHITECTURE:
  TUI → CLI Layer → Domain Engine → Events + Projections

PRINCIPLE: TUI is thin wrapper over CLI
  - No direct event writes from TUI
  - No duplicated domain logic
  - TUI invokes CLI commands
  - Ensures CLI completeness
```

**Why?**:
- Forces well-designed CLI
- TUI can't bypass rules
- Scriptable by default
- Testing easier

---

## Human-Centered Defaults

### History Presentation

```
DEFAULT: Semantic summaries with icons
  󰙴  Created in Basement:Toolbox
  󱁤  Moved to Kitchen (temporary use)
  ⁇  Marked missing (last: Kitchen)
  󰍉  Found in Garage (home: Basement:Toolbox)

--raw: Exact event types with full fields
```

**Why?**:
- Humans read history more than events
- Icons provide quick visual scanning
- Raw mode for debugging/export

### Verbosity Levels

```
DEFAULT: Minimal, immediate answer
  -v:  More context (full paths)
  -vv: Full detail (timestamps, actors)
  -q:  Quiet (suppress confirmations)
  -qq: Silent (exit code only)
```

**Why?**:
- Fast answers for daily use
- Detail available when needed
- Scriptable with -q/-qq

### UUID Hiding

```
DEFAULT: Show short ID (first 6 chars)
  "10mm socket (id: 8f3a2c)"

--id: Show full UUID
  "10mm socket (id: 8f3a2c1d-4e5f-6789-0abc-def123456789)"
```

**Why?**:
- UUIDs are implementation detail
- Short form sufficient for disambiguation
- Full form available for debugging

---

## Determinism Over Convenience

### No Fuzzy Parsing

```
DECISION: Exact canonical matching only
REASON: Deterministic, scriptable, predictable

ALLOWED: Fuzzy in completion layer (fzf)
FORBIDDEN: Fuzzy in command execution
```

**Why?**:
- Scripts must be reliable
- No "did you mean?" ambiguity
- Errors are explicit, not silent mismatches

### No Ambiguous Resolution

```
DECISION: Multiple matches return all
REASON: Force user to disambiguate explicitly

TOOL: Location-scoped selector (LOCATION:ITEM)
```

**Example**:
```bash
# Ambiguous (returns all "screwdriver" items)
wherehouse where screwdriver

# Explicit
wherehouse where garage:toolbox:screwdriver
```

---

## Removals Preserve History

### Item Removal

```
DECISION: Items are removed by moving to "Removed" system location
REASON: Preserves history; item may have been borrowed, moved, or lost

ALTERNATIVE: Move to Missing
  - For temporarily lost items
  - Can be marked found later
```

### Location Removal

```
DECISION: Can remove only empty locations
REASON: Prevents accidental data loss

VALIDATION:
  - No items in location
  - No sub-locations
```

### Projects Cannot Be Removed

```
DECISION: Projects cannot be removed
REASON: Projects may have historical item associations

USE: project.completed to close out a project
```

---

## Export / Import Strategy

### Export Format

```
STRUCTURE:
  - Event log (ordered by event_id)
  - Projection snapshot (optional)
  - Schema version
  - Config version
  - Checksums (event log integrity)
```

### Import Strategy (Future)

```
DEFAULT: Rebuild projections from events
  - Slow but guaranteed correct
  - Validates event log integrity

--fast: Trust projection snapshot
  - Skip rebuild
  - Faster import
  - Assumes export was valid
```

---

## Doctor Command Philosophy

### Validation Without Repair

```
COMMAND: wherehouse doctor

ACTIONS:
  1. Validate structural integrity (tree cycles, FKs)
  2. Validate event stream (no gaps, valid types)
  3. Rebuild projections (temp tables)
  4. Compare with current projections
  5. Report mismatches (exit non-zero)

NO ACTIONS:
  - No silent repair
  - No "best guess" fixes
  - No event modification
```

**Why?**:
- Transparency over convenience
- User should understand corruption
- Explicit repair decisions

### Rebuild Flag

```
COMMAND: wherehouse doctor --rebuild

ACTION: Destructively replace projections with rebuilt version
USE CASE: After confirming corruption diagnosis
```

---

## Network Storage Considerations

### Compatibility

```
SUPPORTED: NFS, SMB, network mounts
REQUIREMENTS:
  - WAL journal mode
  - Adequate busy_timeout
  - Synchronized clocks recommended (but not required for correctness)
```

### Performance

```
EXPECTATION: Slower than local storage
MITIGATION:
  - synchronous=NORMAL (vs FULL)
  - wal_autocheckpoint tuning
  - Connection pooling (future)
```

### Locking

```
MECHANISM: SQLite lock files (.db-shm, .db-wal)
BEHAVIOR: Concurrent reads, serialized writes per item
CAVEAT: Network filesystems may have locking quirks
```

---

## Configuration Philosophy

### Resolution Order

```
1. CLI flags (highest priority)
2. Environment variables (WHEREHOUSE_*)
3. --config file path
4. ./wherehouse.toml (current directory)
5. XDG config (~/.config/wherehouse/config.toml)
6. Defaults (lowest priority)
```

**Why?**:
- Standard Unix convention
- Scriptable via env vars
- Project-specific config (./wherehouse.toml)
- Per-user defaults (XDG)

### Required Config Version

```
REQUIREMENT: config_version = 1

PURPOSE:
  - Detect old config files
  - Prevent silent misconfigurations
  - Enable config migrations
```

---

## Future-Proofing

### What's Designed For

```
✓ Multiple read projections (different views)
✓ Time-travel queries (events have timestamps)
✓ Export/import (event portability)
✓ Schema evolution (rebuild projections)
✓ Analytics (event log analysis)
```

### What's NOT Designed For

```
✗ Millions of items (SQLite limits)
✗ Distributed/multi-site (single DB file)
✗ Real-time collaboration (eventual consistency)
✗ Complex permissions (attribution only)
```

---

## Key Trade-offs Summary

| Decision | Trade-off | Rationale |
|----------|-----------|-----------|
| Event sourcing | Complexity vs auditability | Audit trail worth complexity |
| Integer event_id | Flexibility vs determinism | Determinism essential |
| Global canonical names | Filesystem-like vs unambiguous | Simplicity wins |
| No fuzzy matching | Convenience vs predictability | Scripts need reliability |
| SQLite | Scalability vs simplicity | Personal use, simplicity wins |
| CLI-first | TUI features vs consistency | Force good CLI design |
| No auto-repair | Convenience vs transparency | Trust requires transparency |
| UUID v7 | Compatibility vs sorting | Sorting by creation time nice |

---

## Versioning Strategy

### Schema Version

```
CURRENT: schema_version = 1
STORED: schema_metadata table
CHECKED: On every DB open
```

### Config Version

```
CURRENT: config_version = 1
STORED: config file
CHECKED: On config load
```

### Event Types

```
STRATEGY: Additive only (no breaking changes)
  - New event types added
  - Existing event types preserved
  - Projection logic handles unknown events (future)
```

---

**Document Purpose**: Guide implementation decisions and explain "why" behind "what"

**Version**: 1.0 (from DESIGN.md v1)
**Last Updated**: 2026-02-19
