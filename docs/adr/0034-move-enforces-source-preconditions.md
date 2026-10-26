# Move enforces source preconditions

`move` changes an entity's location only, never its status. It previously rejected only `locked` sources and `discrete` destinations, so it would happily relocate a `missing` or `loaned` entity — whose location is by definition unknown or elsewhere — and would accept the entity itself or one of its descendants as the destination, corrupting the hierarchy with a cycle. Following the pattern of [ADR 0024](0024-status-transition-commands-enforce-source-preconditions.md), `move` now rejects any source whose status is not `ok` or `borrowed`: a `missing` entity must be `found` first, and a `loaned` entity must be `return`ed first, each being the single exit path for that status. `borrowed` stays movable because a borrowed entity is physically in the inventory. The destination must be an eligible parent — the root included — and may not be the entity itself or any of its descendants. Moving to the current parent is rejected rather than recorded as a no-op event, for the same reason ADR 0024 rejects `X→X` status transitions. All guards live in the app layer so CLI and web share one rule.

## Considered Options

- **Move implies found** — moving a `missing` entity also marks it `ok`. Rejected: two events from one action, and it blurs the one-command-per-status-transition boundary from ADR 0024.
- **Web-only restriction** — rejected: CLI and web would disagree about the same domain operation, and a crafted request would bypass a UI-only guard.
- **Cycle check in the event bus** — rejected: the app layer is where every other move precondition lives, and replay of already-validated history should not re-litigate it.
