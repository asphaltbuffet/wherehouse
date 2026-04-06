# CLAUDE.md Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `CLAUDE.md` (1-line redirect) and `AGENTS.md` (~300 lines) with a single, minimal `CLAUDE.md` (~120 lines) that reduces agent exploration overhead for the five highest-frequency task types.

**Architecture:** Single `CLAUDE.md` using a task-dispatch table with inline shortcuts — each task type gets a "read first" pointer and 2-3 inline facts that skip the most common exploration calls. Hard rules and CI/tooling facts are inlined. The `ai-docs/knowledge/` deep-dive docs and `.claude/project-config.md` are kept unchanged.

**Tech Stack:** Markdown only. Verification via manual inspection against spec success criteria.

---

## File Map

| File | Action | Purpose |
|---|---|---|
| `CLAUDE.md` | Rewrite | New ~120-line file replacing both CLAUDE.md and AGENTS.md |
| `AGENTS.md` | Delete | Replaced by new CLAUDE.md |

All other files (`ai-docs/knowledge/`, `.claude/project-config.md`, `.claude/agents/`, `.claude/commands/`) are untouched.

---

### Task 1: Write the new CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Read the current CLAUDE.md and AGENTS.md to confirm nothing is missed**

```bash
cat CLAUDE.md
cat AGENTS.md
```

Cross-check the hard rules section of AGENTS.md against the spec. Verify no rule from the "Skill triggers", "Shell and tools", "Testing", "Architecture and code style", and "UI/UX conventions" sections is omitted from the new design without deliberate intent.

- [ ] **Step 2: Write the new CLAUDE.md**

Replace the entire contents of `CLAUDE.md` with:

```markdown
# Wherehouse

Event-sourced CLI inventory tracker. Go + SQLite. "Where did I put my 10mm socket?"

**Build**: `mise run dev` (full pipeline) | `mise run test` | `mise run lint` | `mise run build`
**VCS**: `jj` only — no `git` commands
**Tools**: `rg` not grep, `fd` not find, `sd` not sed, `jq` for JSON
**Agents**: when a shortcut below is stale, update it inline — don't add a note, just fix it

---

## Task Shortcuts

| Task | Read first | Key facts (verify before trusting) |
|---|---|---|
| New command/subcommand | `ai-docs/knowledge/cli-contract.md` | Pattern: `NewXxxCmd(db xxxDB)` + `NewDefaultXxxCmd()` wired in `cmd/root.go`. Per-command `db.go` defines minimal `xxxDB` interface + `//go:generate mockery`. Register in `cmd/root.go` via `NewDefaultXxxCmd()`. |
| Output format changes | `cmd/root.go`, `internal/cli/output.go` | Global `--json` flag already exists (`cmd/root.go:61`). JSON output paths are implemented in most commands. All styles via `appStyles` singleton in `internal/styles/styles.go` — never inline `lipgloss.NewStyle()` in rendering functions. |
| Data structures / events | `ai-docs/knowledge/events.md`, `ai-docs/knowledge/business-rules.md` | Event types live in `internal/database/eventTypes.go` (iota + `eventTypeByName` map). Adding a new type: add iota entry → regenerate `eventtype_string.go` (`go generate ./internal/database/`) → add case in `internal/database/eventHandler.go`. |
| Troubleshooting | `ai-docs/knowledge/business-rules.md` | Events immutable, ordered by `event_id` only (timestamps are informational, not unique). Every `ORDER BY` that could tie must include `event_id ASC/DESC` as tiebreaker. System locations (`Missing`, `Borrowed`, `Loaned`) are predefined and immutable. |
| CI / dev tooling | `go.mod`, `.github/workflows/`, `flake.nix` | All 4 CI workflows use `go-version-file: 'go.mod'` — no hardcoded Go versions. Dev shell (`flake.nix`) uses bare `pkgs.go` from nixpkgs pin. `mise.toml` has no Go version pin. |

### What doesn't exist yet (don't search for these)

- **Tags/tagging** — no `ItemTaggedEvent`, no `tags` column, no `cmd/tag/`
- **TUI** — `internal/tui/` does not exist; `ai-docs/research/tui/` has proposals only
- **Projects CLI** — `internal/database/project.go` exists but no `cmd/project/` commands
- **`internal/events/` package** — event types live in `internal/database/eventTypes.go`

When you search for something that doesn't exist and spend 3+ calls confirming it, add it here.

---

## Structure

**Navigation**: prefer Serena MCP tools over Read/Grep/Glob for code navigation.
Use `find_symbol` to locate functions, `get_symbols_overview` for a file's symbols,
`find_referencing_symbols` for callers. Use `search_for_pattern` when symbol name is
uncertain. Only read full files for non-code assets (YAML, TOML, markdown).

```
cmd/                    # CLI commands — one subdir per command
  <cmd>/
    <cmd>.go            # cobra command: NewXxxCmd(db xxxDB) + NewDefaultXxxCmd()
    db.go               # minimal xxxDB interface + //go:generate mockery
    output.go           # rendering helpers (if needed)
    <cmd>_test.go
internal/
  cli/                  # shared helpers: selectors, output formatting, flags
  config/               # config management (TOML via viper)
  database/             # SQLite: events, projections, migrations, replay
    eventTypes.go       # EventType iota + ParseEventType + eventTypeByName map
    eventHandler.go     # processEventInTx routing switch
    migrations/         # SQL schema files (golang-migrate)
  logging/              # log rotation
  nanoid/               # ID generation
  styles/               # appStyles singleton (lipgloss)
ai-docs/
  knowledge/            # authoritative reference docs (read per task-type table above)
  research/             # design proposals — may not be implemented yet
  sessions/             # session plans and status
```

---

## CI / Dev Tooling

**Go version**: `go.mod` is the single source of truth. All CI workflows use
`go-version-file: 'go.mod'` — changing the Go version means updating `go.mod` only.

**Dev shell**: `flake.nix` uses bare `pkgs.go` (version from nixpkgs pin).
To pin a specific version: change to `pkgs.go_1_XX`.

**mise**: `mise.toml` has no Go version pin — Go tooling comes from the flake.

**Nix rules**:
- Quote flake refs containing `#`: `nix shell 'nixpkgs#vhs'`
- Missing command fallback: `nix run '.#<tool>'` → `nix shell 'nixpkgs#<tool>' -c <cmd>` → `nix develop -c <cmd>`
- Use `writeShellApplication` not `writeShellScriptBin`
- `benchstat` is at `nixpkgs#goperf`; `pkgs.python3.pkgs` not `pkgs.python3Packages`

**CI rules**:
- Pin Actions to version tags (`@v3.x.x` not `@main` or `@latest`)
- No `=` in CI go commands (PowerShell misparses): use `-bench .` not `-bench=.`
- Treat all lint/test warnings as errors before committing

---

## Hard Rules

### Before every commit
- `/pre-commit` — fixes all lint/test issues first
- `/commit` — commit message conventions
- `/audit-docs` — after features or fixes

### Code
- **Constructor pattern**: every command exposes `NewXxxCmd(db xxxDB)` + `NewDefaultXxxCmd()`. Never pass `*database.DB` directly to a run function.
- **Per-command DB interface**: each `cmd/<cmd>/db.go` defines a minimal interface; `//go:generate mockery` directive required.
- **Enums**: typed `iota` constants only — never switch on bare integers (`exhaustive` linter enforces this).
- **ORDER BY tiebreakers**: every query that could tie must include `event_id ASC/DESC`.
- **Styles singleton**: add new styles to `appStyles` struct in `internal/styles/styles.go`; never inline `lipgloss.NewStyle()`.
- **Event type registration**: add iota → regenerate `eventtype_string.go` → add case in `eventHandler.go`.

### Testing
- **TDD for bug fixes**: write a failing test first, confirm it fails, then fix. Don't game it — fix the root cause.
- **testify**: `require` for preconditions (test stops on fail), `assert` for assertions (test continues).
- **Error paths**: every function that can fail needs at least one test exercising that failure.
- **`t.Skip()`**: only for platform-specific tests (`if runtime.GOOS != "darwin"`). Never skip because it's hard.

### UX
- **Silence is success**: no empty-state placeholders or success confirmations by default. Surface more with `-v`/`--verbose`.
- **Actionable errors**: every user-facing error must include what failed, likely cause, and a remediation step.
- **Colorblind-safe**: Wong palette via `lipgloss.AdaptiveColor{Light: "...", Dark: "..."}`. See `internal/styles/styles.go`.

### Debugging
- **Two-strike rule**: if your second fix attempt fails, stop. Re-read the full code path end-to-end before trying again.

---

## Maintaining this file

- When a shortcut in the Task Shortcuts table is stale: update the row inline
- When a "what doesn't exist" entry gets implemented: remove it
- When you spend 3+ calls finding something that should have been a shortcut: add it
- Keep every entry terse enough to scan in one line — no prose explanations
```

- [ ] **Step 3: Verify the file is ~120 lines and scans well**

```bash
wc -l CLAUDE.md
```

Expected: 110–130 lines. If significantly over, look for prose that can be tightened.

- [ ] **Step 4: Commit**

```bash
jj commit -m "docs: rewrite CLAUDE.md as minimal task-dispatch guide"
```

---

### Task 2: Delete AGENTS.md

**Files:**
- Delete: `AGENTS.md`

- [ ] **Step 1: Confirm AGENTS.md content is fully covered**

Read through the AGENTS.md hard rules one more time and confirm each has a home:

- Skill triggers (`/pre-commit`, `/commit`, `/audit-docs`) → Hard Rules > Before every commit ✓
- Shell tools (`rg`, `fd`, `sd`, `jq`, `jj`) → Identity block ✓
- Nix rules → CI / Dev Tooling ✓
- CI rules → CI / Dev Tooling ✓
- Testing rules → Hard Rules > Testing ✓
- Architecture/code style → Hard Rules > Code ✓
- UI/UX conventions → Hard Rules > UX ✓
- Two-strike rule → Hard Rules > Debugging ✓
- "What is not yet implemented" → Task Shortcuts > What doesn't exist yet ✓
- Project structure → Structure section ✓
- Essential commands → Identity block ✓
- Task-type routing table → Task Shortcuts table ✓

Items intentionally dropped (not in new CLAUDE.md):
- Workflow orchestration guidance (`/dev` skip rules) — lives in `.claude/project-config.md`
- `No &&` shell rule — lives in user-level `~/.claude/CLAUDE.md`
- Editing constraints (ASCII default) — rarely triggered, not worth the noise
- Development pacing advice (pause after each stage) — orchestration concern, not agent concern
- Session log pointer — agents find `ai-docs/sessions/` via structure map

- [ ] **Step 2: Delete AGENTS.md**

```bash
jj file forget AGENTS.md
rm AGENTS.md
```

- [ ] **Step 3: Verify deletion and commit**

```bash
jj st
```

Expected output includes `D AGENTS.md`.

```bash
jj commit -m "chore: remove AGENTS.md (consolidated into CLAUDE.md)"
```

---

### Task 3: Verify

**Files:** none modified

- [ ] **Step 1: Confirm CLAUDE.md is the only agent guide at repo root**

```bash
ls *.md
```

Expected: `CLAUDE.md` present, `AGENTS.md` absent.

- [ ] **Step 2: Spot-check the five task-type shortcuts**

For each row in the Task Shortcuts table, verify the "Read first" file exists and the "Key facts" are accurate:

```bash
# New command/subcommand
ls ai-docs/knowledge/cli-contract.md
rg "NewXxxCmd|NewDefaultXxxCmd" cmd/root.go

# Output format changes
rg "json" cmd/root.go | head -5
ls internal/styles/styles.go

# Data structures / events
rg "eventTypeByName" internal/database/eventTypes.go
rg "processEventInTx" internal/database/eventHandler.go

# CI / dev tooling
rg "go-version-file" .github/workflows/build.yml
rg "go-version-file" .github/workflows/golangci-lint.yml
```

- [ ] **Step 3: Spot-check "what doesn't exist" entries**

```bash
fd cmd/tag       # should return nothing
fd internal/tui  # should return nothing
fd cmd/project   # should return nothing
fd internal/events # should return nothing
```

- [ ] **Step 4: Confirm line count is reasonable**

```bash
wc -l CLAUDE.md
```

Expected: 110–130 lines.

- [ ] **Step 5: Final commit if any fixes were needed, otherwise done**

If the spot-checks found stale facts and you corrected them:

```bash
jj commit -m "docs: fix stale shortcuts in CLAUDE.md"
```

If everything was accurate, no commit needed — the previous two commits are the complete change.
