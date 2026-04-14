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
| New command/subcommand | `ai-docs/knowledge/cli-contract.md` | Pattern: `NewXxxCmd() *cobra.Command` — no db arg. DB opened inside `RunE` via `cli.OpenDatabase(cmd.Context())`, then passed to `runXxxCore(cmd, args, db)`. Per-command `db.go` defines minimal `xxxDB` interface + `//go:generate mockery`. Register in `cmd/root.go` via `NewXxxCmd()`. |
| Output format changes | `cmd/root.go`, `internal/cli/output.go` | Global `--json` flag already exists in `cmd/root.go` (`PersistentFlags()`). There is no per-command `--format` flag — `--json` is the global mechanism; adding `--format` would duplicate it. JSON output paths are implemented in most commands. All styles via `appStyles` singleton in `internal/styles/styles.go` — never inline `lipgloss.NewStyle()` in rendering functions. |
| Data structures / events | `ai-docs/knowledge/events.md`, `ai-docs/knowledge/business-rules.md` | Event types live in `internal/database/eventTypes.go` (iota + `eventTypeByName` map). Adding a new type: add iota entry → regenerate `eventtype_string.go` (`go generate ./internal/database/`) → add case in `internal/database/eventHandler.go`. |
| Troubleshooting | `ai-docs/knowledge/business-rules.md` | Events immutable, ordered by `event_id` only (timestamps are informational, not unique). Every `ORDER BY` that could tie must include `event_id ASC/DESC` as tiebreaker. System locations (`Missing`, `Borrowed`, `Loaned`) are predefined and immutable. |
| CI / dev tooling | See **CI / Dev Tooling** section below | `go.mod` is the single Go version source of truth; all CI workflows use `go-version-file: 'go.mod'`. **No CI workflow changes are needed when updating Go version — only edit `go.mod`.** |

### What doesn't exist yet (don't search for these)

- **Tags/tagging** — no `ItemTaggedEvent`, no `tags` column, no `cmd/tag/`
- **TUI** — `internal/tui/` does not exist; `ai-docs/research/tui/` has proposals only
- **Projects CLI** — `internal/database/project.go` exists but no `cmd/project/` commands
- **`internal/events/` package** — event types live in `internal/database/eventTypes.go`

When you search for something that doesn't exist and spend 3+ calls confirming it, add it here.

---

## Structure

**Navigation**: use Serena MCP tools for all code navigation (see global CLAUDE.md).

```
cmd/                    # CLI commands — one subdir per command
  <cmd>/
    <cmd>.go            # cobra command: NewXxxCmd() — opens DB via cli.OpenDatabase; delegates to runXxxCore(cmd, args, db)
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
- **Constructor pattern**: `NewXxxCmd() *cobra.Command` — no db parameter. Open DB inside `RunE` with `cli.OpenDatabase(cmd.Context())`, then call `runXxxCore(cmd, args, db)` for testability. Never pass `*database.DB` directly to a run function.
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

- When a "what doesn't exist" entry gets implemented: remove it
- When you spend 3+ calls finding something that should have been a shortcut: add it
- Keep every entry terse enough to scan in one line — no prose explanations
