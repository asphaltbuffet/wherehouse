# Replace EntityType with mutable locked and discrete flags

`EntityType` (`place`, `container`, `leaf`) is removed. The type classification was a permanent label set at creation that could not be changed, which caused problems when the intended use of an entity evolved — for example, adding individual children to something originally created as a leaf. Instead, two mutable boolean attributes replace it:

- `locked` — the entity cannot be directly reparented by the user, and cannot be set to `EntityStatusMissing`. Cascade moves from an ancestor reparent still propagate normally. Defaults to `false`. Old `place` entities migrate to `locked=true` during projection replay; old `container` and `leaf` entities migrate to `locked=false`.
- `discrete` — the entity may not have children added (via `add` or `move`). Intended as a guard rail against accidental nesting, not a structural constraint. Defaults to `false`. Old `leaf` entities migrate to `discrete=true` during projection replay; old `place` and `container` entities migrate to `discrete=false`.

Both flags are set in the `EntityCreatedEvent` payload and can be changed at any time via paired events (`EntityLockedEvent`/`EntityUnlockedEvent`, `EntityDiscreteSetEvent`/`EntityDiscreteClearedEvent`). Since the event log is immutable, the `place`→`locked=true` migration lives solely in the projection replay path — old `EntityCreatedEvent` payloads are read as-is and the projection builder translates `entity_type: "place"` to `locked=1`.

## Considered options

Keeping `EntityType` and adding flags alongside it was rejected — it would preserve the confusing type/flag duality without removing the root problem (type being permanent and wrong at creation time).
