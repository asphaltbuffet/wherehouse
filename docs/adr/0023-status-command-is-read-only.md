# status command is read-only

`wherehouse status <path>` is a read-only lookup. The `--set` flag is removed.

## Context

Every named status transition has a dedicated command (`lost`, `found`, `loan`, `return`, `borrow`, `remove`). The generic `--set` flag was a bypass that skipped all per-transition guards — including the `borrowed` terminal-status constraint added in ADR 0022. It accepted `borrowed` and `removed` as valid `--set` values, which are both invalid manual transitions (borrowing requires `borrow`; removal requires `remove`).

## Decision

`status <path>` fetches and displays the current status of an entity. It accepts no mutation flags. All status changes go through their dedicated commands.

## Output shape

- Returns a list of `StatusOutput` ranked by `last_event_id DESC` (most recent first).
- Includes entities at any status, including `removed` — useful for auditing a returned borrowed item or a soft-deleted entity.
- Empty result (no entity at that path, in any status) is an error (`ErrNotFound`), not silence.
- `--json` emits a JSON array (`[]StatusOutput`), consistent with `list` and `scry`.

## Considered alternatives

Keeping `--set` but adding the borrowed guard was considered. Rejected: the generic escape hatch cannot be made safe without replicating all per-transition logic (locked checks, borrowed-terminal check, loaned context requirements) inside a single function. Intent-driven commands are the right abstraction.
