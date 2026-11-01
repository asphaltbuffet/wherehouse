# Legal destinations are decided once, in the app layer, over a swappable hierarchy reader

Whether a destination is a **legal destination** (see CONTEXT.md) is answered by pure functions in `internal/app` (`destination.go`): one for a new entity (`add`, `borrow`: the destination is an eligible parent) and one for a relocation (`move`/`use`: eligible parent; not self or a descendant; no home cycle; and the relocation-kind rules for the current parent and the home, ADR 0039). They read the tree through a one-method `hierarchy` interface (`get(id)`, any status). Every rule walks upward, so nothing else is needed. There are two adapters: one reads through the write transaction for the guards (ADR 0035 unchanged: the in-transaction guard still decides), and one reads a `ListAllEntities` snapshot for the web move picker. A third reads committed state with no transaction for the "Move to root" choice: the store's transactions take the write lock, and a form render needs only a few lookups. Previously the guards walked the tree lazily, and `FindMoveDestinations` filtered a snapshot with its own exclusion set that skipped the home-cycle rule. The picker could therefore offer destinations the write rejected.

The add-child rule is one pure predicate. `Check(ActionAddChild)`, the destination functions, and the result projector all call it. The projector sets `EntityResult.CanAddChild`, which the tree template and parent-candidate searches read; `EntityResult.IsEligibleParent` and the template's own `not .Discrete` test are removed. The predicate needs only status and `discrete`, which every result already carries, so list pages pay no extra query (ADR 0040's reason for keeping `EntityActions` off `EntityResult` does not apply).

A failed destination check is a `DestinationRejection` (`Verb`, `Kind`, the path of the entity being placed, the destination path, `Blocker` (the displaced entity in a home cycle), and `Rehome` (the destination is a displaced entity's current parent)). It unwraps to both `Kind` and `ErrInvalidDestination`, so callers can match the specific cause or the generic one. `Error()` renders the CLI sentence from one template; web maps `Kind` to its own copy (ADR 0014).

## Considered Options

- **The guards load a full snapshot inside the transaction too**: rejected. Every move and use would pay a full-table read under the write lock just to avoid writing a second adapter.
- **Keep both algorithms and add parity tests**: rejected. The rule would still be written twice; tests would catch drift but not prevent it.
- **Leave the relocation-kind rules (current parent, home) in the commands**: rejected. The picker would still restate them, and the acceptance property ("every offered destination is accepted") would depend on two copies.
- **Extend ADR 0040's `Rejection`**: rejected. Its `Remedy` always targets the same entity, while a home cycle's fix ("return the drill first") targets a different one. Many fields would be empty for one use or the other.
- **Show home-cycle destinations disabled in the picker with a reason**: deferred. Illegal destinations are hidden, consistent with ADR 0034. The typed rejection per candidate keeps this open as a UI-only change.

## Consequences

- Correctness of the picker now rests on the two adapters agreeing. A table test asserts they return the same `Kind` for every (moving, destination, relocation) triple over a fixture that has a home cycle, a removed ancestor with a live child (#287), a discrete entity, and a displaced entity.
- CLI wording for destination errors changes to the template; it was never a contract (ADR 0002, ADR 0040).
- Destination legality stays separate from action eligibility (ADR 0040): "may X be relocated at all" and "may X go to Y" are still two checks.
