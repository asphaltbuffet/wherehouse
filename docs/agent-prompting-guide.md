# Agent Prompting Guide

**Purpose:** Write effective prompts that minimize token usage while maximizing agent effectiveness.

---

## Quick Rules

1. **Be Specific** - Define exactly what you need
2. **Limit Scope** - Constrain the task explicitly
3. **Specify Format** - Say how much output you want
4. **Choose Right Agent** - Use the most focused agent for the task

---

## Examples

### ❌ Bad Prompts (Waste Tokens)

**Too vague:**
```
"Design a tags system"
```
Problem: Agent will explore requirements, consider alternatives, create comprehensive docs

**Too broad:**
```
"Implement validation"
```
Problem: Agent will implement everything related to validation

**No constraints:**
```
"Review this code"
```
Problem: Agent will review everything in exhaustive detail

### ✅ Good Prompts (Efficient)

**Specific scope:**
```
Design database schema for item tags:
1. Many-to-many relationship (item_tags junction table)
2. DDL only (no Go implementation)
3. Include indexes for common queries
```
Savings: Agent focuses on exactly what's needed

**Limited implementation:**
```
Implement IsValidItemName(name string) error function:
1. Check: not empty, no colons
2. Return: ErrInvalidName on failure
3. Write tests with testify
```
Savings: Agent implements one function, not entire validation layer

**Constrained review:**
```
Review internal/events/item_moved.go for:
1. Event-sourcing violations only
2. Focus on from_location validation
3. Brief summary (3 sentences max)
```
Savings: Agent skips style, performance, general issues

---

## Agent-Specific Tips

### golang-architect

**Scope it:**
```
Design architecture for [feature]:
1. Component breakdown only
2. No implementation details
3. One design doc, under 300 lines
```

**Don't say:**
```
"Design the complete system for managing tags with full implementation guide"
```

### golang-developer

**One thing at a time:**
```
Implement [single function/handler]:
1. Function signature: [exact signature]
2. Required behavior: [2-3 bullet points]
3. Write tests first (TDD)
```

**Don't say:**
```
"Implement all the validation logic for the system"
```

### db-developer

**DDL or code, not both:**
```
Design schema for [feature]:
1. DDL statements only
2. Indexes for [specific queries]
3. No Go implementation yet
```

OR

```
Implement database queries for [feature]:
1. Functions: GetItemsByTag, GetTagsForItem
2. Use prepared statements
3. Write query tests
```

**Don't say:**
```
"Design and implement complete database layer for tags"
```

### golang-ui-developer

**Design OR implementation:**
```
Design CLI for wherehouse tags command:
1. Flag structure only
2. Output modes (human, JSON, quiet)
3. One design file, under 500 lines
```

OR

```
Implement wherehouse tags command:
1. Cobra command in cmd/tags.go
2. Integrate with domain.GetItemTags()
3. Support --json and -q flags
```

**Don't say:**
```
"Create comprehensive CLI documentation with examples, flowcharts, and test strategies"
```

### golang-tester

**Specific function:**
```
Write tests for IsValidItemName function:
1. Test cases: empty, colons, whitespace, valid
2. Use testify require/assert
3. Table-driven structure
```

**Don't say:**
```
"Write comprehensive test suite for validation"
```

### code-reviewer

**Focus area:**
```
Review internal/events/item_moved.go:
1. Event-sourcing patterns only
2. Check: from_location validation, transaction handling
3. Critical/High issues only
```

**Don't say:**
```
"Review all the code for quality, security, performance, and style"
```

---

## Output Format Constraints

Add these to your prompts:

**For design tasks:**
- "Return 3-sentence summary + write details to single file"
- "One file maximum, under 200 lines"
- "Essential patterns only, no examples"

**For implementation:**
- "Write code + tests only (no docs)"
- "Brief summary with test results"

**For reviews:**
- "Critical issues only (skip style)"
- "3-sentence summary + detailed review file"

---

## Common Patterns

### Starting a new feature

**Phase 1 - Architecture:**
```
@golang-architect: Design architecture for [feature]:
1. Component breakdown
2. Event types needed
3. Projection changes
Output: One design doc, under 300 lines
```

**Phase 2 - Database:**
```
@db-developer: Design schema for [feature]:
1. Projection table: [table_name]
2. DDL + indexes
3. No implementation yet
Output: DDL only
```

**Phase 3 - Implementation:**
```
@golang-developer: Implement [specific handler]:
1. Function: Handle[EventType]
2. Validation: [specific rules]
3. TDD (tests first)
Output: Code + passing tests
```

**Phase 4 - CLI:**
```
@golang-ui-developer: Implement wherehouse [command]:
1. Command in cmd/[name].go
2. Flags: [list specific flags]
3. Integrate with [domain function]
Output: Code + tests
```

**Phase 5 - Review:**
```
@code-reviewer: Review [component]:
1. Focus: event-sourcing compliance
2. Critical/High issues only
Output: Brief review
```

### Each step is focused, minimal token usage

---

## Measurement

After implementing these patterns, you should see:

**Before optimization:**
- Average agent usage: ~36K tokens
- Design tasks: 50-70K tokens
- Implementation: 30-40K tokens

**After optimization:**
- Average agent usage: ~20-25K tokens (30-40% reduction)
- Design tasks: 20-30K tokens (60% reduction)
- Implementation: 15-25K tokens (40% reduction)

---

## Anti-Patterns to Avoid

❌ "Do everything related to X"
❌ "Create comprehensive documentation"
❌ "Implement the complete system for Y"
❌ "Review all aspects of the code"
❌ Asking for flowcharts, diagrams, extensive examples
❌ Combining design + implementation in one prompt
❌ No output size constraints

✅ One focused task per prompt
✅ Explicit scope boundaries
✅ Output format/size specified
✅ Design and implementation separated
✅ Code over documentation

---

**Remember:** Agents are specialists. Treat them like focused contractors, not project managers.
