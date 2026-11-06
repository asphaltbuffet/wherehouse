# Store reads and writes go through separate Reader and Tx types

`internal/store` exposes two querier types. `Reader` has the read methods only. `Tx` embeds `Reader` and adds every write, including `AppendEvent`, which is the one writer to the `events` table. `Store.Reader()` returns a `Reader` over the connection pool, and `Store.ExecInTransaction` passes a `Tx` to its callback. Both hold their connection in an unexported field, so code outside `store` cannot issue SQL. Before this change, `store.Tx` was an interface exposing `ExecContext`, and the store had about 45 methods split into Tx and non-Tx twins. `eventbus` wrote `events` rows with its own hand-written INSERTs. Two rules were held up only by comments: `Reader()` must never write (ADR 0040), and guards must run inside the write transaction (ADR 0035). Now the types enforce both. A `Reader` has no write methods, and only code holding a `Tx` can write. This adds compile-time enforcement to those ADRs; it does not replace them.

Each read that returns entities states its status filter in its name: `GetLiveEntity` / `GetEntityAnyStatus`, `GetLiveDescendants` / `GetDescendantsAnyStatus`. Neither form is the default (see **Live** in CONTEXT.md). The old Tx twins did not always filter the same way as their non-Tx counterparts. `GetEntityTx` and `GetDescendantsTx` included removed entities, while `GetEntity` and `GetDescendants` did not. A merge that kept the bare names would have quietly changed which rows some callers get.

## Considered Options

- **Keep methods on `*Store` and pass a querier argument** (`s.GetLiveEntity(ctx, s.Reader(), id)`): rejected. The receiver would be used only as a namespace, and nothing would stop a write method from receiving the pool.
- **Package-level functions taking a querier** (`store.GetLiveEntity(ctx, q, id)`): rejected for the same reason. Reads and writes would both accept the same argument type.
- **The bare name means live, and `AnyStatus` opts out**: rejected. Call sites that currently use the any-status Tx twins would silently start filtering.

## Consequences

- `Store.DB()` remains, but only for tests that insert deliberately malformed rows (for example `doctor_test.go`). Production code outside `store` must not use it.
- Write methods drop the `Tx` suffix (`tx.UpdateEntity`, `tx.InsertTag`), because the receiver already says so.
