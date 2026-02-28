# Database Schema Design Review

> **Reviewer:** db-developer agent
> **Date:** 2026-02-20
> **Document:** docs/plans/2026-02-20-database-schema-design.md
> **Status:** APPROVED WITH MINOR RECOMMENDATIONS

---

## Executive Summary

**Overall Assessment:** STRONG DESIGN - well-aligned with event-sourcing principles and SQLite best practices.

The design demonstrates excellent understanding of:
- Event-sourcing architecture constraints
- SQLite optimization strategies
- Transaction boundary management
- Validation-first approach
- Projection rebuild determinism

**Recommendation:** Approve for implementation with minor adjustments noted below.

---

## 1. Schema Design Analysis

### ✅ EXCELLENT: Events Table Design

**Strengths:**
- JSON-only payload is correct choice for polymorphic events
- `event_id INTEGER PRIMARY KEY AUTOINCREMENT` ensures strict ordering
- No foreign keys from events (immutability preserved)
- Indexes on `event_type` and `timestamp_utc` support common queries

**No issues found.**

### ✅ EXCELLENT: Projection Tables

**locations_current:**
- Cached `full_path_display` and `full_path_canonical` correct for fast hierarchical queries
- `depth` field enables efficient tree queries
- `is_system` flag protects special locations
- Partial index `WHERE is_system = 1` is smart optimization

**items_current:**
- Temporary use state machine fields correct
- `last_event_id` enables replay checkpoint tracking
- NOT unique constraint on `canonical_name` is correct (items can have duplicate names)

**projects_current:**
- Simple design appropriate for v1
- CHECK constraint on status enforces valid transitions

**No schema issues found.**

### ✅ EXCELLENT: Index Strategy

**Strong points:**
- Composite index `(canonical_name, location_id)` perfect for LOCATION:ITEM selector pattern
- Covering index `(location_id, display_name, canonical_name)` optimizes listing queries
- Partial indexes on sparse columns (`in_temporary_use`, `project_id`, `is_system`)
- All foreign key columns indexed

**Verification needed during implementation:**
- Use `EXPLAIN QUERY PLAN` to verify indexes are used
- Monitor index effectiveness with real query patterns

---

## 2. SQLite Best Practices Review

### ✅ EXCELLENT: PRAGMA Configuration

**Correct choices:**
```sql
PRAGMA journal_mode = WAL;           ✓ Correct for concurrency
PRAGMA foreign_keys = ON;            ✓ CRITICAL - must enable explicitly
PRAGMA synchronous = NORMAL;         ✓ Safe for WAL + network storage
PRAGMA busy_timeout = 30000;         ✓ Appropriate for network mounts
PRAGMA wal_autocheckpoint = 1000;    ✓ Prevents unbounded WAL growth
```

**All pragmas are appropriate for the use case.**

### ⚠️ MINOR RECOMMENDATION: Connection Pooling

**Issue:** Document states "Single connection - pooling provides no benefit"

**Clarification:**
- For **write** operations: Correct - SQLite serializes writes
- For **read** operations: A small pool (2-5 connections) can improve read concurrency in WAL mode

**Recommendation:**
```go
// Connection setup
db.SetMaxOpenConns(5)   // Allow concurrent reads in WAL mode
db.SetMaxIdleConns(2)   // Keep some connections ready
db.SetConnMaxLifetime(time.Hour)
```

**Impact:** LOW - single connection is acceptable for v1, but consider pool for read-heavy workloads.

### ✅ EXCELLENT: Transaction Strategy

**Correct approach:**
- `BEGIN IMMEDIATE TRANSACTION` prevents deferred lock upgrades
- Deferred `tx.Rollback()` pattern is safe
- Atomic scope: validate → insert event → update projection

**No issues found.**

---

## 3. Event-Sourcing Architecture Alignment

### ✅ EXCELLENT: Critical Invariants Preserved

**Verified against business-rules.md:**

| Invariant | Design Implementation | Status |
|-----------|----------------------|--------|
| Event ordering by `event_id` | `INTEGER PRIMARY KEY AUTOINCREMENT` | ✓ |
| Events immutable | No DELETE/UPDATE on events table, no FKs to events | ✓ |
| No silent repair | Validation failures stop replay immediately | ✓ |
| Projections disposable | Rebuild logic preserves system locations, replays all events | ✓ |
| `from_location_id` validation | Explicit comparison in validation functions | ✓ |

**All critical invariants correctly implemented.**

### ✅ EXCELLENT: Validation Logic

**Pre-insert validation (item.moved):**
```go
// 1. Item must exist
// 2. CRITICAL: from_location_id must match projection
// 3. Target location must exist
// 4. Cannot move to same location
// 5. Project validation (if project_action="set")
```

**Validation logic is comprehensive and correct.**

### ✅ EXCELLENT: Projection Update Logic

**Temporary use state machine:**
```go
if event.MoveType == "temporary_use" {
    if !item.InTemporaryUse {
        // First temp move - capture origin
        newTempOrigin = sql.NullString{String: event.FromLocationID, Valid: true}
    } else {
        // Subsequent temp move - preserve origin
        newTempOrigin = item.TempOriginLocationID
    }
} else { // "rehome"
    // Clear temporary state
    newInTempUse = false
    newTempOrigin = sql.NullString{Valid: false}
}
```

**State machine logic correct - matches business-rules.md requirements.**

---

## 4. Replay & Rebuild Logic Review

### ✅ EXCELLENT: Replay Strategy

**Rebuild process:**
```sql
-- 1. Clear projections (preserve system locations)
DELETE FROM items_current;
DELETE FROM locations_current WHERE is_system = 0;  ✓ Correct
DELETE FROM projects_current;

-- 2. Replay events in strict order
SELECT * FROM events ORDER BY event_id ASC;  ✓ Correct ordering
```

**Correct approach - system locations preserved during rebuild.**

### ✅ EXCELLENT: Fail-Fast on Validation Errors

```go
if err := applyEventToProjection(tx, eventID, eventType, payloadJSON, timestampUTC); err != nil {
    return fmt.Errorf("replay failed at event_id=%d: %w", eventID, err)
}
```

**Correct - no silent repair, stops immediately with diagnostic info.**

### ⚠️ MINOR RECOMMENDATION: Replay Transaction Scope

**Current design:** Rebuild all projections in single transaction

**Consideration:**
- Single transaction = atomic rebuild (good)
- Large event logs may hit SQLite transaction size limits (unlikely but possible)

**Recommendation:**
- For v1: Single transaction is fine
- For future: Consider batch commits every N events (e.g., 10,000) with rollback on failure

**Impact:** LOW - not an issue for typical use cases, only extreme event logs.

---

## 5. Query Performance Analysis

### ✅ EXCELLENT: Index Coverage for Query Patterns

**Verified query → index mapping:**

| Query Pattern | Index Used | Status |
|---------------|------------|--------|
| Find item by name | `idx_items_canonical` | ✓ |
| Find item by location | `idx_items_location` | ✓ |
| LOCATION:ITEM selector | `idx_items_canonical_location` (composite) | ✓ |
| List items in location (hierarchical) | `idx_locations_path_canonical` | ✓ |
| Items for project | `idx_items_project` (partial) | ✓ |
| Items in temp use | `idx_items_temp_use` (partial) | ✓ |
| List items at location (with covering) | `idx_items_location_covering` | ✓ |

**All critical query patterns have appropriate indexes.**

### ✅ GOOD: Path Prefix Queries

**Hierarchical query pattern:**
```sql
WHERE l.full_path_canonical LIKE 'garage%'
```

**Index support:**
- `idx_locations_path_canonical` supports prefix matching
- LIKE with leading wildcard (`%pattern`) would NOT use index (not used in design)

**Correct approach.**

### ⚠️ MINOR OPTIMIZATION: Consider Full-Text Search

**Current design:** No full-text search indexes

**Use case:** User searches "socket wrench" across all items

**Current approach:**
```sql
WHERE canonical_name LIKE '%socket%wrench%'  -- No index
```

**Future consideration:**
```sql
CREATE VIRTUAL TABLE items_fts USING fts5(
    item_id, display_name, canonical_name,
    content='items_current'
);
```

**Impact:** LOW - exact name lookup works fine for v1, FTS is future enhancement.

---

## 6. Transaction Boundaries & Concurrency

### ✅ EXCELLENT: Transaction Scope

**Correct scope for commands:**
```go
func (db *Database) CreateItem(ctx context.Context, event ItemCreatedEvent) error {
    tx.Begin()
    defer tx.Rollback()

    // 1. Validate (read projection)
    // 2. Insert event
    // 3. Update projection

    tx.Commit()
}
```

**Entire operation is atomic - correct approach.**

### ✅ EXCELLENT: Isolation Level

**SQLite default = SERIALIZABLE:**
- Strongest consistency guarantee
- No phantom reads, no dirty reads
- Correct for event-sourcing (prevents concurrent modification)

**No issues found.**

### ⚠️ MINOR RECOMMENDATION: Retry Logic Enhancement

**Current design:**
```go
func withRetry(ctx context.Context, maxRetries int, fn func() error) error {
    for i := 0; i < maxRetries; i++ {
        if err := fn(); err != nil {
            if isBusyError(err) && i < maxRetries-1 {
                time.Sleep(time.Duration(50*(1<<i)) * time.Millisecond)
                continue
            }
            return err
        }
        return nil
    }
}
```

**Recommendation:** Add context cancellation check:
```go
if err := ctx.Err(); err != nil {
    return fmt.Errorf("context cancelled: %w", err)
}
```

**Impact:** LOW - improves graceful shutdown.

---

## 7. Data Integrity & Constraints

### ✅ EXCELLENT: Foreign Key Constraints

**Verified:**
```sql
-- items_current
FOREIGN KEY (location_id) REFERENCES locations_current(location_id)
FOREIGN KEY (temp_origin_location_id) REFERENCES locations_current(location_id)
FOREIGN KEY (project_id) REFERENCES projects_current(project_id)

-- locations_current
FOREIGN KEY (parent_id) REFERENCES locations_current(location_id)
```

**All relationships properly constrained.**

**CRITICAL:** `PRAGMA foreign_keys=ON` is set on connection (verified in design).

### ✅ EXCELLENT: CHECK Constraints

```sql
CHECK (status IN ('active', 'completed'))  -- projects_current
```

**Enforces valid enum values at database level.**

### ✅ EXCELLENT: UNIQUE Constraints

```sql
canonical_name TEXT NOT NULL UNIQUE  -- locations_current
```

**Prevents duplicate location names (required by business rules).**

### ⚠️ MINOR IMPROVEMENT: Additional CHECK Constraints

**Recommendation:** Add invariant checks:

```sql
-- items_current: temp_origin_location_id should be NULL when not in temporary use
ALTER TABLE items_current ADD CONSTRAINT chk_temp_use_consistency
CHECK (
    (in_temporary_use = 1 AND temp_origin_location_id IS NOT NULL) OR
    (in_temporary_use = 0 AND temp_origin_location_id IS NULL)
);
```

**Impact:** LOW - validation logic already enforces this, but database-level check provides additional safety.

---

## 8. Error Handling & Testing Strategy

### ✅ EXCELLENT: Error Taxonomy

```go
var (
    ErrNotFound               = errors.New("entity not found")
    ErrAlreadyExists          = errors.New("entity already exists")
    ErrValidation             = errors.New("validation failed")
    ErrConcurrentModification = errors.New("concurrent modification detected")
    ErrIntegrity              = errors.New("integrity constraint violation")
)
```

**Good error types for user-facing diagnostics.**

### ✅ EXCELLENT: SQLite Error Handling

```go
func HandleSQLiteError(err error) error {
    // Check for sql.ErrNoRows
    // Check for SQLite constraint errors
    // Check for BUSY/LOCKED errors
}
```

**Comprehensive coverage of SQLite-specific errors.**

### ✅ EXCELLENT: Testing Strategy

**Test organization:**
- `schema_test.go` - Schema creation, version checks
- `events_test.go` - Event insertion
- `projections_test.go` - Projection CRUD
- `replay_test.go` - Replay logic, consistency
- `validation_test.go` - Validation rules
- `integration_test.go` - End-to-end workflows

**Comprehensive test coverage plan.**

### ✅ EXCELLENT: Test Patterns

**In-memory database for tests:**
```go
func setupTestDB(t *testing.T) *Database {
    db, err := Open(":memory:")
    require.NoError(t, err)
    t.Cleanup(func() { db.Close() })
    return db
}
```

**Correct approach - fast, isolated tests.**

**Index verification test:**
```go
func TestStore_FindItemsByLocation_UsesIndex(t *testing.T) {
    const query = `EXPLAIN QUERY PLAN SELECT ...`
    // Verify index is used (not SCAN TABLE)
}
```

**Excellent - verifies query optimization.**

---

## 9. Schema Initialization & Migrations

### ✅ EXCELLENT: Auto-Initialization

```go
func ensureSchema(db *sql.DB) error {
    // Check if schema exists
    // If not, create it
    // If exists, verify version
    // Seed system locations during init
}
```

**Correct approach for first-run setup.**

### ✅ EXCELLENT: System Location Seeding

```go
// Seed system locations (Missing, Borrowed) with generated UUIDs
```

**Correct - system locations created during initialization, UUIDs assigned once.**

### ⚠️ MINOR RECOMMENDATION: Migration Framework

**Current design:** "V1 uses embedded schema for simplicity. Migration complexity deferred."

**Trade-off acknowledged:** Correct decision for v1.

**Recommendation for v2:**
- Use golang-migrate or custom migration framework
- Migration files: `internal/database/migrations/001_description.sql`
- Track applied migrations in `schema_migrations` table (separate from `schema_metadata`)

**Example:**
```sql
CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL,
    description TEXT NOT NULL
);
```

**Impact:** LOW - not needed for v1, plan ahead for v2.

---

## 10. Potential Issues & Edge Cases

### ⚠️ POTENTIAL ISSUE: Location Reparenting Performance

**Current design:**
```python
def recompute_paths(location_id, new_parent_id):
    # Recursively update all descendants
    for child in get_children(location_id):
        recompute_paths(child.location_id, location_id)
```

**Issue:** Deep trees or wide subtrees may cause performance issues.

**Example:** Reparenting a location with 1,000 descendants = 1,000+ UPDATE queries.

**Recommendation:**
- For v1: Acceptable (location reparenting is rare operation)
- For future optimization: Use Common Table Expressions (CTEs) for recursive update

```sql
WITH RECURSIVE subtree AS (
    SELECT location_id, parent_id FROM locations_current
    WHERE location_id = ?
    UNION ALL
    SELECT l.location_id, l.parent_id
    FROM locations_current l
    JOIN subtree s ON l.parent_id = s.location_id
)
UPDATE locations_current
SET full_path_display = ..., full_path_canonical = ...
WHERE location_id IN (SELECT location_id FROM subtree);
```

**Impact:** MEDIUM - consider optimization if users report slow reparenting.

### ⚠️ POTENTIAL ISSUE: Event Log Growth

**Current design:** No event log pruning or archival strategy.

**Issue:** Event table grows indefinitely.

**Recommendation:**
- For v1: Not an issue (events are small, SQLite handles millions of rows)
- For future: Consider archival strategy (move old events to separate database)

**Impact:** LOW - not a v1 concern, monitor long-term.

### ✅ NO ISSUE: Concurrent Write Conflicts

**Scenario:** Two users move same item simultaneously.

**Design handles correctly:**
1. First transaction commits successfully
2. Second transaction fails validation: `from_location_id` mismatch
3. Error returned to user: "Location mismatch (possible concurrent modification)"

**Correct detection and error handling.**

---

## 11. SQL Injection Safety

### ✅ EXCELLENT: Prepared Statements

**All queries use parameterized statements:**
```go
result, err := tx.Exec(`
    INSERT INTO events (event_type, timestamp_utc, actor_user_id, payload, note)
    VALUES (?, ?, ?, ?, ?)
`, eventType, time.Now().UTC(), userID, payloadJSON, note)
```

**No string concatenation for SQL - safe from injection.**

### ✅ EXCELLENT: Parameter Binding

**Correct use of `?` placeholders and ordered parameters.**

**No SQL injection vulnerabilities found.**

---

## 12. Missing Elements Analysis

### ⚠️ MINOR: Missing Event Table Indexed Columns

**Current design:** JSON-only payload, no indexed columns for entity IDs.

**From events.md:**
```sql
-- Optional: separate indexed columns for critical fields
item_id          TEXT,           -- for item events
location_id      TEXT,           -- for location events
project_id       TEXT,           -- for project events
```

**Recommendation:** Add indexed ID columns for history queries:

```sql
CREATE TABLE events (
    event_id         INTEGER PRIMARY KEY AUTOINCREMENT,
    event_type       TEXT NOT NULL,
    timestamp_utc    TEXT NOT NULL,
    actor_user_id    TEXT NOT NULL,
    payload          TEXT NOT NULL,
    note             TEXT,

    -- Indexed entity IDs (extracted from payload)
    item_id          TEXT,
    location_id      TEXT,
    project_id       TEXT
);

CREATE INDEX idx_events_item_id ON events(item_id) WHERE item_id IS NOT NULL;
CREATE INDEX idx_events_location_id ON events(location_id) WHERE location_id IS NOT NULL;
CREATE INDEX idx_events_project_id ON events(project_id) WHERE project_id IS NOT NULL;
```

**Use case:** "Show history for item X" (very common query).

**Without index:** Full table scan + JSON parsing.
**With index:** Efficient lookup.

**Impact:** MEDIUM - strongly recommend adding for v1.

### ✅ COVERED: Audit Trail

**User attribution:** `actor_user_id` in every event - correct.

### ✅ COVERED: Timestamps

**UTC RFC3339:** Correct format for portability and timezone safety.

---

## 13. Documentation Quality

### ✅ EXCELLENT: Comprehensive Documentation

**Strengths:**
- Clear rationale for design decisions
- Explicit trade-offs documented
- Code examples for all patterns
- Integration with business-rules.md verified

**No documentation gaps found.**

---

## Final Recommendations Summary

### CRITICAL (Must Fix Before Implementation)
- **NONE** - Design is sound for implementation.

### STRONGLY RECOMMENDED (Implement in v1)
1. **Add indexed entity ID columns to events table** (item_id, location_id, project_id)
   - Enables efficient history queries
   - Small storage overhead, significant query benefit

### MINOR RECOMMENDATIONS (Consider for v1)
2. **Add CHECK constraint for temp_origin_location_id consistency**
   - Provides database-level invariant enforcement
   - Low effort, high safety value

3. **Add context cancellation check to retry logic**
   - Improves graceful shutdown
   - One-line addition

### FUTURE ENHANCEMENTS (Defer to v2+)
4. **Consider connection pooling for read concurrency** (5 connections)
5. **Optimize location reparenting with CTEs** (if performance issues reported)
6. **Add full-text search for item names** (FTS5 virtual table)
7. **Design migration framework** (golang-migrate or custom)

---

## Review Checklist Results

- [✓] Schema design follows event-sourcing principles
- [✓] Indexes cover all critical query patterns
- [✓] Foreign keys properly defined and enforced
- [✓] Transaction boundaries are correct and atomic
- [✓] Validation logic is complete and correct
- [✓] Replay logic handles all event types
- [✓] Temporary use state machine is correct
- [✓] Path recomputation logic is sound
- [✓] Error handling is appropriate
- [✓] Testing strategy is comprehensive
- [✓] SQLite PRAGMAs are correct for use case
- [✓] JSON payload structure is appropriate
- [✓] No SQL injection vulnerabilities
- [✓] Performance considerations addressed

**All checklist items passed.**

---

## Final Verdict

**APPROVED FOR IMPLEMENTATION** with minor recommendations.

The database schema design is **excellent** and demonstrates deep understanding of:
- Event-sourcing architecture constraints
- SQLite optimization and best practices
- Transaction management and concurrency
- Validation-first approach
- Replay determinism and consistency

**Quality Level:** Production-ready with minor enhancements.

**Confidence:** HIGH - design aligns perfectly with business rules and events catalog.

---

## Next Steps

1. **Implement strongly recommended enhancement:** Add indexed entity ID columns to events table
2. **Proceed with implementation** following the patterns documented in design
3. **Write comprehensive tests** as outlined in testing strategy
4. **Verify index usage** with EXPLAIN QUERY PLAN during implementation
5. **Create implementation plan** (use writing-plans skill)

---

**Reviewer:** db-developer agent
**Review Date:** 2026-02-20
**Recommendation:** APPROVE WITH MINOR ENHANCEMENTS
