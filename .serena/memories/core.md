# Wherehouse — Core

Event-sourced CLI inventory tracker (Go). Single binary, SQLite storage, no CGo.

## Source map
- `cmd/` — Cobra CLI commands; one subdir per command
- `internal/app/` — business logic (App struct, import, export, doctor)
- `internal/eventbus/` — event dispatch + projection maintenance (Bus)
- `internal/store/` — SQLite persistence; `migrations/` has SQL schema
- `internal/inventory/` — pure domain types (Entity, Event, EventType, EntityStatus)
- `internal/web/` — HTTP server, handlers, embedded assets
- `internal/entitypath/` — colon-separated path parsing
- `internal/styles/` — lipgloss singleton

## Key invariants
- Events are append-only; `event_id ASC` defines replay order (timestamps informational)
- `entities_current` is the sole projection table — rebuildable from event stream
- Every ORDER BY that could tie must include `event_id ASC` as tiebreaker
- No undo — corrections create compensating events

Domain glossary: `mem:domain`
Build/test commands: `mem:suggested_commands`
Code conventions: `mem:conventions`
Task completion: `mem:task_completion`
