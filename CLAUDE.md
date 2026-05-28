# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## VCS

Use **jujutsu** (`jj`), not `git`. Never run `git` commands directly.

## Build & Test Commands

```bash
mise run build       # go build → dist/wherehouse
mise run test        # gotestsum, race detector, coverage → bin/coverage.out
mise run lint        # golangci-lint --fix → bin/golangci-lint.html
mise run generate    # go generate ./... (stringer for iota enums)
mise run mock        # regenerate mocks via mockery
mise run dev         # generate + lint + test + snapshot + mock
mise run snapshot    # goreleaser build (single target, no release)
mise run cover       # coverage HTML → bin/coverage.html
mise run mod-tidy    # go mod tidy + gomod2nix generate

# Run a single test package
gotestsum -- -race ./internal/database/...

# Run one test by name
gotestsum -- -run TestFoo ./cmd/scry/...
```

Always run `mise run lint` and `mise run test` before committing.

## Task Completion Checklist

1. Fix all warnings (`mise run lint`, `mise run test`, compiler)
2. Run `/pre-commit` skill before every commit
3. Run `/commit` skill for message conventions (conventional commits: `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`)
4. Run `/audit-docs` after features or fixes

## Architecture: Event Sourcing

Events are the **immutable source of truth** (append-only). Projections are derived/rebuildable state. No undo — corrections create compensating events. Replay ordering is strictly by `event_id` (timestamps are informational only).

**Database schema (source of truth):**
```sql
events (event_id PK, event_type, timestamp_utc, actor_user_id, payload JSON, note)
```

**Projection tables (derived, rebuildable):**
- `locations_current` — current location hierarchy
- `items_current` — current item state and associations
- `projects_current` — active and completed projects

Every `ORDER BY` that could tie **must** include `event_id ASC` as a tiebreaker.

## Repository Layout

```
cmd/                     # CLI commands (cobra); one subdir per command
  add/                   # add item / add location
  config/                # config init/get/set/check/edit/path
  history/               # event timeline for an item
  list/                  # list items/locations
  move/                  # move items between locations
  remove/                # remove items
  rename/                # rename items/locations
  scry/                  # find/search items
  serve/                 # local web server (shell only — no HTTP/template/embed code)
  status/                # item status commands
  root.go                # root command, registers via NewDefaultXxxCmd()
internal/
  app/                   # business logic layer (App struct, EntityResult, HistoryResult)
  cli/                   # shared CLI helpers: selectors, output, flags, user identity
  config/                # XDG-compliant TOML config (viper-backed)
  eventbus/              # event dispatch and path-propagation
  inventory/             # pure domain types (EntityType, EntityStatus, Entity, Event)
  store/                 # SQLite persistence layer; migrations/ subdir has SQL schema
  web/                   # HTTP server, handlers, templates, embedded assets
  entitypath/            # colon-separated path parsing (e.g. "Garage:Toolbox:Drawer")
  logging/               # structured logging + rotation
  nanoid/                # 10-char alphanumeric NanoID generation
  styles/                # lipgloss appStyles singleton
  version/               # build version info
```

## Key Patterns

### Command Constructor Pattern
Every command package exposes two constructors:
- `NewXxxCmd(db xxxDB)` — for tests (inject interface)
- `NewDefaultXxxCmd()` — for production wiring

Registered in `cmd/root.go` via `NewDefaultXxxCmd()`.

### Per-Command DB Interface
Each `cmd/xxx/db.go` defines a minimal `xxxDB` interface covering only what that command needs, with a `//go:generate mockery` directive. Never pass `*database.DB` directly to a command's run function.

### web package (internal/web)
`cmd/serve/` is a **thin shell only** — no `net/http`, `html/template`, or `//go:embed`. All server logic lives in `internal/web`. Verify: `rg -l '"net/http"|"html/template"|//go:embed' cmd/serve/` → empty.

`internal/web` uses `//go:embed assets` in `routes.go`. Tests use `httptest.NewServer(srv.Handler())`.

**`EntityResult` field pitfall:** `CanonicalName` is the normalized *leaf name only* (no colons). `FullPathDisplay` is the full colon-separated path (e.g. `"Garage:Toolbox"`). Use `FullPathDisplay` when checking entity depth or path structure.

### Adding a New Event Type
1. Add to `EventType` iota + line-comment string + `eventTypeByName` map in `internal/database/eventTypes.go`
2. Run `go generate ./...` to regenerate `eventtype_string.go`
3. Add a case to `processEventInTx` in `eventHandler.go`

### Styles
All styles live as private fields on the `Styles` struct in `internal/styles/styles.go`. Access via public accessor methods on the `appStyles` singleton. Never inline `lipgloss.NewStyle()` in rendering functions. Use Wong palette with `lipgloss.AdaptiveColor{Light, Dark}` for colorblind safety.

## Code Conventions

- Go 1.25, no CGo
- `testify/assert` for non-fatal assertions, `testify/require` for preconditions
- Wrap errors: `fmt.Errorf("context: %w", err)`
- All DB operations use transactions via `ExecInTransaction` and `WithRetry` helpers
- Timestamps: RFC3339 with UTC `Z` suffix
- No type assertions / `any` casts — prefer typed interfaces
- No magic numbers — use stdlib constants
- UI: silence is success; verbose output only with `-v`/`--verbose`; `--json` on all commands

## Open TODOs (from code review — do not silently ignore)

- **[M1]** Queries in `internal/database/item.go` and `location.go` missing `event_id ASC` tiebreakers on `ORDER BY display_name`
- **[M2]** `cmd/remove/item.go` intentional system-location logic needs a clarifying comment
- **[M3]** `internal/database/search.go` `extractLocationFromEvent` missing `item.removed` case

## Shell Tooling Preferences

- `fd` over `find`, `rg` over `grep`, `sd` over `sed`, `jq` for JSON
- No `&&` between shell commands — run as separate tool calls
- No `git` commands — use `jj` equivalents
- `jj describe <change-id> -m "message"` renames any commit; `jj log --no-graph -r 'trunk()..@'` reviews full branch history

## Agent skills

### Issue tracker

Issues live in GitHub Issues at `github.com/asphaltbuffet/wherehouse`. See `docs/agents/issue-tracker.md`.

### Triage labels

Custom label vocabulary in use (e.g. `to_triage`, `ready_afk`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context repo: one `CONTEXT.md` + `docs/adr/` at the root. See `docs/agents/domain.md`.
