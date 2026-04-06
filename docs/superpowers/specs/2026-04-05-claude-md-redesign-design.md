# CLAUDE.md Redesign Spec

**Date**: 2026-04-05  
**Status**: Approved  
**Goal**: Replace `CLAUDE.md` (1-line redirect) + `AGENTS.md` (~300 lines) with a single, minimal `CLAUDE.md` that reduces agent exploration overhead for the five highest-frequency task types.

---

## Background

An A/B experiment measured agent exploration cost with and without `AGENTS.md` in context across three tasks:

| Task | Without (calls) | With (calls) | Without (tokens) | With (tokens) |
|---|---|---|---|---|
| CLI `--format json` flag | 15 | 14 | 38,892 | 35,089 |
| CI/Go version update | 12 | 22 | 18,897 | 25,122 |
| New Claude subagent | 7 | 10 | 20,011 | 24,408 |

`AGENTS.md` made things worse overall (+35% tool calls, +9% tokens). Root causes:
- Long file dilutes high-value content (task routing table, "what's not implemented")
- Shell-tool prescriptions add friction on tasks where patterns return no matches
- Structure map doesn't prevent agents from re-exploring via `list_dir`
- Agents loaded the full file even when only 10% was relevant

The one case where it helped: the CLI agent that found `ai-docs/knowledge/cli-contract.md` via the routing table and immediately learned `--json` already existed, saving ~5 calls.

---

## Design

### Audience

Claude Code agents only (Serena MCP available). Not tool-agnostic.

### Structure

Single `CLAUDE.md`, ~120 lines, replacing both `CLAUDE.md` and `AGENTS.md`.  
`ai-docs/knowledge/` docs are kept for deep-dive reference but not loaded by default.  
`.claude/project-config.md` is kept for `/dev` orchestrator agent routing — not folded in.

### Sections

#### 1. Identity block (~8 lines)

Project name, one-liner, build commands, VCS, preferred tools, and the maintenance instruction (agents update stale shortcuts inline).

#### 2. Task shortcuts table

The highest-ROI section. One row per task type, each with:
- **Read first**: the one file that gives the most leverage
- **Key facts**: 1-3 inline shortcuts that skip the most exploration calls

Covers the five priority task types:
- New command/subcommand
- Output format changes
- Data structures / events
- Troubleshooting
- CI / dev tooling

Followed immediately by a **"What doesn't exist yet"** subsection — prevents agents spending calls confirming absence of tags, TUI, projects CLI, `internal/events/` package.

Agents are instructed to update stale rows and add new shortcuts when they spend 3+ calls finding something that should have been inline.

#### 3. Project structure

Compact annotated directory tree. Prefaced with a Serena MCP navigation note — reinforces preference for `find_symbol` / `get_symbols_overview` / `find_referencing_symbols` over Read/Grep/Glob at the moment agents are about to start exploring.

#### 4. CI / Dev tooling

Dedicated section (absent from current `AGENTS.md` in actionable form) covering:
- Go version: `go.mod` is single source of truth; all CI workflows use `go-version-file: 'go.mod'`
- Dev shell: `flake.nix` uses bare `pkgs.go`; how to pin a specific version
- `mise.toml`: no Go pin
- Nix rules: quote flake refs, missing command fallback order, `writeShellApplication`, `nixpkgs#goperf`
- CI rules: pin Actions to version tags, no `=` in go commands, warnings as errors

#### 5. Hard rules

Non-negotiable constraints grouped by when they apply:
- **Before every commit**: `/pre-commit`, `/commit`, `/audit-docs`
- **Code**: constructor pattern, per-command DB interface, enums, ORDER BY tiebreakers, styles singleton, event type registration
- **Testing**: TDD for bug fixes, testify require/assert, test every error path, t.Skip() only for platform tests
- **UX**: silence is success, actionable errors, Wong palette
- **Debugging**: two-strike rule

Removed from current `AGENTS.md`: workflow orchestration guidance, `No &&` shell rule (lives in global CLAUDE.md), editing constraints (ASCII default), development pacing advice.

#### 6. Maintenance instructions

How agents keep the file healthy:
- Update stale shortcuts inline (don't add notes)
- Remove entries from "what doesn't exist" when they get implemented
- Add a shortcut when 3+ calls were needed to find something

---

## What Changes

| File | Action |
|---|---|
| `CLAUDE.md` | Replaced with new ~120-line file |
| `AGENTS.md` | Deleted |
| `ai-docs/knowledge/` | Kept, unchanged |
| `.claude/project-config.md` | Kept, unchanged |

---

## Success Criteria

For the three benchmark tasks, the "with" condition should use fewer tool calls than "without":
- CLI output task: target ≤10 calls (vs 15 baseline)
- CI/Go version task: target ≤8 calls (vs 12 baseline)
- New subagent task: target ≤6 calls (vs 7 baseline)

---

## Out of Scope

- Changes to `ai-docs/knowledge/` content
- Changes to `.claude/agents/` or `.claude/commands/`
- Changes to `.claude/project-config.md`
- Any agent routing or orchestration logic
