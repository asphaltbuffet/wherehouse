# Core

Go CLI tool (wherehouse) for inventory management via event sourcing. Single SQLite database.

## Source map

- `cmd/` — cobra CLI commands; one subdir per command. Each has `NewXxxCmd(db xxxDB)` + `NewDefaultXxxCmd()`.
- `internal/app/` — business logic `App` struct; delegates to store and eventbus. All public operations go here.
- `internal/eventbus/` — event dispatch, validation, TruncateAndReplay (projection rebuild).
- `internal/store/` — SQLite persistence; `migrations/` subdir.
- `internal/inventory/` — pure domain types (EntityType, EntityStatus, Entity, Event, EventType).
- `internal/web/` — HTTP server, handlers, templates, embedded assets.
- `internal/cli/` — shared CLI helpers (selectors, output, flags, user identity).
- `internal/config/` — XDG-compliant TOML config (viper-backed).
- `internal/entitypath/` — colon-separated path parsing.
- `internal/styles/` — lipgloss appStyles singleton; Wong palette.
- `internal/nanoid/` — 10-char alphanumeric NanoID generation.
- `internal/logging/` — structured logging + rotation.
- `internal/version/` — build version info.

## Key invariants

- Events are immutable append-only source of truth. Projections are rebuildable.
- No undo — corrections via compensating events.
- Replay order: strictly by `event_id ASC`.
- Every `ORDER BY` that could tie must include `event_id ASC` as tiebreaker.
- No CGo. Go 1.25.

See `mem:conventions` for code patterns, `mem:tech_stack` for tooling, `mem:task_completion` for done criteria.
