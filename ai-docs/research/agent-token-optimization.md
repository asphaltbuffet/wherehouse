# Agent System Token Usage Optimization

**Date**: 2026-02-20
**Purpose**: Reduce token usage in the 6-agent system for wherehouse

---

## Current Token Usage Analysis

### Test Results (Actual Usage)

| Agent | Tokens Used | Tool Calls | Duration | Notes |
|-------|-------------|------------|----------|-------|
| golang-developer | 35,155 | 22 | 222s | Moderate |
| db-developer | 45,021 | 5 | 32s | High for simple task |
| **golang-ui-developer** | **63,513** | 19 | 347s | **HIGHEST** - created 9 docs |
| golang-tester | 29,545 | 7 | 62s | Reasonable |
| code-reviewer | 19,564 | 0 | 19s | **LOWEST** - review only |
| golang-architect | ~25,000 (est) | N/A | N/A | From previous session |

**Average**: ~36,300 tokens per agent invocation
**Total for all 6**: ~218,000 tokens

### Agent Definition Sizes

| Agent | Words | Est. Tokens | % of Total |
|-------|-------|-------------|------------|
| db-developer | 3,774 | ~4,700 | 23% |
| code-reviewer | 2,952 | ~3,700 | 18% |
| golang-tester | 2,989 | ~3,700 | 18% |
| golang-developer | 2,628 | ~3,300 | 16% |
| golang-ui-developer | 2,561 | ~3,200 | 16% |
| golang-architect | 1,351 | ~1,700 | 9% |
| **Total** | **16,255** | **~20,300** | **100%** |

### Knowledge Base Size

| File | Words | Est. Tokens |
|------|-------|-------------|
| events.md | 1,887 | ~2,400 |
| architecture.md | 1,777 | ~2,200 |
| business-rules.md | 1,754 | ~2,200 |
| projections.md | 1,658 | ~2,100 |
| domain-model.md | 1,203 | ~1,500 |
| critical-constraints.md | 1,044 | ~1,300 |
| README.md | 971 | ~1,200 |
| cli-contract.md | 429 | ~540 |
| **Total** | **10,723** | **~13,440** |

---

## Root Causes of High Token Usage

### 1. Verbose Agent Definitions (~20K tokens baseline)

**Repeated boilerplate across all agents:**
- **Anti-recursion section**: ~150 lines per agent (identical across all 6)
- **Output format/return protocol**: ~100 lines per agent (nearly identical)
- **Context economy section**: ~80 lines per agent (identical)
- **Scope definition with examples**: ~50 lines per agent
- **Quality checklists**: ~40 lines per agent

**Impact**: ~420 lines of repetitive content per agent × 6 agents = ~2,500 lines of duplicate content

### 2. Excessive Implementation Patterns

**db-developer (longest at 868 lines):**
- 6 full code examples (~300 lines total)
- Query pattern (~80 lines)
- Transaction pattern (~55 lines)
- Migration pattern (~95 lines)
- Connection setup (~30 lines)
- Error handling (~35 lines)
- Testing pattern (~50 lines)

**golang-ui-developer (545 lines):**
- 4 full code examples (~250 lines total)
- Command pattern (~50 lines)
- Flag handling (~40 lines)
- Output formatting (~50 lines)
- Selector parsing (~15 lines)

**Impact**: Code examples account for ~35-40% of agent definition tokens

### 3. Over-Engineering by golang-ui-developer

During test, golang-ui-developer created:
- 9 separate documentation files
- ~100K characters total
- Flowcharts, comprehensive examples, test strategies

**For a simple test task** (design --tags flag pattern), this was massive overkill.

**Impact**: 63,513 tokens used (175% more than average)

### 4. Knowledge Base Loading

Agents may load knowledge base files they don't need:
- CLAUDE.md is always loaded (~2,000 tokens)
- Some agents likely load all knowledge files
- No selective loading based on task

**Impact**: +13K tokens if entire knowledge base loaded

### 5. Model Selection

| Agent | Model | Cost Multiplier |
|-------|-------|-----------------|
| golang-architect | opus | 1.0x |
| golang-developer | sonnet | 0.2x |
| db-developer | opus | 1.0x |
| golang-ui-developer | haiku | 0.05x |
| golang-tester | haiku | 0.05x |
| code-reviewer | opus | 1.0x |

- 3 agents use opus (expensive but capable)
- 2 agents use haiku (cheap but less capable)
- 1 agent uses sonnet (balanced)

**golang-ui-developer uses haiku but consumed most tokens** - model choice didn't help!

---

## Optimization Recommendations

### Priority 1: Reduce Agent Definition Boilerplate (HIGH IMPACT)

**Problem**: ~2,500 lines of duplicate content across agents
**Target**: Reduce agent definitions by 40-50%

#### Solution 1A: Extract Common Sections to Shared File

Create `.claude/agents/_shared.md`:
```markdown
## Standard Agent Protocols

### Anti-Recursion Rule
[Single copy of anti-recursion instructions]

### Output Format & Context Economy
[Single copy of return protocol]

### Quality Standards
[Shared quality checks]
```

Then reference in agent definitions:
```markdown
---
name: db-developer
description: |
  [Scope description only]
includes:
  - _shared.md
---

[Agent-specific instructions only]
```

**Estimated Savings**: ~8K tokens per agent (40% reduction)

#### Solution 1B: Drastically Simplify Anti-Recursion Section

Current: ~150 lines of self-awareness checks
Target: ~20 lines

```markdown
## Anti-Recursion Rule

YOU ARE the {agent-name} agent. Do NOT use Task tool to invoke yourself.

Delegate to OTHER agent types only:
- golang-developer → can delegate to golang-architect, db-developer, golang-tester, code-reviewer
- db-developer → can delegate to golang-architect, golang-developer, code-reviewer
[etc]
```

**Estimated Savings**: ~2K tokens per agent

#### Solution 1C: Simplify Output Format Section

Current: ~100 lines with examples
Target: ~30 lines

```markdown
## Output Format

Return format (max 5 sentences):
```
# {Work Type} Complete

Status: {Success/Failed}
{One-line summary}
{Key metrics}
Details: {file-path}
```

Write full details to files in `ai-docs/sessions/` or `ai-docs/research/`.
```

**Estimated Savings**: ~1.5K tokens per agent

### Priority 2: Remove or Reference Code Examples (MEDIUM IMPACT)

**Problem**: Code examples account for ~7K tokens per agent
**Target**: Reduce by 80%

#### Solution 2A: Link to Example Repository

Instead of including full code examples, reference external files:
```markdown
## Implementation Patterns

See examples in `docs/patterns/`:
- database-query-pattern.go
- transaction-pattern.go
- cli-command-pattern.go

Key principles:
1. Always use prepared statements
2. Defer tx.Rollback()
3. Handle NULL with sql.NullString
```

**Estimated Savings**: ~5K tokens per agent with code examples

#### Solution 2B: Provide Minimal Pattern Snippets

Instead of 80-line examples, show 5-line patterns:
```markdown
## Database Query Pattern

```go
// Key pattern: prepared statements + deferred close
rows, err := db.Query("SELECT id FROM items WHERE location_id = ?", locID)
if err != nil { return err }
defer rows.Close()
// ... scan rows ...
```

**Estimated Savings**: ~4K tokens per agent

### Priority 3: Fix golang-ui-developer Over-Engineering (HIGH IMPACT)

**Problem**: 63K tokens for simple design task
**Target**: Reduce to ~25K tokens (60% reduction)

#### Solution 3A: Add Output Constraints

Add to golang-ui-developer definition:
```markdown
## Output Guidelines

For DESIGN tasks (not implementation):
- Create 1-2 core files max (design doc + examples)
- Limit to 500-1000 lines total
- Focus on essentials: flags, parsing, validation, output modes

For IMPLEMENTATION tasks:
- Write actual Go code + tests
- Create 2-3 files (code, tests, summary)
```

#### Solution 3B: Use haiku → sonnet for Complex Tasks

golang-ui-developer uses haiku (cheapest), but it generated way too much documentation.

Change to:
```yaml
model: sonnet
```

Sonnet is better at following constraints and produces more focused output.

**Estimated Impact**: 20-30% reduction in output verbosity

### Priority 4: Selective Knowledge Base Loading (MEDIUM IMPACT)

**Problem**: Agents may load 13K tokens of knowledge unnecessarily
**Target**: Load only relevant files

#### Solution 4A: Document Which Agents Need Which Files

| Agent | Required Knowledge Files |
|-------|--------------------------|
| golang-architect | architecture.md, domain-model.md |
| golang-developer | events.md, business-rules.md, domain-model.md |
| db-developer | projections.md, events.md, business-rules.md |
| golang-ui-developer | cli-contract.md, domain-model.md |
| golang-tester | business-rules.md (for test cases) |
| code-reviewer | business-rules.md, architecture.md |

Add to each agent definition:
```markdown
## Required Reading

Before starting work, read:
- `.claude/knowledge/business-rules.md` - Critical invariants
- `.claude/knowledge/events.md` - Event schemas

Do NOT read other knowledge files unless specifically needed.
```

**Estimated Savings**: ~5-8K tokens per agent

#### Solution 4B: Inline Critical Constraints

Instead of referencing business-rules.md (1,754 words), inline just the critical rules:
```markdown
## Critical Business Rules

1. Events are immutable (never modify)
2. Order by event_id only (not timestamps)
3. No colons in item/project names
4. Validate from_location before move events
5. Projections are disposable

Full rules: `.claude/knowledge/business-rules.md` (read only if needed)
```

**Estimated Savings**: ~1.5K tokens per agent

### Priority 5: Optimize Model Selection (LOW-MEDIUM IMPACT)

**Current:**
- opus: golang-architect, db-developer, code-reviewer (3)
- sonnet: golang-developer (1)
- haiku: golang-ui-developer, golang-tester (2)

#### Solution 5A: Use Sonnet for Most Agents

Recommended changes:
```yaml
golang-architect: opus → sonnet   # Still needs design capability
golang-developer: sonnet (keep)   # Good balance
db-developer: opus → sonnet       # Sonnet can handle DB design
golang-ui-developer: haiku → sonnet  # Better constraint following
golang-tester: haiku (keep)       # Simple task, haiku works
code-reviewer: opus (keep)        # Needs deep analysis
```

**Impact on quality**: Minimal - sonnet is highly capable
**Impact on cost**: 80% cost reduction for 2 agents switching from opus

#### Solution 5B: Use Task-Based Model Selection

Instead of hardcoding model, select based on task complexity:
- Simple tasks (< 500 lines code): haiku
- Medium tasks (design, implementation): sonnet
- Complex tasks (architecture, review): opus

**This requires user prompt tuning rather than agent config changes.**

### Priority 6: User Prompt Optimization (HIGH IMPACT, NO CHANGES NEEDED)

**Problem**: User prompts might be too vague or complex
**Target**: Give agents clear, focused tasks

#### Recommendation 6A: Be Specific

❌ Bad: "Design a tags system"
✅ Good: "Design database schema for item tags: (1) many-to-many table, (2) DDL only, (3) no implementation"

❌ Bad: "Implement validation"
✅ Good: "Implement IsValidItemName(name string) error function. Check: (1) not empty, (2) no colons. Write tests."

**Impact**: 20-30% reduction in token usage from agents exploring/clarifying

#### Recommendation 6B: Limit Scope Explicitly

Add to prompts:
- "Design only, no implementation"
- "One file maximum, under 200 lines"
- "Essential patterns only, no examples"

**Impact**: Prevents over-engineering like golang-ui-developer's 9-file output

#### Recommendation 6C: Specify Output Format

Add to prompts:
- "Return 3-sentence summary only"
- "Write details to single file at [path]"
- "No code examples in output, just principles"

**Impact**: 30-40% reduction in response verbosity

---

## Implementation Priority

### Immediate (Do First)

1. **Simplify Anti-Recursion Section** (Priority 1B)
   - Change: All 6 agent definitions
   - Effort: 30 minutes
   - Impact: -2K tokens per agent = -12K total

2. **Simplify Output Format Section** (Priority 1C)
   - Change: All 6 agent definitions
   - Effort: 20 minutes
   - Impact: -1.5K tokens per agent = -9K total

3. **Add Output Constraints to golang-ui-developer** (Priority 3A)
   - Change: 1 agent definition
   - Effort: 10 minutes
   - Impact: -20K tokens for that agent

4. **User Prompt Guidelines Document** (Priority 6)
   - Change: Create user guide
   - Effort: 20 minutes
   - Impact: 20-30% reduction through better prompts

**Total Immediate Impact**: ~40K tokens saved (~20% reduction)

### Short Term (Next)

5. **Remove/Reference Code Examples** (Priority 2B)
   - Change: db-developer, golang-ui-developer definitions
   - Effort: 1 hour
   - Impact: -8K tokens combined

6. **Selective Knowledge Base Loading** (Priority 4A)
   - Change: All agent definitions + documentation
   - Effort: 45 minutes
   - Impact: -6K tokens per agent (when less needed)

7. **Change golang-ui-developer to sonnet** (Priority 3B + 5A)
   - Change: Model selection in definitions
   - Effort: 5 minutes
   - Impact: Better constraint following, 30% less verbose

**Total Short Term Impact**: Additional 15K tokens saved

### Long Term (Consider)

8. **Extract Common Sections** (Priority 1A)
   - Change: Major refactor of all definitions
   - Effort: 3-4 hours
   - Impact: -8K tokens per agent = -48K total
   - Risk: Requires testing Claude Code's support for includes

9. **Task-Based Model Selection** (Priority 5B)
   - Change: Workflow system
   - Effort: Significant development
   - Impact: Optimal model selection per task

---

## Expected Outcomes

### After Immediate Changes
- **Agent definitions**: 20K → 12K tokens (40% reduction)
- **Average agent invocation**: 36K → 25K tokens (30% reduction)
- **Estimated cost savings**: 30-35% per agent invocation

### After Short Term Changes
- **Agent definitions**: 12K → 8K tokens (60% total reduction)
- **Average agent invocation**: 25K → 18K tokens (50% reduction)
- **Estimated cost savings**: 45-50% per agent invocation

### After Long Term Changes
- **Agent definitions**: 8K → 5K tokens (75% total reduction)
- **Average agent invocation**: 18K → 12K tokens (65% reduction)
- **Estimated cost savings**: 60-65% per agent invocation

---

## Testing Strategy

After making changes:

1. **Re-run agent tests** (same tasks as original)
2. **Compare token usage** (before/after)
3. **Verify quality maintained** (outputs still correct)
4. **Check edge cases** (agents still refuse out-of-scope work)

**Success Criteria**:
- ✅ Token usage reduced by target %
- ✅ All agents still pass original tests
- ✅ Output quality maintained or improved
- ✅ No functionality regressions

---

## Monitoring Recommendations

Track per-agent metrics:
```yaml
agent_invocation:
  agent_name: string
  task_type: string
  tokens_used: int
  tool_calls: int
  duration_ms: int
  success: boolean
```

Alert if:
- Agent uses > 50K tokens for simple task
- Agent creates > 5 files for design task
- Agent takes > 300s for medium task

---

## Conclusion

**Primary Issue**: Verbose agent definitions with repeated boilerplate (~20K tokens baseline)

**Quick Wins** (30% reduction):
1. Simplify anti-recursion sections
2. Simplify output format sections
3. Add output constraints to golang-ui-developer
4. Improve user prompting

**Best ROI**: Focus on immediate changes first (1 hour work for 30-40% savings)

**Biggest Problem Agent**: golang-ui-developer (63K tokens) - fix with output constraints + model change

**Long Term**: Extract common sections to shared file (requires Claude Code feature support)
