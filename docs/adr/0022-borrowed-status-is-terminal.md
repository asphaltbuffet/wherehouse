# borrowed status is terminal

Entities with `borrowed` status cannot be transitioned to any other status except `removed`, which is applied automatically by the `return` command. Direct `remove` is also blocked — `return` is the only valid exit. `lost`, `found`, `loan`, and `status --set` are all blocked on borrowed entities.

This constraint exists because borrowed items are externally owned: they should never be "absorbed" into permanent inventory (`ok`), and the return workflow must be explicit. Tracking intermediate states like `missing` for borrowed items is deferred — it would require knowing an entity's origin status when applying `found`, which is not currently tracked.

## Consequences

The `return` command has branching behavior: if the entity is `borrowed`, it sets `removed`; otherwise it sets `ok`. This branch is in `markReturnInTx` in the app layer.
