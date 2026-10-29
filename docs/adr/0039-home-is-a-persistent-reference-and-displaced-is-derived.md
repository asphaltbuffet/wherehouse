# Home is a persistent reference; displaced is derived, not a status

Every entity has a **home**: a reference to the entity it belongs in. Home is separate from its current parent. `move` sets both the parent and the home. The new `use` command sets only the parent. An entity is **displaced** when its parent is not its home. Displaced is derived from those two references and is not an `EntityStatus`, so it is independent of status: a `borrowed`, `missing`, or `loaned` entity can also be displaced. This answers "what's out of place?" without a separate tracking session, and it leaves the status lifecycle untouched.

## Decision

- **Home is an entity reference, not a path**, so it stays valid when an ancestor is moved.
- **Existing events define home without migration.** `EntityCreatedEvent` and `EntityBorrowedEvent` set home to the initial parent, and `EntityReparentedEvent` sets both the parent and the home. Replaying any existing log therefore gives every entity the correct home. For the same reason, `EntityReparentedEvent` must never gain a "temporary" flag: old events would become ambiguous.
- **Three new intent-specific events** (following ADR 0021's precedent of recording intent rather than effect):
  - `EntityUsedEvent`: reparent, home unchanged
  - `EntityReturnedHomeEvent`: reparent to home, with the destination stated in the payload
  - `EntityRehomedEvent`: home set to the current parent, with no reparent
- **`return` restores both status and location.** This supersedes the `return` row of ADR 0024:

| Status | Location | `return` does |
|---|---|---|
| `ok` | displaced | moves it home |
| `ok` | at home | rejected (nothing to restore) |
| `loaned` | at home | `loaned → ok` |
| `loaned` | displaced | `loaned → ok` and moves it home, in one transaction |
| `borrowed` | either | `borrowed → removed`, with no relocation |
| `missing` | either | rejected; `found` first (which does not relocate) |

- **Homes are protected, not checked after the fact.** While an entity is the home of any non-removed displaced entity, removing it or setting it `discrete` is rejected. Any move or use that would place a home inside the subtree of an entity it is home to is also rejected. As a result, `return` of a displaced `ok` entity always has a valid destination. A locked entity is never displaced: `use` is blocked by `locked`, and `lock` is rejected while the entity is displaced.

## Considered Options

- **An `in_use` status.** Rejected. Status is single-valued, so `in_use` would conflict with `borrowed` (which is terminal), `missing` (lost while away from home) and `loaned`. It would also mix location into the lifecycle, against the rule that "move changes location only, never status".
- **Home captured only at use time.** Rejected because it can't answer "what isn't where it belongs?" for entities that ended up out of place by other routes.
- **A separate verb (such as `stow`) for sending an entity home.** Rejected. `return` is a single intent, "restore this entity to how it should be", and not an overloaded command.
- **Writing return-home as a plain `EntityReparentedEvent`.** Its effect on the projection is identical, but history would show "moved" instead of "returned".
- **Detecting broken homes at return time.** Rejected in favor of preventing them, consistent with ADR 0032.
