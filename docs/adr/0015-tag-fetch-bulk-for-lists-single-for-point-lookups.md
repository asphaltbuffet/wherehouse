# Tag fetch is bulk for list operations, single-query for point lookups; `entityToResult` stays pure

`entityToResult` is a pure free function that accepts `tags []string` explicitly and performs no I/O. Callers are responsible for supplying the tags. Two helpers implement the two load patterns:

- `entityWithTags(ctx, e)` — fetches tags for a single entity via `store.GetTagsByEntity`, then calls `entityToResult`. Used by point-lookup operations (`CreateEntity`, `RenameEntity`, `ReparentEntity`, `RemoveEntity`, `GetEntityByPath`, `GetEntityByID`).
- List operations (`ListEntities`, `GetChildren`, `FindEntities`) call `store.GetTagsByEntities(ctx, ids)` to fetch tags for the full result set in one query, then pass each entity's slice into `entityToResult` directly.

We rejected the alternative introduced in the entity-tags PR: promoting `entityToResult` to a method on `*App` that issues `GetTagsByEntity` internally. That approach is concise at each call site but silently introduces an N+1 query pattern — every list operation becomes O(N) SQL round-trips with no visible signal at the call site. A future author adding a new list method would reproduce the same pattern without knowing it was a problem. Making the I/O explicit in the caller keeps the cost visible and forces a deliberate choice between bulk and single fetch.

The two-helper shape (`entityWithTags` for point lookups, `GetTagsByEntities`+`entityToResult` for lists) is the canonical pattern for any future method that returns one or many `EntityResult` values.
