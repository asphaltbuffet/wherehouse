# Core

Wherehouse: personal inventory tracker CLI. Go 1.25, event-sourced SQLite backend.

## Source map

- `cmd/` — cobra commands, one subdir per command
- `internal/app/` — business logic (`App` struct, result types like `HistoryResult`, `ExportResult`)
- `internal/store/` — SQLite persistence (`Store` struct, migrations/)
- `internal/inventory/` — pure domain types (`EntityType`, `EntityStatus`, `Entity`, `Event`)
- `internal/cli/` — shared CLI helpers: selectors, output, user identity
- `internal/config/` — XDG TOML config (viper)
- `internal/web/` — HTTP server, handlers, templates, embedded assets
- `internal/eventbus/` — event dispatch, path propagation
- `internal/entitypath/` — colon-separated path parsing
- `internal/styles/` — lipgloss appStyles singleton (Wong palette, AdaptiveColor)
- `internal/nanoid/` — 10-char alphanumeric NanoID generation
- `cmd/root.go` — root command, registers via `NewDefaultXxxCmd()`

## Key invariants

- Events are immutable source of truth (append-only). Projections are rebuildable.
- Every `ORDER BY` that could tie **must** include `event_id ASC` tiebreaker.
- DB schema source of truth: `events(event_id PK, event_type, timestamp_utc, actor_user_id, payload JSON, note)`
- Projection tables: `locations_current`, `items_current`, `projects_current`

See `mem:conventions` for command patterns; `mem:tech_stack` for build tools.
