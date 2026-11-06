# Event payloads are sealed types that name their EventType

Every payload struct in `internal/eventbus` implements a sealed `Payload` interface: `EventType()`, `Subject()` (the entity ID), and an unexported marker method. Only `eventbus` types can satisfy it. `Bus.Dispatch(ctx, tx, actor, p Payload, note string)` is the only public way to write an event, and it always runs inside a transaction the caller owns. The bus marshals `p`, sets the event's type from `p.EventType()` and the `entity_id` column from `p.Subject()`, and stores an empty note as `NULL`. Handlers are registered with generic helpers (`register[P]`, `registerPathChange[P]`, and a relocating variant over a `relocator` interface). The registry key therefore comes from the payload type, and each handler receives its payload already decoded. Before this change, app code marshalled payloads to `json.RawMessage` and passed the `EventType` separately, so nothing tied the two together. `EntityUsedPayload` and `EntityReparentedPayload` have the same JSON shape, so either could be sent under the other's type. The bus also re-parsed every payload into `map[string]any` to find `entity_id`, and when that field was missing it wrote `NULL` without reporting anything. Every handler, and `decodeRelocation`, decoded the payload again by hand.

`Dispatch` rejects **derived events** (`EventType.IsDerived()`, see CONTEXT.md). Only the bus writes them, during path propagation.

The `entity_id` column and the payload's `Subject()` must agree, but each path checks this differently. Live dispatch cannot produce a mismatch, because the column comes from `Subject()`. Import (`ReplayEvent`) rejects any record whose column differs from its payload, because import is the only path that accepts untrusted input. A NULL column claims nothing, so import fills it from the payload, as live dispatch would; a record with no entity_id in either place is rejected. Projection rebuild (`TruncateAndReplay`) tolerates a mismatch, because it is the recovery tool and must still work on logs written before this check existed. Doctor reports existing mismatches as event-log issues (ADR 0011 shape) and says that `doctor --rebuild --force` rebuilds anyway.

## Considered Options

- **Typed constructors per event (`eventbus.Used(id, parent)`) with unexported payload structs**: rejected. They are stricter, but import, doctor and tests need the raw structs, and we would have to maintain about 16 constructors.
- **Keep `json.RawMessage` and only extract `entity_id` more safely**: rejected. It leaves the payload/type pairing unchecked.
- **A runtime factory map with type assertions in handlers**: rejected. It goes against the project rule against `any` casts, and the generic helpers decode once with no assertions.
- **A separate unexported interface for derived payloads, enforced at compile time**: rejected. It protects a single event type at the cost of making every generic helper handle two interface types.
- **Enforce column/payload agreement in rebuild too**: rejected. An existing database with a legacy mismatch could then no longer be rebuilt.

## Consequences

- Adding an event type means adding a payload struct with the three methods and one `register` line. The registry entry can no longer name a payload or handler that doesn't match its key.
- `EntityStatusChangedPayload.Status` is an `inventory.EntityStatus`. Its JSON encoding is byte-identical to the previous string field, and a golden test guards that. An unknown status now fails at decode, so doctor reports it as a malformed payload.
- An export with a hand-edited or corrupted `entity_id` that previously imported will now be rejected.
