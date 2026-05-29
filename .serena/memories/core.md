# Core

Wherehouse: CLI inventory management tool in Go. Event-sourced; `events` table is append-only source of truth; projection tables (`entities_current`) are derived/rebuildable.

## Entry points
- `cmd/` — Cobra CLI commands, one subdir per command
- `internal/` — all business logic, store, web server, domain types
- `main.go` — wires root command

## Critical invariants
- All SQL goes through `internal/store` — commands never touch `*sql.DB` directly
- `ExecInTransaction` + `WithRetry` wrappers for all DB ops
- `ORDER BY` with ties must include `event_id ASC` as tiebreaker
- Event replay ordering is strictly by `event_id`; timestamps are informational
- No CGo; Go 1.25

## Memory graph
- Build/test/lint commands: `mem:suggested_commands`
- Code conventions and patterns: `mem:conventions`
- Tech stack details: `mem:tech_stack`
- Task-done checklist: `mem:task_completion`
