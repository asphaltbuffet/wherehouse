# Mutation guards run inside the write transaction; bus handlers stay guard-free

Every precondition on a mutation (liveness, `locked`, `discrete`, status, cycle checks) lives in the app layer and must read through the same transaction that dispatches the event — `ExecInTransaction` with `*Tx` store reads and `bus.DispatchInTx`. Since #300 the connection opens transactions with `_txlock=immediate`, so the write lock is taken at `BEGIN` and a guard read inside the transaction sees every prior commit; a guard read through the pooled connection before `BEGIN` does not, and lets concurrent writers interleave between check and write (#301: a move into a parent that is concurrently removed, or two crossing moves `A→B` / `B→A` forming a cycle). The event bus handlers (`applyEventTx`) do not re-validate domain rules: they are shared by live dispatch, `TruncateAndReplay`, and import, and must apply recorded history as-is.

## Considered Options

- **Guards in bus handlers as defense in depth** — rejected: replay and import would start failing on databases that already contain historical violations (e.g. from the pre-ADR-0034 cycle bug or #287), making projections non-rebuildable, and the only way out would be silent repair, which the hard invariants forbid.
- **Guards before the transaction, accept the race** — rejected: the web UI and CLI can mutate the same database concurrently, so the race is reachable.

## Consequences

Detecting already-corrupt state (a live entity under a removed parent, a parent cycle) is `doctor`'s job, not the write path's. A mutation that composes several events (e.g. `tag` with multiple adds/removes) commits them in one transaction, so a failure applies none of them.
