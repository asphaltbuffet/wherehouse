# Wherehouse Agent System - Complete Capabilities Reference

**Date**: 2026-02-19
**Version**: 1.0
**Status**: Complete 6-Agent System

This document provides a comprehensive reference for all agents in the wherehouse project.

---

## Table of Contents

1. [System Overview](#system-overview)
2. [Agent Specifications](#agent-specifications)
3. [Agent Capabilities Matrix](#agent-capabilities-matrix)
4. [When to Use Each Agent](#when-to-use-each-agent)
5. [Quality Standards](#quality-standards)
6. [Integration Patterns](#integration-patterns)
7. [Quick Reference](#quick-reference)

---

## System Overview

The wherehouse project uses **6 specialized agents** that work together to provide complete software development capabilities:

```
┌─────────────────────────────────────────────────────────┐
│                  WHEREHOUSE AGENT SYSTEM                │
│                   6 Specialized Agents                  │
└─────────────────────────────────────────────────────────┘

    Architecture          Implementation           Quality Assurance
        │                      │                          │
        ▼                      ▼                          ▼
┌────────────────┐   ┌─────────────────────┐   ┌──────────────────┐
│ golang-        │   │ golang-developer    │   │ golang-tester    │
│ architect      │──▶│ db-developer        │──▶│ code-reviewer    │
│                │   │ golang-ui-developer │   │                  │
└────────────────┘   └─────────────────────┘   └──────────────────┘
```

**Design Philosophy**:
- **Specialized Expertise**: Each agent masters one domain
- **Clear Boundaries**: No overlap in responsibilities
- **Quality Gates**: Multiple verification layers
- **Parallel Execution**: Independent work runs simultaneously
- **Consistent Standards**: All agents follow same quality criteria

---

## Agent Specifications

### 1. 🔵 golang-architect (opus, blue)

**Primary Role**: Architecture Design & Planning

**Model**: opus (most capable reasoning for complex design decisions)

**Color**: blue (traditional architecture/planning color)

**Scope**:
- All Go code architecture across entire project
- Package structure and organization
- Interface design and API boundaries
- Component interactions and dependencies
- Trade-off analysis and decision documentation

**Responsibilities**:
- ✅ Design package structure (`/cmd/`, `/pkg/`, `/internal/`)
- ✅ Define interfaces and abstractions
- ✅ Plan implementation approach
- ✅ Document architectural decisions
- ✅ Identify dependencies between components
- ✅ Evaluate trade-offs (simplicity vs. flexibility, etc.)
- ✅ Create architecture plans for features

**Does NOT**:
- ❌ Implement code (delegates to implementation agents)
- ❌ Write tests (delegates to golang-tester)
- ❌ Review code (delegates to code-reviewer)

**Output Format**:
```markdown
# Architecture Plan Complete

Status: [Success/Partial/Failed]
Approach: [One-liner architecture summary]
Complexity: [Simple/Medium/Complex]
Key decisions: [Top 2-3 architectural decisions]
Details: [full-path-to-plan-file]
```

**Key Patterns**:
- Reads DESIGN.md and knowledge docs before planning
- Focuses on simplicity and Go idioms
- Documents alternatives considered and why rejected
- Plans for testability and maintainability
- Considers event-sourcing constraints

**Invocation Example**:
```
User: "Design the architecture for tracking item weights"
Main Chat → golang-architect
Returns: "Architecture plan complete. 3-tier approach: validation → event → projection. Details: ai-docs/..."
```

---

### 2. 🟡 golang-developer (opus, yellow)

**Primary Role**: Core Logic Implementation

**Model**: opus (complex event-sourcing logic requires strong reasoning)

**Color**: yellow (distinct implementation color)

**Scope**:
- `/pkg/` - Reusable packages
- `/internal/events/` - Event handlers
- `/internal/projections/` - Projection builders
- `/internal/models/` - Domain models
- `/internal/validation/` - Business rule validation

**Responsibilities**:
- ✅ Implement event handlers (create, move, delete, etc.)
- ✅ Implement projection update logic
- ✅ Enforce business rules from business-rules.md
- ✅ Validate before event creation (critical!)
- ✅ Write unit tests for own code
- ✅ Run tests and linting on own code
- ✅ Integrate database calls from db-developer

**Does NOT**:
- ❌ Implement database queries (delegates to db-developer)
- ❌ Implement CLI commands (delegates to golang-ui-developer)
- ❌ Design architecture (delegates to golang-architect)
- ❌ Review code (delegates to code-reviewer)

**Output Format**:
```markdown
# [Feature] Implementation Complete

Status: [Success/Partial/Failed]
[One-liner what was implemented]
Changed: [N] files (list key files)
Tests: [X/Y] passing | Linting: [Clean/N errors]
Details: [full-path-to-implementation-file]
```

**Key Patterns**:
- **Event-Sourcing Pattern**: Validate → Create Event → Update Projection (atomic)
- **Validation Pattern**: Always validate `from_location_id` before moves
- **Transaction Pattern**: Event + projection in single transaction
- **Error Handling**: Explicit errors, no silent repair
- **Testing**: Table-driven tests with require/assert

**Critical Rules**:
- Events are immutable (never modify after creation)
- Ordering by `event_id` only (not timestamps)
- No silent repair on validation failures
- Transactions must be atomic (event + projection)

**Invocation Example**:
```
User: "Implement the item.moved event handler"
Main Chat → golang-developer
Returns: "Implementation complete. Tests: 25/25 | Linting: Clean. Details: ..."
```

---

### 3. 🟢 db-developer (opus, green)

**Primary Role**: Database Design & Implementation

**Model**: opus (SQLite optimization and query correctness require expertise)

**Color**: green (database/data color)

**Scope**:
- `/internal/database/` - ALL database code
- Schema design (events, projections)
- Query implementation
- Migration code
- Connection management

**Responsibilities**:
- ✅ Design database schemas (tables, indexes, constraints)
- ✅ Implement SQL queries (prepared statements only)
- ✅ Write migration code with version tracking
- ✅ Configure SQLite (WAL mode, pragmas, foreign keys)
- ✅ Handle NULL values correctly (sql.NullString, etc.)
- ✅ Implement transaction patterns (deferred rollback)
- ✅ Write database tests (in-memory, constraint tests)
- ✅ Run tests and linting on own code
- ✅ Verify index usage with EXPLAIN QUERY PLAN

**Does NOT**:
- ❌ Implement business logic (delegates to golang-developer)
- ❌ Implement CLI commands (delegates to golang-ui-developer)
- ❌ Design Go architecture (delegates to golang-architect)

**Output Format**:
```markdown
# Database [Design/Implementation] Complete

Status: [Success/Partial/Failed]
[One-liner summary of work done]
Changes: [Tables/queries/migrations added or modified]
Tests: [X/Y] passing | Linting: [Clean/N errors]
Details: [full-path-to-implementation-file]
```

**Key Patterns**:
- **Query Pattern**: Always use prepared statements, never string concatenation
- **Transaction Pattern**: `defer tx.Rollback()` for safety
- **NULL Handling**: Use `sql.NullString` for nullable columns
- **Migration Pattern**: Version tracking with up/down functions
- **Testing Pattern**: In-memory databases for speed

**SQLite Configuration**:
```sql
PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;
PRAGMA synchronous=NORMAL;
PRAGMA busy_timeout=30000;
```

**Critical Rules**:
- All queries use prepared statements (security)
- Transactions must be atomic with deferred rollback
- Foreign keys must be ON
- Indexes verified with EXPLAIN QUERY PLAN

**Invocation Example**:
```
User: "Implement query functions for finding items by location"
Main Chat → db-developer
Returns: "DB implementation complete. Tests: 15/15 | Linting: Clean. Details: ..."
```

---

### 4. 🔵 golang-ui-developer (haiku, cyan)

**Primary Role**: CLI & TUI Implementation

**Model**: haiku (CLI patterns are well-established, don't need opus)

**Color**: cyan (user-facing/interface color)

**Scope**:
- `/cmd/` - CLI commands (cobra)
- `/internal/tui/` - TUI screens (if applicable)
- User input handling
- Output formatting

**Responsibilities**:
- ✅ Implement cobra CLI commands
- ✅ Parse and validate flags
- ✅ Call core logic from golang-developer
- ✅ Call database queries from db-developer
- ✅ Format output (human-readable, JSON, quiet modes)
- ✅ Write help text and examples
- ✅ Handle errors with user-friendly messages
- ✅ Write CLI tests (flag parsing, output format)
- ✅ Run tests and linting on own code

**Does NOT**:
- ❌ Implement business logic (delegates to golang-developer)
- ❌ Implement database queries (delegates to db-developer)
- ❌ Duplicate domain logic (thin layer only)

**Output Format**:
```markdown
# [Command] CLI Implementation Complete

Status: [Success/Partial/Failed]
[One-liner what was implemented]
Commands: [List commands added/modified]
Tests: [X/Y] passing | Linting: [Clean/N errors]
Details: [full-path-to-implementation-file]
```

**Key Patterns**:
- **Thin Layer Pattern**: CLI calls core logic, doesn't duplicate it
- **Flag Pattern**: Standard flags (--json, -q, -v, --config)
- **Output Pattern**: Human-readable default, JSON optional
- **Error Pattern**: User-friendly messages with actionable guidance

**CLI Contract**:
- Command structure: `wherehouse <verb> <noun> [flags]`
- Selector syntax: `LOCATION:ITEM` (both canonical names)
- Standard flags: `--json`, `-q/-qq`, `-v/-vv`
- Exit codes: 0=success, non-zero=error

**Invocation Example**:
```
User: "Implement 'wherehouse borrow' command"
Main Chat → golang-ui-developer
Returns: "CLI implementation complete. Tests: 8/8 | Linting: Clean. Details: ..."
```

---

### 5. 🟣 golang-tester (opus, magenta)

**Primary Role**: Testing & Verification

**Model**: opus (comprehensive test design requires deep reasoning)

**Color**: magenta (high visibility for testing work)

**Scope**:
- All `*_test.go` files across entire codebase
- Test infrastructure
- Linting verification
- Coverage analysis

**Responsibilities**:
- ✅ Write tests BEFORE implementation (TDD)
- ✅ Write comprehensive test suites (happy path, boundaries, errors, edge cases)
- ✅ Run entire test suite (`go test ./...`)
- ✅ Run linting on entire codebase (`golangci-lint run`)
- ✅ Verify test coverage
- ✅ Detect race conditions (`-race` flag)
- ✅ Design test strategies for event-sourcing
- ✅ Write database tests (in-memory, constraints)
- ✅ Write CLI tests (flag parsing, output)

**Does NOT**:
- ❌ Implement production code (delegates to implementation agents)
- ❌ Design architecture (delegates to golang-architect)
- ❌ Review code quality (delegates to code-reviewer)

**Output Format**:
```markdown
# Testing [Complete/Failed]

Status: [Success/Failures]
Tests: [X/Y] passing ([Z]% coverage)
Linting: [Clean/N errors]
Key findings: [Critical issues or all clear]
Details: [full-path-to-test-results-file]
```

**Key Patterns**:
- **Testify Pattern**: Use `require.*` for critical checks, `assert.*` for non-blocking
- **Table-Driven Tests**: For validation functions and business rules
- **Event-Sourcing Tests**: Deterministic replay, projection consistency, validation failures
- **Database Tests**: In-memory SQLite, transaction rollback, constraint enforcement
- **CLI Tests**: Flag parsing, output formatting, error messages

**Critical Rules**:
- **ALWAYS** use testify/assert and testify/require
- **NEVER** use t.Fatal, t.Fatalf, t.Error, or t.Errorf
- Use `require.*` when failure would cause nil pointer dereferences
- Use `assert.*` for independent checks

**Test Categories**:
1. **Happy Path**: Normal, expected usage
2. **Boundary Conditions**: Empty inputs, max values, nil checks
3. **Error Conditions**: Invalid inputs, failures, constraint violations
4. **Edge Cases**: Uncommon but valid scenarios
5. **Integration**: Cross-component workflows

**Verification Commands**:
```bash
go test ./... -v -race -coverprofile=coverage.out
golangci-lint run
```

**Success Criteria**:
- ✅ All tests pass
- ✅ No race conditions
- ✅ Linting clean
- ✅ Coverage reasonable

**Invocation Example**:
```
User: "Write tests for item.moved handler before implementing"
Main Chat → golang-tester
Returns: "10 tests written, all failing (expected). Ready for implementation."
```

---

### 6. 🔴 code-reviewer (opus, red)

**Primary Role**: Code Review & Quality Assurance

**Model**: opus (deep code analysis and bug detection require expertise)

**Color**: red (traditional review/critical analysis color)

**Scope**:
- All Go code across entire codebase
- Security analysis
- Performance review
- Code quality assessment

**Responsibilities**:
- ✅ Review code for correctness
- ✅ Identify bugs and logic errors
- ✅ Find security vulnerabilities (SQL injection, etc.)
- ✅ Detect performance issues (N+1 queries, missing indexes)
- ✅ Check event-sourcing correctness
- ✅ Verify business rule enforcement
- ✅ Assess code quality and maintainability
- ✅ Provide actionable feedback with examples

**Does NOT**:
- ❌ Implement fixes (delegates to implementation agents)
- ❌ Write tests (delegates to golang-tester)
- ❌ Design architecture (delegates to golang-architect)

**Output Format**:
```markdown
# Code Review Complete

Assessment: [Ready/Needs Changes/Major Refactor]
Critical: [N] issues | High: [N] issues
Key concern: [Most critical issue]
Risk: [Low/Medium/High]
Details: [full-path-to-review-file]
```

**Review Categories**:

**1. Event-Sourcing Correctness (CRITICAL)**:
- ✅ Events immutable
- ✅ `from_location_id` validation
- ✅ Atomic transactions (event + projection)
- ✅ No silent repair

**2. Business Rule Enforcement (CRITICAL)**:
- ✅ All invariants from business-rules.md enforced
- ✅ No colons in names
- ✅ System location protection
- ✅ Deletion constraints

**3. Security Issues (HIGH)**:
- ✅ Prepared statements only (no SQL injection)
- ✅ No sensitive data in logs
- ✅ Safe error messages

**4. Performance Issues (MEDIUM)**:
- ✅ Indexes used
- ✅ No N+1 queries
- ✅ Efficient algorithms

**5. Code Quality (MEDIUM)**:
- ✅ Functions under ~50 lines
- ✅ Clear naming
- ✅ Proper error handling
- ✅ Good testability

**6. Database Patterns (HIGH for DB code)**:
- ✅ Prepared statements
- ✅ Transaction patterns
- ✅ NULL handling
- ✅ Context propagation

**7. Go Idioms (MEDIUM)**:
- ✅ Errors-as-values
- ✅ Accept interfaces, return structs
- ✅ Small interfaces

**Review Output Structure**:
```markdown
✅ Strengths
- What code does well

⚠️ Concerns
🔴 CRITICAL (must fix before merge)
🟡 HIGH (should fix before merge)
🟠 MEDIUM (consider fixing)
⚪ LOW (optional improvements)

🔍 Questions
- Clarifying questions about intent

📊 Summary
- Assessment: [Ready/Needs Changes/Major Refactor]
- Priority Fixes: [Top 3]
- Risk: [Low/Medium/High]
- Testability: [Good/Fair/Poor]
```

**Confidence-Based Reporting**:
- **HIGH Confidence**: Must report (clear bugs, security issues)
- **MEDIUM Confidence**: Should report (performance, maintainability)
- **LOW Confidence**: Skip (style preferences, nitpicks)

**Invocation Example**:
```
User: "Review the item.moved implementation"
Main Chat → code-reviewer
Returns: "Assessment: Ready to Merge. No critical issues. Risk: Low. Details: ..."
```

---

## Agent Capabilities Matrix

| Capability | architect | developer | db-dev | ui-dev | tester | reviewer |
|------------|-----------|-----------|--------|--------|--------|----------|
| **Design architecture** | ✅ Primary | ❌ | ❌ | ❌ | ❌ | ❌ |
| **Implement core logic** | ❌ | ✅ Primary | ❌ | ❌ | ❌ | ❌ |
| **Implement database** | ❌ | ❌ | ✅ Primary | ❌ | ❌ | ❌ |
| **Implement CLI/TUI** | ❌ | ❌ | ❌ | ✅ Primary | ❌ | ❌ |
| **Write tests** | ❌ | ✅ Own code | ✅ Own code | ✅ Own code | ✅ All code | ❌ |
| **Run tests** | ❌ | ✅ Own code | ✅ Own code | ✅ Own code | ✅ All code | ❌ |
| **Run linting** | ❌ | ✅ Own code | ✅ Own code | ✅ Own code | ✅ All code | ❌ |
| **Review code** | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ Primary |
| **Fix bugs** | ❌ | ✅ Core | ✅ Database | ✅ CLI | ❌ | ❌ |
| **TDD (tests first)** | ❌ | ❌ | ❌ | ❌ | ✅ Primary | ❌ |
| **Verify quality** | ❌ | ✅ Own | ✅ Own | ✅ Own | ✅ All | ✅ All |

---

## When to Use Each Agent

### golang-architect

**Use when:**
- ✅ Starting a new feature that needs design
- ✅ Refactoring existing architecture
- ✅ Evaluating multiple implementation approaches
- ✅ Designing interfaces between components
- ✅ Planning complex features

**Don't use when:**
- ❌ Implementing code (use golang-developer instead)
- ❌ Fixing simple bugs (use golang-developer directly)
- ❌ Writing tests (use golang-tester instead)

### golang-developer

**Use when:**
- ✅ Implementing event handlers
- ✅ Implementing projection logic
- ✅ Writing validation code
- ✅ Creating domain models
- ✅ Fixing bugs in core logic

**Don't use when:**
- ❌ Need database queries (use db-developer)
- ❌ Need CLI commands (use golang-ui-developer)
- ❌ Need architecture design (use golang-architect)

### db-developer

**Use when:**
- ✅ Designing database schemas
- ✅ Implementing SQL queries
- ✅ Writing migrations
- ✅ Configuring SQLite
- ✅ Optimizing database performance
- ✅ Fixing database bugs

**Don't use when:**
- ❌ Need business logic (use golang-developer)
- ❌ Need CLI integration (use golang-ui-developer)

### golang-ui-developer

**Use when:**
- ✅ Implementing CLI commands
- ✅ Adding command flags
- ✅ Formatting output
- ✅ Writing help text
- ✅ Handling user input
- ✅ Fixing CLI bugs

**Don't use when:**
- ❌ Need business logic (use golang-developer)
- ❌ Need database queries (use db-developer)

### golang-tester

**Use when:**
- ✅ Starting TDD (write tests before implementation)
- ✅ Running full test suite verification
- ✅ Checking linting compliance
- ✅ Analyzing test coverage
- ✅ Investigating test failures

**Don't use when:**
- ❌ Need code implementation (use implementation agents)
- ❌ Need code review (use code-reviewer)

### code-reviewer

**Use when:**
- ✅ Implementation complete, ready for review
- ✅ Investigating reported bugs
- ✅ Conducting architectural review
- ✅ Checking for security vulnerabilities
- ✅ Assessing code quality

**Don't use when:**
- ❌ Need code fixes (use implementation agents)
- ❌ Need tests written (use golang-tester)
- ❌ In middle of implementation (wait until complete)

---

## Quality Standards

All agents adhere to consistent quality standards:

### Code Standards

**All implementation agents verify**:
- ✅ Code compiles without errors or warnings
- ✅ `go vet` passes
- ✅ `golangci-lint run` passes (no errors)
- ✅ Tests pass for modified code
- ✅ Code follows Go idioms

### Testing Standards

**golang-tester enforces**:
- ✅ Use testify/require and testify/assert (NEVER t.Fatal/t.Error)
- ✅ Table-driven tests for validation functions
- ✅ Tests cover happy path, boundaries, errors, edge cases
- ✅ Event-sourcing tests (replay, consistency, validation)
- ✅ Database tests (transactions, constraints, indexes)
- ✅ CLI tests (flags, output, errors)

### Event-Sourcing Standards

**All agents respect**:
- ✅ Events are immutable
- ✅ Ordering by `event_id` only
- ✅ `from_location_id` validation required
- ✅ No silent repair on failures
- ✅ Atomic transactions (event + projection)
- ✅ Projections are disposable

### Security Standards

**code-reviewer verifies**:
- ✅ All SQL uses prepared statements
- ✅ No user input in SQL strings
- ✅ Error messages don't leak sensitive data
- ✅ Input validation present
- ✅ No command injection risks

---

## Integration Patterns

### Pattern 1: TDD Workflow

```
1. golang-tester: Write tests first (RED)
2. golang-developer/db-developer/ui-developer: Implement (GREEN)
3. golang-tester: Verify tests pass (GREEN confirmed)
4. code-reviewer: Review quality
5. (Optional) golang-developer: Refactor if needed
6. golang-tester: Verify tests still pass
```

### Pattern 2: Parallel Implementation

```
1. golang-architect: Design system
2. golang-tester: Write tests
3. PARALLEL:
   - db-developer: Implement database
   - golang-developer: Implement core logic
4. golang-ui-developer: Implement CLI (after dependencies ready)
5. golang-tester: Verify everything
6. code-reviewer: Review everything
```

### Pattern 3: Bug Fix Workflow

```
1. code-reviewer: Investigate bug, identify root cause
2. Implementation agent: Fix bug in appropriate layer
   - golang-developer for core bugs
   - db-developer for database bugs
   - golang-ui-developer for CLI bugs
3. golang-tester: Verify fix + no regressions
4. code-reviewer: Verify fix addresses root cause
```

### Pattern 4: Review Feedback Loop

```
1. code-reviewer: Find issues
2. Implementation agents: Fix issues
   - Each agent fixes their domain
   - Each runs own tests + linting
3. golang-tester: Re-verify everything
4. code-reviewer: Re-review
5. Loop until approved
```

---

## Quick Reference

### Invocation Syntax

```bash
# In main chat, use Task tool:
Task tool → [agent-name] with prompt

# Example:
"Use golang-architect to design the weight tracking feature"
"Use golang-tester to write tests for item.moved"
"Use db-developer to implement the query functions"
"Use code-reviewer to review the implementation"
```

### Agent Selection Quick Guide

```
Need to...                          Use...
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Design architecture                 golang-architect
Write tests before implementation   golang-tester
Implement event handlers            golang-developer
Implement database queries          db-developer
Implement CLI commands              golang-ui-developer
Run full test suite                 golang-tester
Review code for quality             code-reviewer
Fix core logic bugs                 golang-developer
Fix database bugs                   db-developer
Fix CLI bugs                        golang-ui-developer
Verify linting across codebase      golang-tester
Check security vulnerabilities      code-reviewer
Optimize performance                code-reviewer (identify) + db-developer (fix)
```

### Output File Locations

**Architecture plans**:
- `ai-docs/sessions/YYYYMMDD-HHMMSS/01-planning/architecture.md`

**Implementation details**:
- `ai-docs/sessions/YYYYMMDD-HHMMSS/02-implementation/[component]-implementation.md`

**Test results**:
- `ai-docs/sessions/YYYYMMDD-HHMMSS/03-testing/test-results.md`

**Code reviews**:
- `ai-docs/sessions/YYYYMMDD-HHMMSS/04-review/code-review.md`

**Ad-hoc work**:
- `ai-docs/research/[category]/[topic].md`

---

## Summary

The wherehouse agent system provides:

✅ **Complete coverage**: Design → Implement → Test → Review
✅ **Specialized expertise**: Each agent masters one domain
✅ **Quality gates**: Multiple verification layers
✅ **Parallel execution**: Independent work runs simultaneously
✅ **Consistent standards**: Tests + linting + review for all code
✅ **Clear boundaries**: No overlap, no confusion

**Result**: High-quality, well-tested, maintainable code with event-sourcing integrity.

---

**Version**: 1.0
**Last Updated**: 2026-02-19
**Agent Count**: 6 (complete system)
