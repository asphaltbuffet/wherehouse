# Event-Sourced Libraries vs Custom Implementation

**Purpose**: Comparative analysis of Go event-sourcing libraries versus Wherehouse's custom implementation
**Date**: 2026-02-21
**Status**: Research complete, no implementation changes

---

## Executive Summary

This document analyzes whether Wherehouse should adopt an existing Go event-sourcing library or continue with its planned custom implementation. After evaluating 6 major libraries against Wherehouse's specific requirements, **the recommendation is to proceed with the custom implementation** due to:

1. **Simplicity** - Wherehouse needs only 5-10% of what these libraries provide
2. **SQLite focus** - Most libraries optimize for distributed systems, not single-file databases
3. **Control** - Custom implementation aligns precisely with design philosophy
4. **Learning curve** - Library abstraction overhead exceeds implementation complexity for this use case

---

## Libraries Evaluated

### 1. Event Horizon (looplab/eventhorizon)
- **Last Update**: v0.16.0 (Dec 2022) - 836 commits
- **Stars**: ~1.5k (popular)
- **Focus**: Full CQRS/ES toolkit for distributed systems
- **Backends**: MongoDB (primary), PostgreSQL, DynamoDB, Redis (community)
- **Complexity**: High - comprehensive framework with middleware, tracing, event buses
- **SQLite Support**: None native

### 2. hallgren/eventsourcing
- **Last Update**: Active (889 commits)
- **Stars**: ~300
- **Focus**: Aggregate-centric event sourcing
- **Backends**: SQLite ✓, PostgreSQL, SQL Server, BBolt, Event Store DB
- **Complexity**: Moderate - requires aggregate pattern implementation
- **SQLite Support**: **Yes** - first-class support

### 3. modernice/goes
- **Last Update**: v0.6.5 (Feb 2026) - actively maintained
- **Stars**: ~200
- **Focus**: Distributed event-driven architectures with CQRS, DDD, Sagas
- **Backends**: MongoDB, PostgreSQL, NATS JetStream
- **Complexity**: Moderate-to-High - designed for multi-service systems
- **SQLite Support**: None

### 4. thefabric-io/eventsourcing
- **Last Update**: Active (24 commits, newer project)
- **Stars**: ~50 (emerging)
- **Focus**: PostgreSQL-optimized with Go generics (type-safe)
- **Backends**: PostgreSQL only
- **Complexity**: Moderate-to-High - heavy use of generics, CQRS consumer patterns
- **SQLite Support**: None

### 5. mishudark/eventhus
- **Last Update**: 105 commits (maintenance mode)
- **Stars**: ~200
- **Focus**: Simple CQRS/ES toolkit
- **Backends**: MongoDB, RabbitMQ, NATS
- **Complexity**: Beginner-to-Moderate
- **SQLite Support**: None

### 6. quintans/eventsourcing
- **Last Update**: Active (257 commits)
- **Stars**: ~100
- **Focus**: Advanced features (GDPR, upcasting, outbox pattern, blue/green)
- **Backends**: PostgreSQL, MySQL, MongoDB, NATS, Kafka
- **Complexity**: High - steep learning curve, complex projection strategies
- **SQLite Support**: None

---

## Wherehouse Requirements vs Library Capabilities

| Requirement | Wherehouse Need | Library Typical Support |
|-------------|-----------------|-------------------------|
| **Event Store** | Simple append-only log in SQLite | MongoDB/PostgreSQL primary, complex abstractions |
| **Ordering** | Integer event_id (autoincrement) | Often UUID-based or timestamp-based |
| **Projections** | 3 tables (items, locations, projects) | Complex projection frameworks with streaming |
| **Replay** | Single-threaded, deterministic | Often distributed, concurrent |
| **Storage** | Single SQLite file | Distributed databases, message buses |
| **CQRS** | Not needed (simple reads from projections) | Full CQRS with command/query buses |
| **Snapshots** | Not planned (rebuild acceptable) | Performance-critical snapshot systems |
| **Event Bus** | Not needed (local-only) | RabbitMQ, NATS, Kafka integrations |
| **Sagas** | Not needed | Long-running process orchestration |
| **Multi-tenancy** | Not needed | Often built-in |
| **Distribution** | Explicitly NOT distributed | Designed for microservices |

**Alignment Score**: ~5-15% feature overlap

---

## Pros of Using a Library

### Code Reduction
- **Event persistence** - pre-built event store abstractions
- **Serialization** - JSON marshaling handled automatically
- **Aggregate patterns** - structured approach to state management
- **Validation hooks** - middleware for cross-cutting concerns
- **Testing utilities** - in-memory stores, fixtures

**Estimate**: 200-400 lines of boilerplate avoided

### Battle-Tested Patterns
- **Concurrency handling** - proven locking strategies
- **Snapshot optimization** - performance patterns for large aggregates
- **Schema evolution** - event versioning and upcasting strategies
- **Error handling** - standardized error types and recovery

### Community Support
- **Documentation** - guides, examples, tutorials
- **Issue resolution** - community troubleshooting
- **Updates** - bug fixes, security patches
- **Integrations** - existing adapters for popular tools

### Future Extensibility
- **Easy additions** - if Wherehouse needs event bus later
- **Migration paths** - scaling to distributed systems
- **Tool ecosystem** - monitoring, tracing, visualization

---

## Cons of Using a Library

### Complexity Overhead

**Unnecessary Abstractions**:
```go
// Library approach (Event Horizon example)
aggregate := eventhorizon.NewAggregate(ItemAggregateType, itemID)
cmd := &MoveItemCommand{from, to, moveType}
err := commandHandler.HandleCommand(ctx, cmd)

// Wherehouse needs (custom)
tx.Exec("INSERT INTO events (...) VALUES (...)")
tx.Exec("UPDATE items_current SET location_id = ? WHERE item_id = ?")
```

**Learning Curve**:
- Understanding library's event model vs direct SQL
- Framework-specific patterns (aggregates, commands, handlers)
- Configuration complexity (stores, buses, serializers)
- Debugging through abstraction layers

**Estimate**: 2-5 days learning + ongoing cognitive overhead

### SQLite Mismatch

**Most libraries optimize for**:
- Network round-trips (connection pooling, batching)
- Distributed consistency (sagas, 2PC)
- Horizontal scaling (sharding, partitioning)

**Wherehouse needs**:
- Single-file simplicity
- Deterministic local replay
- Direct SQL control for doctor command
- Foreign key enforcement (PRAGMA foreign_keys=ON)

**hallgren/eventsourcing** is the only library with SQLite support, but:
- Still uses aggregate pattern (not needed for simple entities)
- Abstracts SQL (loses transparency for doctor validation)
- Snapshot complexity (Wherehouse doesn't need it)

### Loss of Control

**Critical Wherehouse Requirements**:

1. **event_id ordering** - libraries often use timestamps or UUIDs
2. **from_location_id validation** - custom integrity check not standard
3. **Projection rebuilds** - need exact control for doctor --rebuild
4. **No silent repair** - libraries may have automatic recovery
5. **Explicit failures** - framework error handling may obscure root causes

**Risk**: Library assumptions conflict with design philosophy

### Dependency Weight

```
# hallgren/eventsourcing dependencies (example)
- github.com/hallgren/eventsourcing/core
- github.com/hallgren/eventsourcing/eventstore/* (multiple backends)
- JSON marshaling, reflection overhead
- Aggregate interface requirements

# Custom implementation dependencies
- modernc.org/sqlite (OR mattn/go-sqlite3)
- encoding/json (stdlib)
- database/sql (stdlib)
```

**Custom approach**: ~50-100 lines for event store + projections
**Library approach**: 1000+ lines of library code + adapter glue

### Philosophical Misalignment

**Wherehouse Design Values**:
```
✓ Explicit over implicit
✓ Simple over feature-rich
✓ Transparent over abstracted
✓ Deterministic over convenient
```

**Library Tendencies**:
```
✗ Implicit framework magic (command handlers, middleware)
✗ Feature-rich (90% unused features)
✗ Abstracted storage (can't inspect raw SQL)
✗ Convenient defaults (may hide behavior)
```

**Quote from architecture.md**:
> "No Silent Magic... User should always understand what happened and why."

Libraries prioritize **generality**, Wherehouse prioritizes **transparency**.

---

## Trade-offs Analysis

### Scenario 1: Using hallgren/eventsourcing (Best Fit)

**Gains**:
- SQLite support out of the box
- Aggregate pattern for state management
- Snapshot support (future-proofing)
- Testing in-memory store

**Costs**:
- Must model items/locations as aggregates (conceptual overhead)
- Event ID may not be guaranteed integer sequence
- Projection rebuild needs custom code anyway (doctor command)
- Aggregate.Transition() method for every event type (boilerplate)
- Loss of direct SQL transparency

**Net**: Saves ~150 lines of event store code, costs ~200 lines of aggregate adapters + learning time

### Scenario 2: Custom Implementation (Current Plan)

**Gains**:
- Total control over event_id ordering (INTEGER AUTOINCREMENT)
- Direct SQL for doctor validation and rebuild
- from_location_id integrity checks exactly as designed
- Zero abstraction overhead (read code = understand behavior)
- No dependency beyond SQLite driver

**Costs**:
- Write ~200-300 lines of event store + replay logic
- Manual JSON marshaling for event payloads
- No community patterns (design from first principles)
- Future scaling requires more custom work

**Net**: More initial code, perfect alignment with requirements

---

## Key Decision Factors

### 1. Wherehouse is NOT a typical event-sourced system

**Typical event-sourced application**:
- Distributed microservices
- High concurrency demands
- Complex aggregates with business rules
- Event bus pub/sub
- CQRS with separate read/write models

**Wherehouse**:
- Single-user CLI tool (low concurrency)
- Simple entities (items, locations, projects)
- Projections are just denormalized views
- No event distribution needed
- Single SQLite file

**Verdict**: Libraries solve problems Wherehouse doesn't have

### 2. Code volume comparison

**Custom Event Store** (~200 lines):
```go
// events.go
type EventStore interface {
    Append(event Event) error
    GetAll() ([]Event, error)
    GetByEntity(id string) ([]Event, error)
}

// sqlite_eventstore.go (implementation)
// - INSERT INTO events
// - SELECT with ORDER BY event_id
// - JSON marshal/unmarshal
```

**Custom Replay** (~100 lines):
```go
// replay.go
func RebuildProjections(db *sql.DB) error {
    // Clear projection tables
    // SELECT events ORDER BY event_id
    // for event: validate + apply
    // return error on validation failure
}
```

**Total custom code**: ~300 lines
**hallgren/eventsourcing integration**: ~250 lines (adapters) + library complexity

**Savings**: Negligible

### 3. Transparency requirement

**Doctor command needs**:
```go
// Must compare:
rebuilt_projection == current_projection

// Requires:
- Exact replay algorithm visibility
- Direct SQL comparison queries
- Byte-for-byte deterministic results
```

**Library approach**:
- Replay algorithm hidden in framework
- Comparison logic custom anyway
- Harder to debug mismatches

**Custom approach**:
- Replay logic in 100 lines of visible Go
- Direct SQL queries for comparison
- Easy to trace discrepancies

**Verdict**: Transparency wins

### 4. Future-proofing consideration

**If Wherehouse later needs**:
- Event bus (NATS, RabbitMQ): Add publisher after event append (10 lines)
- Snapshots: Add snapshot table + replay optimization (50 lines)
- Event versioning: Add upcasting in replay loop (20 lines per event)
- Multiple projections: Same replay, different handlers (minimal)

**Library lock-in risk**:
- Switching libraries = rewrite event store layer
- Upgrading library versions = potential breaking changes
- Library abandonment = maintenance burden

**Custom implementation**:
- ~300 lines to maintain (manageable)
- Easy to extend (own code)
- No external dependency risk

**Verdict**: Custom is more future-proof for this use case

---

## Recommendation

### **Proceed with Custom Implementation**

**Rationale**:

1. **Simplicity**: Wherehouse event-sourcing needs are straightforward (~300 lines total)
2. **SQLite alignment**: Custom implementation optimizes for single-file, local-first design
3. **Transparency**: Direct SQL access critical for doctor command validation
4. **Control**: event_id ordering, from_location_id validation, explicit failure modes
5. **No bloat**: Libraries bring 90%+ unused features designed for distributed systems
6. **Learning curve**: Understanding 300 lines of custom code < learning library abstractions
7. **Maintenance**: Smaller surface area, no dependency upgrade treadmill

**Risk Mitigation**:
- Reference hallgren/eventsourcing for patterns (don't copy code, learn approach)
- Document event store contract clearly (for future maintainers)
- Write comprehensive tests for replay logic (no library to rely on)
- Keep event storage simple (resist premature optimization)

---

## Implementation Guidance (Custom Approach)

### Core Components

**1. Event Store** (`internal/database/events.go`)
```go
// Simple interface
type EventStore interface {
    Append(ctx context.Context, event Event) (eventID int64, err error)
    GetAll(ctx context.Context) ([]Event, error)
    GetByEntity(ctx context.Context, entityType, entityID string) ([]Event, error)
    GetSince(ctx context.Context, eventID int64) ([]Event, error)
}

// SQLite implementation (150 lines)
// - INSERT with JSON payload
// - SELECT with ORDER BY event_id
// - Unmarshal type-specific payloads
```

**2. Projection Builder** (`internal/projections/builder.go`)
```go
// Replay events into projections
func Rebuild(ctx context.Context, db *sql.DB, eventStore EventStore) error {
    // Tx: clear projection tables
    // events := eventStore.GetAll()
    // for each event: validate + apply
    // Commit or rollback on error
}

// Incremental update (for live commands)
func ApplyEvent(ctx context.Context, tx *sql.Tx, event Event) error {
    // Validate against current projection
    // Update projection tables
    // Validate invariants
}
```

**3. Event Types** (`internal/events/types.go`)
```go
// Base event
type Event struct {
    EventID      int64     `json:"event_id"`
    EventType    string    `json:"event_type"`
    TimestampUTC time.Time `json:"timestamp_utc"`
    ActorUserID  string    `json:"actor_user_id"`
    Payload      json.RawMessage `json:"payload"`
    Note         *string   `json:"note,omitempty"`
}

// Type-specific payloads
type ItemMovedPayload struct {
    ItemID         string  `json:"item_id"`
    FromLocationID string  `json:"from_location_id"`
    ToLocationID   string  `json:"to_location_id"`
    MoveType       string  `json:"move_type"`
    ProjectAction  string  `json:"project_action"`
    ProjectID      *string `json:"project_id,omitempty"`
}
```

**4. Doctor Command** (`cmd/doctor.go`)
```go
// Validate consistency
func runDoctor(rebuild bool) error {
    // temp_db := create in-memory database
    // Rebuild(temp_db, eventStore) // build fresh projections
    // if rebuild: replace main projections
    // else: compare temp vs main, report diffs
}
```

**Total Lines**: ~300-400 (well within single-file comprehension)

### Testing Strategy

```go
// Integration tests with in-memory SQLite
func TestEventReplay(t *testing.T) {
    db := setupTestDB()
    store := NewSQLiteEventStore(db)

    // Append events
    store.Append(itemCreatedEvent)
    store.Append(itemMovedEvent)

    // Rebuild projections
    Rebuild(db, store)

    // Assert projection state
    assertItemLocation(t, db, itemID, expectedLocationID)
}

// Validation failure test
func TestReplayFailsOnInvalidFrom(t *testing.T) {
    // Create event with wrong from_location_id
    // Expect replay to fail loudly
}
```

---

## When to Reconsider

**Use a library if Wherehouse later needs**:
- Distributed deployment (multi-server event processing)
- Event bus integration as primary feature
- Complex sagas (multi-step workflows)
- Horizontal scaling (10M+ events)
- Advanced snapshot strategies (aggregate performance critical)

**Signal to reconsider**: When event-sourcing complexity exceeds 1000 lines of custom code

**Current estimate**: 300-400 lines (well below threshold)

---

## References

### Libraries Researched
- [Event Horizon (looplab/eventhorizon)](https://github.com/looplab/eventhorizon)
- [hallgren/eventsourcing](https://github.com/hallgren/eventsourcing)
- [modernice/goes](https://github.com/modernice/goes)
- [thefabric-io/eventsourcing](https://github.com/thefabric-io/eventsourcing)
- [mishudark/eventhus](https://github.com/mishudark/eventhus)
- [quintans/eventsourcing](https://github.com/quintans/eventsourcing)
- [Awesome Go CQRS/ES](https://github.com/snamiki1212/awesome-go-cqrs-event-sourcing)

### Articles Consulted
- [Simplifying Event Sourcing in Golang - TheFabric.IO](https://www.thefabric.io/blog/simplifying-event-sourcing-in-golang)
- [Event Sourcing in Go - Victor's Blog](https://victoramartinez.com/posts/event-sourcing-in-go/)
- [How to write Event-Sourcing library in Go - Medium](https://medium.com/@0x9ef/how-to-write-event-sourcing-library-in-go-2b84d28445b9)

---

## Conclusion

**Wherehouse should implement event-sourcing from scratch** using direct SQLite operations. The custom approach:

✅ Aligns with design philosophy (explicit, transparent, simple)
✅ Matches technical requirements (SQLite, event_id ordering, validation)
✅ Minimizes complexity (300 lines vs library abstraction)
✅ Maintains control (doctor command, replay logic)
✅ Reduces dependencies (only SQLite driver)
✅ Keeps code readable (custom beats framework magic)

**The best library is no library** for this use case.

---

**Version**: 1.0
**Status**: Research complete
**Next Steps**: Proceed with custom implementation as planned in DESIGN.md
