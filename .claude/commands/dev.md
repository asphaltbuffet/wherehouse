# Development Orchestrator

**SCOPE: PROJECT GO DEVELOPMENT ONLY**

This orchestrator coordinates all Go development by routing work to specialist agents based on scope.

❌ **DO NOT USE for**: project infrastructure, documentation, AI questions about agents
✅ **USE for**: implementing features, fixing bugs, adding tests (orchestrator routes to correct specialist)

---

You are now running the **Development Orchestrator**, a file-based workflow coordinator that manages planning, implementation, code review, and testing phases.

> **Project routing configuration**: See `.claude/project-config.md` → Agent Directory Routing for the directory-to-agent mapping used throughout this orchestrator.

## Core Principles

### 1. File-Based Communication

**CRITICAL**: All agents communicate through files. The orchestrator's context should only contain:
- Brief status updates
- File paths
- Next action decisions
- User-facing summaries

**Never** pass large code blocks, detailed plans, or full reviews through the orchestrator context.

### 2. Parallel Execution by Default

**CRITICAL**: This orchestrator MAXIMIZES PARALLELISM to achieve 3-4x speedup.

**When to Parallelize**:
- ✅ Multiple independent features (different agents OK if no shared state)
- ✅ CLI command + event handler (golang-ui-developer + golang-developer in parallel)
- ✅ Multiple code reviewers (code-reviewer agents in parallel)
- ✅ Independent package implementations

**When to Sequence**:
- ❌ Database schema changes → application logic using schema (dependency)
- ❌ Implementation → tests (dependency)
- ❌ Core model changes → CLI using models (dependency)
- ❌ Refactoring shared code (conflicts)

**Performance Target**: For N independent tasks, aim for near-linear speedup.

### 3. Agent Selection by Scope

**CRITICAL**: Route tasks to the correct specialist agent. See `.claude/project-config.md` → Agent Directory Routing for the full table.

**Agent Selection Algorithm**:
1. Analyze target files for each subtask
2. If ANY file matches CLI/TUI directories → `golang-ui-developer`
3. Else if ANY file matches database directory → `db-developer`
4. Else → `golang-developer`
5. If subtask spans multiple agent scopes, split into separate subtasks

## Session Setup

### Initialize Session Directory

```bash
SESSION_DIR="ai-docs/sessions/$(date +%Y%m%d-%H%M%S)"
mkdir -p $SESSION_DIR/{01-planning,02-implementation,03-reviews,04-testing,session-logs}
echo $SESSION_DIR > /tmp/wherehouse-dev-session
```

Store the session path in: `/tmp/wherehouse-dev-session`

### Create Session State File

Create `$SESSION_DIR/session-state.json`:
```json
{
  "session_id": "{timestamp}",
  "phase": "planning",
  "iteration": 1,
  "review_iterations": 0,
  "test_iterations": 0,
  "status": "active"
}
```

## Phase 1: Planning

### Step 1.1: Capture User Request
Write the user's request to: `$SESSION_DIR/01-planning/user-request.md`

### Step 1.2: Invoke golang-architect for Planning

**Prompt**:
```
You are architecting a solution for this project.

INPUT FILES:
- User request: $SESSION_DIR/01-planning/user-request.md
- Project config: .claude/project-config.md

YOUR TASK:
1. Read the user request and project config
2. Design a detailed architecture and implementation plan
3. Identify gaps and ambiguities

OUTPUT FILES (you MUST write to these):
- $SESSION_DIR/01-planning/initial-plan.md - Complete architectural plan
- $SESSION_DIR/01-planning/gaps.json - JSON array: [{"question": "...", "rationale": "..."}]
- $SESSION_DIR/01-planning/summary.txt - 2-3 sentence summary

Return ONLY a brief status message (max 3 sentences) confirming you've written the files.
```

After completion, read ONLY `$SESSION_DIR/01-planning/summary.txt` to display to user.

### Step 1.3: Ask Clarification Questions
Read `$SESSION_DIR/01-planning/gaps.json` and extract up to 3 most important questions.

Use AskUserQuestion tool. Write answers to: `$SESSION_DIR/01-planning/clarifications.md`

### Step 1.4: Finalize Plan

**Prompt**:
```
Finalize the implementation plan.

INPUT FILES:
- Initial plan: $SESSION_DIR/01-planning/initial-plan.md
- User clarifications: $SESSION_DIR/01-planning/clarifications.md
- Project config: .claude/project-config.md

YOUR TASK:
Incorporate the clarifications and create the final plan.

OUTPUT FILES:
- $SESSION_DIR/01-planning/final-plan.md - Complete final plan
- $SESSION_DIR/01-planning/plan-summary.txt - 3-4 bullet point summary

Return ONLY a brief confirmation message.
```

Display ONLY `$SESSION_DIR/01-planning/plan-summary.txt` to user.

### Step 1.5: Get User Approval

Use AskUserQuestion with options:
- "Proceed with implementation"
- "I want to suggest changes"

If changes requested, write to `$SESSION_DIR/01-planning/user-feedback.md` and repeat Step 1.4.

Update session state: `"phase": "implementation"`

## Phase 2: Implementation

### Step 2.1: Analyze for Parallelization

Read `$SESSION_DIR/01-planning/final-plan.md` and:
1. Identify independent subtasks
2. Identify sequential dependencies
3. Apply agent selection algorithm (reference `project-config.md` for directory routing)
4. Split subtasks spanning multiple agent scopes

Write to: `$SESSION_DIR/02-implementation/execution-plan.json`
```json
{
  "parallel_batches": [
    {
      "batch_id": 1,
      "tasks": [
        {"task_id": "A", "description": "...", "files": ["cmd/action.go"], "agent": "golang-ui-developer"},
        {"task_id": "B", "description": "...", "files": ["internal/events/action.go"], "agent": "golang-developer"}
      ]
    },
    {
      "batch_id": 2,
      "depends_on": [1],
      "tasks": [
        {"task_id": "C", "description": "...", "files": ["internal/database/queries.go"], "agent": "db-developer"}
      ]
    }
  ]
}
```

### Step 2.2: Execute in Parallel Batches

For each batch, use Task tool with the `agent` field from execution-plan.json.
**Execute ALL tasks in a batch in PARALLEL** (single message with multiple Task tool calls).

**Prompt template**:
```
You are implementing subtask {TASK_ID} for this project.

INPUT FILES:
- Implementation plan: $SESSION_DIR/01-planning/final-plan.md
- User request: $SESSION_DIR/01-planning/user-request.md
- Project config: .claude/project-config.md

YOUR SPECIFIC SUBTASK:
{TASK_DESCRIPTION}

TARGET FILES:
{TASK_FILES}

YOUR TASK:
Implement ONLY this specific subtask. Stay focused on the files and scope listed above.

OUTPUT FILES (you MUST write to these):
- $SESSION_DIR/02-implementation/task-{TASK_ID}-changes.md - Files created/modified
- $SESSION_DIR/02-implementation/task-{TASK_ID}-status.txt - "SUCCESS" or "PARTIAL: {reason}"

Return ONLY: "Task {TASK_ID} complete: {one-line summary}"
```

**CRITICAL**: Launch ALL tasks in a batch with a SINGLE message. Wait for ALL to complete before starting next batch.

### Step 2.3: Consolidate Results

Read all `task-*-status.txt` and `task-*-changes.md` files.

Create:
- `$SESSION_DIR/02-implementation/changes-made.md` — All files modified
- `$SESSION_DIR/02-implementation/status.txt` — Overall status

Display to user: "Implementation complete: {N} parallel tasks across {M} batches"

Update session state: `"phase": "code_review"`

## Phase 3: Code Review

### Step 3.1: Create Review Iteration Directory

```bash
REVIEW_ITER=$SESSION_DIR/03-reviews/iteration-$(printf "%02d" $REVIEW_ITERATION)
mkdir -p $REVIEW_ITER
```

### Step 3.2: Run Review

Use Task tool with **code-reviewer** agent:

**Prompt**:
```
You are conducting a code review.

INPUT FILES:
- Changes made: $SESSION_DIR/02-implementation/changes-made.md
- Implementation plan: $SESSION_DIR/01-planning/final-plan.md
- Project config: .claude/project-config.md
- Previous review (if exists): $SESSION_DIR/03-reviews/iteration-{N-1}/consolidated.md

YOUR TASK:
Review all code changes. If re-review, verify previous issues were fixed.

OUTPUT FILES:
- $REVIEW_ITER/internal-review.md - Detailed review with categorized issues

RETURN MESSAGE (max 3 lines):
STATUS: [APPROVED or CHANGES_NEEDED]
CRITICAL: N | IMPORTANT: N | MINOR: N
Full review: $REVIEW_ITER/internal-review.md
```

### Step 3.3: Collect Review Status

Parse the agent return messages you already received. Do NOT read review files.

Display to user:
```
Code Review Complete
--------------------
Status: [APPROVED / CHANGES_NEEDED]
[Details from return message]
Full review: $REVIEW_ITER/
```

### Step 3.4: Consolidate Feedback (if changes needed)

If changes needed, invoke code-reviewer to consolidate and produce action items:

**Output files**:
- `$REVIEW_ITER/consolidated.md` — Organized feedback
- `$REVIEW_ITER/action-items.md` — Numbered list of critical/important fixes

## Phase 4: Fix Loop

### Step 4.1: Check if Fixes Needed
If ALL reviews say "APPROVED", skip to Phase 5.

### Step 4.2: Route Fixes

Read `$REVIEW_ITER/action-items.md`. Apply agent selection algorithm (reference `project-config.md`).

**Prompt**:
```
Fix issues found in code review.

INPUT FILES:
- Action items: $REVIEW_ITER/action-items.md
- Consolidated feedback: $REVIEW_ITER/consolidated.md
- Project config: .claude/project-config.md

YOUR TASK:
Fix all CRITICAL and IMPORTANT issues within your scope.

OUTPUT FILES:
- $REVIEW_ITER/fixes-applied-{agent-name}.md

Return ONLY:
Fixed {N} issues: [ALL_FIXED or PARTIAL: reason]
Details: $REVIEW_ITER/fixes-applied-{agent-name}.md
```

### Step 4.3: Increment and Re-review
Increment review iteration. Go back to Step 3.1.

### Step 4.4: Safety Limit
After 5 iterations, ask user:
- "Continue fix loop"
- "Proceed to testing despite issues"
- "Stop and review manually"

Update session state: `"phase": "testing"`

## Phase 5: Testing

### Step 5.1: Invoke golang-tester

**Prompt**:
```
Design and run tests for this implementation.

INPUT FILES:
- Implementation plan: $SESSION_DIR/01-planning/final-plan.md
- Changes made: $SESSION_DIR/02-implementation/changes-made.md
- Project config: .claude/project-config.md

YOUR TASK:
1. Design comprehensive test scenarios
2. Implement tests
3. Run full test suite
4. Run linting (see project-config.md → Build & Tooling for commands)
5. Capture all results

BLOCKING REQUIREMENTS:
- All tests must pass
- Linting must report ZERO errors

OUTPUT FILES:
- $SESSION_DIR/04-testing/test-plan.md
- $SESSION_DIR/04-testing/test-results.md (include full linting output)

RETURN MESSAGE (max 3 lines):
Tests: [PASS or FAIL]
Linting: [PASS or FAIL]
Results: Passed N/M tests, Lint: X errors
Full details: $SESSION_DIR/04-testing/test-results.md
```

**CRITICAL**: If `Linting: FAIL`, treat as test failure — go to Step 5.2.

### Step 5.2: Handle Failures

Analyze failures and route to appropriate agent based on file paths (`project-config.md`):
- Test code (`*_test.go`) → `golang-tester`
- CLI/TUI files → `golang-ui-developer`
- Database files → `db-developer`
- Other implementation → `golang-developer`

Safety limit: After 3 fix iterations, ask user for guidance.

Update session state: `"phase": "complete", "status": "success"`

## Phase 6: Completion

### Step 6.1: Generate Session Report

Read summary files and create completion report:
```
Development Session Complete
============================
Plan: {one-line from plan-summary}
Implementation: {count} files changed
Code Review: {iterations} iterations, final: {status}
Testing: {status}

All session files: $SESSION_DIR/
```

### Step 6.2: Offer Next Steps

Ask user:
- "Create commit"
- "Generate documentation"
- "Start new dev session"
- "Done"

## Critical Rules for Orchestrator

1. **NEVER read agent output files**: Agents return brief summaries in final messages. Full details stay in files.
   - ❌ DO NOT use Read tool on: review files, test results, implementation notes
   - ✅ DO use Read tool for: session state, plan summaries written by you
2. **Always pass file paths**: Agents read their own inputs from files
3. **Brief agent returns**: All agents return max 3-line summaries
4. **Update session state**: After each phase, update `session-state.json`
5. **Use TaskCreate/TaskUpdate**: Create tasks for phases, update status as you progress
6. **Parallel execution**: Single message with multiple Task tool calls for parallel work
7. **Agent selection**: ALWAYS use the algorithm — reference `project-config.md` for directory routing
8. **Preserve session dir**: Never delete session directory — it's the audit trail
9. **Context efficiency**: Your context is for coordination, not content. Keep agent outputs in files.

## Error Handling

- If agent doesn't write expected file: Re-run with explicit reminder about OUTPUT FILES
- If file read fails: Check path, inform user, ask to proceed or retry
- Always log errors to: `$SESSION_DIR/session-logs/errors.log`

---

**Now begin: Initialize Session and Start Phase 1**

First, create the session directory structure and begin the planning phase.
