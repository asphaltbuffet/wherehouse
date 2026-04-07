# Wherehouse Knowledge Base

**Authoritative Source**: `docs/DESIGN.md`
**Last Updated**: 2026-02-19
**Version**: 1.0

---

## Document Structure

This knowledge base is optimized for AI agents working on Wherehouse. Each file covers a specific architectural concern:

### 🏗️ [architecture.md](architecture.md)
**When to use**: Understanding design philosophy, trade-offs, key decisions
- Event sourcing rationale
- Technology choices (SQLite, Go)
- Design principles (no magic, explicit, deterministic)
- Trade-offs and rationale
- Future-proofing considerations

**Use for**: Architecture discussions, design reviews, understanding "why"

---

### 📋 [domain-model.md](domain-model.md)
**When to use**: Implementing entities, understanding relationships, canonicalization
- Item, Location, User entities
- Naming rules (display vs canonical)
- Selector syntax (LOCATION:ITEM)
- Canonicalization algorithms
- Entity constraints and lifecycles
- Special locations (Missing, Borrowed)
- Temporary use semantics

**Use for**: Data structure implementation, entity validation, naming logic

---

### 📝 [events.md](events.md)
**When to use**: Implementing event handlers, projection updates, event validation
- Complete event type catalog
- Event schemas (all fields)
- Projection update logic per event
- Validation rules per event
- Replay rules and ordering
- Event storage schema

**Use for**: Event handler implementation, projection logic, replay debugging

---

### 🗂️ [projections.md](projections.md)
**When to use**: Implementing projection tables, rebuild logic, consistency checks
- Projection table schemas
- Rebuild strategy (full vs incremental)
- Path recomputation algorithms
- Consistency validation (doctor command)
- Query optimization patterns
- Concurrency and locking

**Use for**: Database schema, projection rebuild, doctor command, query implementation

---

### ✅ [business-rules.md](business-rules.md)
**When to use**: Implementing validation, enforcing constraints, checking invariants
- Critical invariants (event ordering, immutability)
- Validation rules per event type
- Entity constraints
- Selector resolution rules
- Temporary use tracking rules
- Database constraints and indexes
- "Critical Don'ts" (what never to do)

**Use for**: Validation implementation, constraint enforcement, correctness checks

---

### 💻 [cli-contract.md](cli-contract.md)
**When to use**: Implementing CLI commands, output formatting, shell completion
- Command structure and flags
- Name handling and canonicalization
- Selector syntax
- Output formats (human, JSON, verbosity)
- Completion strategy (bash, zsh, fish)

**Use for**: CLI command implementation, output formatting, completion scripts

---

## Quick Reference by Task

### Implementing a new command
1. **cli-contract.md** - Command interface and flags
2. **business-rules.md** - Validation requirements
3. **events.md** - Which event(s) to create
4. **projections.md** - Projection updates needed

### Adding a new event type
1. **events.md** - Event schema template
2. **business-rules.md** - Validation rules
3. **projections.md** - Projection update logic
4. **domain-model.md** - Entity state changes

### Fixing validation bug
1. **business-rules.md** - Correct validation rules
2. **events.md** - Event-specific validation
3. **domain-model.md** - Entity constraints

### Implementing projections
1. **projections.md** - Schema and rebuild strategy
2. **events.md** - Update logic per event
3. **business-rules.md** - Constraints to enforce

### Understanding design choices
1. **architecture.md** - Philosophy and trade-offs
2. **domain-model.md** - Entity design rationale
3. **business-rules.md** - Invariants and "why"

### Implementing doctor command
1. **projections.md** - Rebuild and comparison logic
2. **business-rules.md** - Validation checks
3. **events.md** - Replay validation rules

---

## File Sizes (Token Efficiency)

| File | Lines | Focus | Token Estimate |
|------|-------|-------|----------------|
| architecture.md | ~420 | Philosophy, decisions | ~3,200 |
| business-rules.md | ~659 | Validation, constraints | ~4,400 |
| cli-contract.md | ~117 | CLI interface | ~900 |
| domain-model.md | ~331 | Entities, relationships | ~2,700 |
| events.md | ~520 | Event catalog | ~4,100 |
| projections.md | ~467 | Projection strategy | ~3,700 |
| **Total** | **~2,514** | **All concerns** | **~19,000** |

**Design Goal**: Keep total under 20K tokens for efficient context use.

---

## How to Use This Knowledge Base

### For New Features
1. Start with **architecture.md** to understand design philosophy
2. Check **domain-model.md** for entity relationships
3. Reference **business-rules.md** for constraints
4. Use **events.md** and **projections.md** for implementation

### For Bug Fixes
1. Start with **business-rules.md** for correct behavior
2. Check **events.md** or **projections.md** for specific logic
3. Validate against **domain-model.md** constraints

### For Refactoring
1. Review **architecture.md** for design principles
2. Ensure **business-rules.md** invariants preserved
3. Update relevant domain/event/projection docs

### For Code Review
1. Validate against **business-rules.md** constraints
2. Check event handling against **events.md**
3. Verify projection logic in **projections.md**
4. Ensure CLI follows **cli-contract.md**

---

## Maintenance

### When to Update
- DESIGN.md changes → rebuild affected knowledge files
- New event types → update events.md, projections.md, business-rules.md
- New entity fields → update domain-model.md, events.md, projections.md
- New validation rules → update business-rules.md
- CLI changes → update cli-contract.md

### Keeping in Sync
- **Source of truth**: `docs/DESIGN.md`
- **Derived**: All .claude/knowledge/ files
- **Process**: Extract and optimize from DESIGN.md
- **Frequency**: After design decisions finalized

---

## Removed Files

### domain_spec.md
**Reason**: Redundant with domain-model.md + events.md
**Date**: 2026-02-19

### Old business-rules.md
**Reason**: Based on outdated PROJECT.md design (CRUD, not event-sourced)
**Date**: 2026-02-19
**Replaced**: Rebuilt from DESIGN.md

### PROJECT.md → docs/PROJECT-v0.md
**Reason**: Outdated design (pre-event-sourcing)
**Date**: 2026-02-19
**Status**: Archived for historical context

---

## Notes for AI Agents

### Reading Strategy
- **Don't load all files** - select relevant files for task
- **Start with README.md** - understand structure first
- **Use Quick Reference** - find right file for task
- **Cross-reference** - files reference each other deliberately

### Critical Sections
- **business-rules.md** "Critical Invariants" - never violate these
- **events.md** "Validation During Replay" - essential for correctness
- **architecture.md** "No Silent Magic" - design philosophy
- **projections.md** "Strict Validation Rules" - consistency checks

### Common Pitfalls
- ❌ Modifying events after creation (immutable)
- ❌ Using timestamps for ordering (event_id only)
- ❌ Auto-repairing corrupted projections (fail explicitly)
- ❌ Auto-creating locations (require explicit creation)
- ❌ Fuzzy matching in command execution (exact only)

---

**This knowledge base is optimized for**:
- Quick task-specific reference
- Token-efficient context loading
- Cross-concern understanding
- Implementation guidance
- Validation and correctness

**Version**: 1.0
**Aligned with**: docs/DESIGN.md v1
