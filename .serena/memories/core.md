# Core — wherehouse

Event-sourced CLI inventory tracker ("where did I put my 10mm socket wrench?").

## Key layers

- `cmd/*/` — Cobra commands; one per operation; inject `*App` via per-cmd interface (`addApp`, `moveApp`, …)
- `internal/app/` — Business logic; `App` struct; exposes `CreateEntity`, `RenameEntity`, `ReparentEntity`, `RemoveEntity`, `GetEntityByPath/ID`, `ListEntities`, `GetChildren`, `ChangeStatus`, `GetHistory`, `FindEntities`
- `internal/eventbus/` — `Bus` dispatches events, inserts them, calls `applyEventTx` → per-type handler; handles path propagation
- `internal/store/` — SQLite `Store`; wraps `*sql.DB`; `ExecInTransaction`/`WithRetry`; entities + events CRUD
- `internal/inventory/` — Pure domain types: `Entity`, `EntityType`, `EntityStatus`, `Event`, `EventType`, `CanonicalizeString`
- `internal/web/` — HTMX server; `internal/web/app.go` defines its own `App` interface (superset of cmd interfaces)
- `internal/cli/` — Shared CLI helpers: `OpenDatabase`, `OutputWriter`, actor, flags, config access
- `internal/entitypath/` — Colon-separated path type

## Critical data flow

Command → `cli.OpenDatabase` → `store.Open` + `app.New(store)` → command injects `*app.App` via local interface → `app.X` → `bus.Dispatch` → `applyEventTx` → handler updates `entities_current` projection in same tx

## Storage schema

Two tables: `events` (append-only, source of truth) + `entities_current` (sole projection, rebuildable). Every `ORDER BY` that could tie must include `event_id ASC`.

See `mem:tech_stack`, `mem:conventions`, `mem:suggested_commands`, `mem:task_completion`.
