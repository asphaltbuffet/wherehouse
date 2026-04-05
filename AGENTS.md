# Wherehouse Agent Guide

You are a coding agent running on a user's computer.

## History

**Run** `/resume-work` at the start of a session to pick up context from previous agents.

## ⚠️ Critical First Steps

Read the knowledge docs relevant to your task — not all of them:

| Task type | Read first |
|---|---|
| New CLI command | `ai-docs/knowledge/cli-contract.md` |
| Event or database work | `ai-docs/knowledge/business-rules.md` ("Critical Invariants" section) |
| Domain model or projection work | `ai-docs/knowledge/domain-model.md` + `ai-docs/knowledge/projections.md` |
| Event type definitions | `ai-docs/knowledge/events.md` |
| Unsure what applies | `ai-docs/knowledge/README.md` |

> `ai-docs/research/` contains design proposals, not necessarily implemented code. Always verify in `internal/` and `cmd/` directly.

### What is not yet implemented

Check here before spending tool calls confirming absence:

- **Tags/tagging**: no `tags` concept exists anywhere — no `ItemTaggedEvent`, no `tags` column, no tag table, no `cmd/tag/`.
- **TUI**: `internal/tui/` does not exist yet; `ai-docs/research/tui/` has design proposals only.
- **Projects**: `internal/database/project.go` exists but project CLI commands are not wired into `cmd/`.
- **`internal/events/` package**: does not exist — event types live in `internal/database/eventTypes.go`.

## Code Implementation

- Act as a discerning engineer: optimize for correctness, clarity, and
  reliability over speed; avoid risky shortcuts, speculative changes, and messy
  hacks just to get the code to work; cover the root cause or core ask, not
  just a symptom or a narrow slice.
- Conform to the codebase conventions: follow existing patterns, helpers,
  naming, formatting, and/or localization; if you must diverge, state why.
- Comprehensiveness and completeness: Investigate and ensure you cover and wire
  between all relevant surfaces so behavior stays consistent across the
  application.
- Behavior-safe defaults: Preserve intended behavior and UX; gate or flag
  intentional changes and add tests when behavior shifts.
- Tight error handling: no broad catches or silent defaults; propagate or
  surface errors explicitly rather than swallowing them.
- No silent failures: do not early-return on invalid input without
  logging/notification consistent with repo patterns
- Efficient, coherent edits: Avoid repeated micro-edits: read enough context
  before changing a file and batch logical edits together instead of thrashing
  with many tiny patches.
- Keep type safety: changes should always pass build and type-check; prefer
  proper types and guards over type assertions or interface{}/any casts.
- Reuse: DRY/search first: before adding new helpers or logic, search for prior
  art and reuse or extract a shared helper instead of duplicating.

## Editing constraints

- Default to ASCII when editing or creating files. Only introduce non-ASCII or
  other Unicode characters when there is a clear justification or the file
  already uses them.
- Add succinct code comments only when code is not self-explanatory. Usage
  should be rare.
- While you are working, you might notice unexpected changes that you didn't
  make. If this happens, **STOP IMMEDIATELY** and ask the user how they would
  like to proceed.

## Exploration and reading files

Maximize parallel tool calls. Batch all reads/searches; only make sequential
calls when one result determines the next query.

## `/dev` orchestrator

- Skip for straightforward tasks; no orchestration needed for single-step plans.

## Special user requests

- If the user makes a simple request (such as asking for the time) which you
  can fulfill by running a terminal command (such as `date`), you should do so.
- If the user asks for a "review", default to a code review mindset: prioritise
  identifying bugs, risks, behavioral regressions, and missing tests. Present
  findings first (ordered by severity with file/line references), follow with
  open questions, and offer a change-summary only as a secondary detail.

## Project Overview

Wherehouse is an **event-sourced** CLI/TUI inventory tracker that answers "Where did I put my 10mm socket?". Built with Go + SQLite, it uses events as source of truth with disposable projections for fast queries. Multi-user attribution only (no permissions).

The project implements event sourcing architecture where:
- **Events** are the source of truth (append-only log)
- **Projections** are derived state (rebuildable)
- **Replay** by `event_id` order ensures determinism
- **No undo** - corrections create new compensating events

## Project Structure

```
wherehouse/
├── cmd/                    # CLI commands (cobra); one subdir per command
│   ├── add/
│   ├── config/
│   ├── find/
│   ├── found/
│   ├── history/
│   ├── initialize/
│   ├── list/
│   ├── loan/
│   ├── lost/
│   ├── migrate/
│   ├── move/
│   └── scry/
├── internal/
│   ├── cli/               # Shared CLI helpers (selectors, output, flags)
│   ├── config/            # Configuration management
│   ├── database/          # SQLite: events, projections, migrations, replay
│   │   ├── eventTypes.go  # EventType iota + ParseEventType + stringer
│   │   ├── eventHandler.go # processEventInTx routing switch
│   │   ├── itemEventHandler.go
│   │   ├── locationEventHandler.go
│   │   ├── projectEventHandler.go
│   │   ├── replay.go      # Event replay engine
│   │   ├── validation.go  # Integrity checks
│   │   └── migrations/    # SQL schema migrations (golang-migrate)
│   ├── logging/           # Logging + log rotation
│   ├── nanoid/            # NanoID generation
│   ├── styles/            # lipgloss style definitions (appStyles singleton)
│   └── version/           # Build version info
├── docs/
│   └── DESIGN.md          # Full design specification
└── ai-docs/
    ├── knowledge/         # Authoritative AI agent references (read these)
    ├── research/          # Design proposals — may not be implemented
    └── sessions/          # Session plans and status
```

## Essential Commands

```bash
mise run build       # build to dist/wherehouse
mise run dev         # full pipeline: generate/mock/test/lint/snapshot
mise run test        # run all tests
mise run lint        # golangci-lint
mise run update-deps # update dependencies
mise run clean       # clean build artifacts
```

## Code Organization & Patterns

### File Layout
- Commands are in `cmd/`, one subdirectory per command
- Shared CLI helpers (selectors, output formatting, flags) in `internal/cli/`
- Database operations, event types, projections, and replay in `internal/database/`
- Configuration management in `internal/config/`
- Event types and handlers are in `internal/database/` (no separate `internal/events/` package)

### Event Sourcing Design
- Events are immutable (never modified or deleted)
- Ordering by `event_id` only (timestamps informational)
- Projections rebuildable from events (`doctor --rebuild`)
- Validation failures stop replay (no silent repair)
- System locations (`Missing`, `Borrowed`, `Loaned`) are special - immutable and predefined

### Error Handling Pattern
- Functions return `error` types for all errors  
- Uses `fmt.Errorf("...: %w", err)` for wrapping errors  
- All database operations use transactions  
- Retry logic implemented for SQL BUSY/LOCKED errors

### Database Operations
- SQLite connection managed via `internal/database/database.go` 
- Uses WAL mode for concurrent access
- Foreign key constraints enabled via PRAGMA
- Migrations handled with `golang-migrate`  
- Database access patterns with `ExecInTransaction` and `WithRetry` helpers
- Projections are derived tables that can be rebuilt from events

## Hard rules (non-negotiable)

These have been repeatedly requested. Violating them wastes the user's time.

### Skill triggers

Use these skills at the indicated times. Each skill contains full procedural
details; do not duplicate that detail here.

- `/pre-commit` -- before every `commit`
- `/commit` -- commit conventions (types, scopes, CI trigger phrases)
- `/audit-docs` -- after features or fixes

### Shell and tools

- **No `git` commands**: Use `jj` equivalents. To find a historical refactor by keyword: `jj log --no-graph -r 'description(glob:"*keyword*")' -T 'change_id ++ " " ++ description.first_line() ++ "\n"'`. To inspect what a commit changed: `jj show <change_id>`.
- **No `&&`**: Run shell commands as separate tool calls (parallel when
  independent, sequential when dependent).
- **Use `jq`, not Python, for JSON**: Use `jq` directly.
- **Treat "upstream" conceptually**: Use the repo's canonical mainline remote
  (e.g. `origin/main`) even if no `upstream` remote exists.
- **Modern CLI tools**: Use `rg` not `grep`, `fd` not `find`, `sd` not
  `sed` where possible.
- **Read deps locally**: To read a dependency's source, look in the local
  Go module cache (`go env GOMODCACHE`) instead of making web requests to
  GitHub, curl, or other alternatives.
- **Never `cd` out of the worktree**: Your cwd is the worktree root. Run
  all commands there. Never `cd` into the parent checkout or any other
  directory.

### Nix

- **Quote flake refs**: Single-quote refs containing `#` so the shell doesn't
  treat `#` as a comment (e.g. `nix shell 'nixpkgs#vhs'`).
- **Fallback priority for missing commands**: (1) `nix run '.#<tool>'`;
  (2) `nix shell 'nixpkgs#<tool>' -c <command>`;
  (3) `nix develop -c <command>`. Never declare a tool unavailable without
  trying all three.
- **Dynamic store paths**: Use
  `nix build '.#wherehouse' --print-out-paths --no-link` at runtime. Never
  hardcode `/nix/store/...` hashes.
- **Use `writeShellApplication`** not `writeShellScriptBin` for Nix shell
  scripts. Use **`pkgs.python3.pkgs`** not `pkgs.python3Packages`.
- **Nix package mappings**: `benchstat` is in `nixpkgs#goperf`.

### JJ and CI

- **Treat all warnings as errors**: Fix all warnings from
  `mise run lint`, `mise run test`, or the compiler before committing.
- **Pin Actions to version tags**: Use `@v3.93.1` not `@main`/`@latest`.
- **No `=` in CI go commands**: PowerShell misparses `=`. Use `-bench .`
  not `-bench=.`.

### Testing

- **Regression tests are strict TDD**: Write a test that reproduces the
  bug first, confirm it fails, then iterate on the fix until the test
  passes. Do not game this by wildly mutating code just to satisfy the
  test -- fix the actual root cause.
- **Use `testify/assert` and `testify/require`**: `require` for
  preconditions, `assert` for assertions. No bare `t.Fatal`/`t.Error`.
- **Test every error path**: Every function that can fail needs at least
  one test exercising that failure.
- **Prefer mocking over complicated test setup**: Tests should not require
  extensive test setups to create a full environment. If there aren't mocks
  available, ask if they can be created as part of fix.
- **`t.Skip()` is only for platform tests**: Don't skip testing because it's
  hard. Only skip if the test is specific to the system running the test (ie,
  `if GOOS != "darwin" { t.Skip("test is only for MacOS")}`)

### Architecture and code style

- **Never switch on bare integers that represent enums**: Define typed
  `iota` constants. The `exhaustive` linter catches missing cases.
- **Use stdlib/codebase constants**: No magic numbers when `math.MaxInt64`
  or a codebase constant exists.
- **Deterministic ordering requires tiebreakers**: Every `ORDER BY` that
  could tie MUST include a tiebreaker (typically `event_id ASC`). Timestamps
  use RFC3339 (1-second resolution) and are not unique — never rely on
  `timestamp_utc` alone for ordering.
- **Command constructor pattern**: Every command package exposes
  `NewXxxCmd(db xxxDB)` (for tests) and `NewDefaultXxxCmd()` (for
  production wiring). Registered in `cmd/root.go` via `NewDefaultXxxCmd()`.
- **Per-command DB interface**: Each command's `db.go` defines a minimal
  `xxxDB` interface covering only the methods that command needs, with a
  `//go:generate mockery` directive. Never pass `*database.DB` directly to
  a command's run function.
- **Event type registration**: New event types go in
  `internal/database/eventTypes.go` (add to `EventType` iota + linecomment
  string + `eventTypeByName` map), then regenerate `eventtype_string.go`
  with `go generate`, then add a case to `processEventInTx` in
  `eventHandler.go`.
- **Styles live in `appStyles`**: Add new styles as private fields on the
  `Styles` struct in `styles.go` with public accessor methods, and reference
  them via the package-level `appStyles` singleton (e.g. `appStyles.Item()`).
  If a new style duplicates an existing definition, add a method alias instead
  of a new field. Never inline `lipgloss.NewStyle()` in rendering functions --
  it defeats the singleton.

### UI/UX conventions

- **Actionable error messages**: Include the failure, likely cause, and
  a concrete remediation step on every user-facing error surface.
- **Unix aesthetic -- silence is success**: No empty-state placeholders
  or success confirmations. Only surface what requires attention by default. Show more information if the user chooses verbose `-v`/`--verbose` output. 
- **Colorblind-safe palette**: Wong palette with
  `lipgloss.AdaptiveColor{Light, Dark}`. See `styles.go`.
- **Visual consistency across outputs**: When changing a visual element's
  appearance, audit every output echoing the same semantics.

### Behavioral guardrails

- **Two-strike rule for bug fixes**: If your second attempt doesn't work,
  **STOP**. Re-read the code path end-to-end and fix the root cause.

If the user asks you to learn something, add behavioral constraints to this
"Hard rules" section, and/or create a skill in `.claude/commands/` for workflows. Update or add files in `ai-docs/knowledge` for future reference.

## Development best practices

- Pause after each stage and wait for developer input before continuing.
- Use `/commit` at logical stopping points.
- Run long commands (`mise run dev`, `mise run snapshot`) in the background.
- For big or core features, use `/dev` before writing code — session records are permanent design artifacts.
- "Refactoring" includes Go, Nix, CI workflows — don't skip non-`.go` files.

# Session log

Session history is in `ai-docs/sessions/`
