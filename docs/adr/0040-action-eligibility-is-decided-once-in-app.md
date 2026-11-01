# Action eligibility is decided once, in the app layer; interfaces read it, never re-derive it

Whether an **action** (see CONTEXT.md) may apply to an entity is answered by one pure function in `internal/app` — `Check(EntityFacts, Action) *Rejection` — which holds the whole transition table of ADR 0024 and ADR 0039 plus the `locked`, `discrete`, live-children and home rules. Write guards call it inside the transaction on facts loaded by `loadFactsTx` (ADR 0035 unchanged: the in-transaction guard still decides). The web UI asks `App.EntityActions(ctx, id)` for an `ActionSet` and uses it for render decisions only; it does not inspect `Status`, `Locked`, `Discrete` or displacement to decide which actions to offer. Previously the table was restated in app guards, in web `Can*` flags, in web tooltip helpers, and in `EntityResult.CanMove`, and web chose its failure message from its own copy because rejections were untyped strings.

A `Rejection` is structured — `Action`, `Kind` (unwrapping to a sentinel such as `ErrLocked`, `ErrWrongStatus`, `ErrDisplaced`, `ErrHasChildren`, `ErrIsHome`, `ErrNoChange`), the entity's `Status`, and an optional `Remedy` action — and carries no interface-specific prose. `Error()` renders the CLI sentence from one template; web maps `Kind` to its own UI copy and falls back to `Error()`. Tests assert on `Kind`/`Remedy`, not message text.

## Considered Options

- **Web runs `Check` itself on facts it loads** — rejected: web would still decide which rules apply to which control, a smaller copy of the same table.
- **Only per-entity-attribute rules in the module; live-children and home rules left as separate guards** — rejected: remove and discrete eligibility would stay split, and web would keep offering Remove on an entity the home rule protects.
- **One shared reason string for CLI and web** — rejected: wording is presentation (ADR 0014); the rule and its remedy are not.
- **Lazy, per-action fact loading** — rejected: it needs its own "which facts does this action need" table. Loading all facts costs two indexed single-row queries under a write lock already held.

## Consequences

- `move`/`use` eligibility covers only "may this entity be relocated at all". Destination legality (self, descendant, eligible parent, home cycle) stays with the relocation guards.
- `EntityActions` is a separate call, not a field on `EntityResult`, so list and search pages do not pay an N+1 facts query.
- CLI rejection messages were reworded to the template; they were never a documented contract (mutation errors render as plain stderr text per ADR 0002).
