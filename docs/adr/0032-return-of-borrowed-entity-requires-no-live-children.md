# Return of a borrowed entity requires no live children

Returning a `borrowed` entity sets it to `removed`. If it still has non-removed children (e.g. borrowed bits inside a borrowed drill case — permitted, since `borrowed` entities are eligible parents), those children would be left under a removed parent and become unreachable from the tree while still carrying a live status. `return` therefore rejects a `borrowed` entity that has any non-removed children; the user must first return or move them. Returning a `loaned` entity is unaffected, since it goes back to `ok` and stays in the hierarchy.

## Considered Options

- **Cascade** — return children in the same transaction. Rejected: there is no sensible target for children in other statuses (an owned `ok` item inside a borrowed case), and implicit multi-entity effects violate "explicit, auditable".
- **Allow and orphan** (prior behavior) — rejected as silent data loss from the user's point of view.

## Consequences

The same orphaning hazard exists for `remove`; extending this guard there is a separate, deliberately deferred behavior change.
