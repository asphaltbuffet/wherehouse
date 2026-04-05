# Project Configuration
> Edit **only this file** when reusing agent/command files in a new project.
> All agents and commands reference this file for project-specific settings.

---

## Project Identity

- **Name**: wherehouse
- **Description**: Event-sourced CLI/TUI inventory tracker built with Go + SQLite
- **One-liner**: "Where did I put my 10mm socket?"

---

## Agent Directory Routing

Used by the `/dev` orchestrator and all agents to determine scope.

| Agent | Owns these paths |
|-------|-----------------|
| `golang-ui-developer` | `cmd/`, `internal/tui/` |
| `db-developer` | `internal/database/` |
| `golang-developer` | `pkg/`, `internal/` (excluding `cmd/`, `internal/tui/`, `internal/database/`) |
| `golang-tester` | `**/*_test.go`, all packages for test runs |
| `golang-architect` | All directories (design only, no implementation) |
| `code-reviewer` | All Go source (`cmd/`, `pkg/`, `internal/`) |

**Routing algorithm** (for `/dev` orchestrator):
1. Any file in `cmd/` or `internal/tui/` → `golang-ui-developer`
2. Any file in `internal/database/` → `db-developer`
3. Otherwise → `golang-developer`
4. If a subtask spans multiple scopes, split it

---

## Technology Stack

- **Language**: Go
- **Database**: SQLite (driver: `modernc.org/sqlite`)
- **CLI framework**: cobra (`spf13/cobra`)
- **Config format**: TOML (`spf13/viper`)
- **Terminal styling**: lipgloss (`charmbracelet/lipgloss`)
- **ID generation**: nanoid (`internal/nanoid`)
- **Test framework**: testify (`require` for preconditions, `assert` for assertions)
- **Migration tool**: golang-migrate
- **Task runner**: mise

---

## Build & Tooling Commands

| Purpose | Command |
|---------|---------|
| Full pipeline (generate/test/lint/build) | `mise run dev` |
| Build only | `mise run build` |
| Test only | `mise run test` |
| Lint only | `mise run lint` |
| Update dependencies | `mise run update-deps` |
| Clean artifacts | `mise run clean` |

---

## Version Control

- **VCS**: jj (Jujutsu) — **no `git` commands**
- **Main branch**: `main`
- **Commit style**: conventional commits (see `/commit` command)

---

## Knowledge Base

All domain reference documents live here:

| Document | Path |
|----------|------|
| Index / where to look | `ai-docs/knowledge/knowledge/README.md` |
| Business rules & invariants | `ai-docs/knowledge/knowledge/business-rules.md` |
| Event schemas | `ai-docs/knowledge/knowledge/events.md` |
| Projection schemas | `ai-docs/knowledge/knowledge/projections.md` |
| CLI contract | `ai-docs/knowledge/knowledge/cli-contract.md` |
| Domain model | `ai-docs/knowledge/knowledge/domain-model.md` |
| Architecture | `ai-docs/knowledge/knowledge/architecture.md` |
| Critical constraints | `ai-docs/knowledge/knowledge/critical-constraints.md` |

---

## Architecture Pattern

- **Pattern**: Event-sourcing
- **Events**: immutable, append-only, ordered by `event_id` (not timestamp)
- **Projections**: derived state, disposable, rebuildable from event replay
- **No undo**: corrections create new compensating events
- **Validation**: always validate before creating events; fail loudly on mismatch
- **Transactions**: event insertion + projection update must be atomic

---

## Domain Concepts

- **Entities**: items, locations, projects
- **System locations**: `Missing`, `Borrowed`, `Loaned` (immutable, predefined — cannot rename/delete/reparent)
- **Move types**: `temporary_use` (preserves origin), `rehome` (clears temp state)
- **Selector syntax**: `LOCATION:ITEM` (both use canonical names)
- **Name canonicalization**: lowercase, spaces/hyphens → underscores, trimmed
- **No colons in names**: reserved for selector syntax

---

## Session Artifacts

- **Session directory**: `ai-docs/sessions/YYYYMMDD-HHMMSS/`
- **Knowledge base**: `ai-docs/knowledge/knowledge/`
- **Research**: `ai-docs/research/`

---

## Code Style Conventions

- **Styles**: all lipgloss styles live in `appStyles` (singleton in `styles.go`); never inline `lipgloss.NewStyle()` in rendering functions
- **Output**: silence is success by default; verbose details behind `-v`/`--verbose`
- **Errors**: actionable — include failure, likely cause, and remediation step
- **Enums**: typed `iota` constants, never bare integers
- **ORDER BY**: always include a tiebreaker (typically `event_id DESC`)
