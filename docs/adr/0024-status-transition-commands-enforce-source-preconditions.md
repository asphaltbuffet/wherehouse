# Status-transition commands enforce source-status preconditions

Each intent-driven status command (`lost`, `found`, `loan`, `return`) asserts that the entity is in a legal *source* status before transitioning it, rather than dispatching an idempotent or no-op status change. A command that cannot make a meaningful transition returns an error; in a batch, one illegal argument rolls back the whole batch (atomic).

## Decision — legal transition table

| Command | Target status | Legal source statuses | Also blocked when |
|---|---|---|---|
| `lost` | `missing` | `ok` | locked |
| `found` | `ok` | `missing` | — |
| `loan` | `loaned` | `ok`, `missing` | locked |
| `return` | `ok` (or `removed` if source is `borrowed`) | `loaned`, `borrowed` | — |
| `remove` | `removed` | any except `borrowed` | — |
| `borrow` | `borrowed` (new entity) | — (creation) | — |

Every non-`ok` recoverable state has exactly one command to leave it: `missing → found`, `loaned/borrowed → return`. There is no overlap and no gap.

## Considered Options

The previous behavior was permissive: commands dispatched the target status regardless of source, so `return` on an `ok` entity, `lost` on an already-`missing` entity, etc. succeeded as no-op `X→X` events. Rejected because it wrote meaningless events to the append-only log and reported success for operations that did nothing.

## Consequences

- The table is deliberately **asymmetric**: `loan` accepts a `missing` source (you marked it missing, then recalled it is at someone's house) but `lost` does **not** accept a `loaned` source (return it first, then mark it lost). This is intentional — a future reader should not "simplify" it into a symmetric table.
- `found` is restricted to `missing` sources only, keeping a sharp boundary with `return` (`loaned`/`borrowed`). The two never overlap.
- `found` has no `locked` guard (recovering a locked entity to `ok` is always allowed); `lost` and `loan` retain their `locked` guards per the `locked` definition in CONTEXT.md.
