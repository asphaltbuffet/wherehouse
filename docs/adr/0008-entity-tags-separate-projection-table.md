# Entity tags stored in a separate `entity_tags` projection table

Tags are stored in a dedicated `entity_tags` table (`entity_id TEXT`, `tag TEXT`, `PRIMARY KEY (entity_id, tag)`) rather than as a JSON column on `entities_current`.

The alternative was a `tags TEXT` JSON array column on `entities_current` (queryable via SQLite's `json_each()`). That approach keeps everything in one table but makes tag-based filtering awkward — `WHERE tag = 'tool'` becomes a JSON predicate rather than a plain join.

We chose the separate table because the primary stated use case for tags is filtering: "give me all entities tagged `tool`". A relational `(entity_id, tag)` table makes that query trivial and index-friendly. The JSON column approach optimizes for co-location at the cost of the query shape that matters most.

`entity_tags` is a projection table: it is derived from `EntityTagAddedEvent` and `EntityTagRemovedEvent` in the event stream and must be fully rebuildable by replaying those events. Tags are retained when an entity is removed, consistent with how `entities_current` handles soft-deleted entities.
