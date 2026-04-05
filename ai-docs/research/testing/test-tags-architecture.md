# Tags Architecture for Items

**Purpose**: Add optional string tags to items in the wherehouse event-sourced system.
**Status**: Architecture plan (test exercise)
**Date**: 2026-02-20

---

## Problem Summary

Items need zero or more simple string tags for categorization and search. Tags must follow event-sourcing patterns: changes to tags produce events, and tag state in projections is rebuildable from events.

---

## Recommended Approach

Use two new event types (`item.tagged` and `item.untagged`) with a separate projection table (`item_tags_current`) for the tag-to-item relationship. This keeps the existing `items_current` table unchanged and follows the existing event patterns.

**Rationale**: A separate junction table is simpler and more queryable than storing tags as JSON within `items_current`. It also avoids modifying the existing item projection schema.

---

## New Events

### item.tagged

**Purpose**: Add a tag to an item.

```json
{
  "event_id": 100,
  "event_type": "item.tagged",
  "timestamp_utc": "2026-02-20T10:00:00Z",
  "actor_user_id": "alice",
  "item_id": "uuid",
  "tag": "hand-tool",
  "note": "optional"
}
```

**Validation**:
- `item_id` must exist in `items_current`
- `tag` must not be empty
- `tag` must not contain `:` (reserved separator)
- `tag` is stored in canonical form (lowercase, trimmed, underscores)
- Duplicate tag on same item is idempotent (no error, no new row)

**Projection Update**:
```sql
INSERT OR IGNORE INTO item_tags_current (item_id, tag) VALUES (event.item_id, event.tag);
```

### item.untagged

**Purpose**: Remove a tag from an item.

```json
{
  "event_id": 101,
  "event_type": "item.untagged",
  "timestamp_utc": "2026-02-20T10:05:00Z",
  "actor_user_id": "alice",
  "item_id": "uuid",
  "tag": "hand-tool",
  "note": "optional"
}
```

**Validation**:
- `item_id` must exist in `items_current`
- `tag` must not be empty
- Removing a tag that does not exist is idempotent (no error)

**Projection Update**:
```sql
DELETE FROM item_tags_current WHERE item_id = event.item_id AND tag = event.tag;
```

---

## Projection Table

### item_tags_current

```sql
CREATE TABLE item_tags_current (
  item_id  TEXT NOT NULL,
  tag      TEXT NOT NULL,

  PRIMARY KEY (item_id, tag),
  FOREIGN KEY (item_id) REFERENCES items_current(item_id) ON DELETE CASCADE
);

CREATE INDEX idx_item_tags_tag ON item_tags_current(tag);
CREATE INDEX idx_item_tags_item ON item_tags_current(item_id);
```

**ON DELETE CASCADE**: When an `item.deleted` event removes a row from `items_current`, the cascade automatically cleans up associated tags. This keeps the projection consistent without needing the `item.deleted` handler to know about tags.

**Rebuild**: During full replay, `item_tags_current` is cleared and rebuilt alongside other projections. The `item.tagged`/`item.untagged` events are processed in `event_id` order.

---

## Components That Need Changes

### 1. Event Types (internal/events/)

- Add `item.tagged` and `item.untagged` to the event type registry
- Define payload structs:

```go
type ItemTaggedPayload struct {
    ItemID string `json:"item_id"`
    Tag    string `json:"tag"`
}

type ItemUntaggedPayload struct {
    ItemID string `json:"item_id"`
    Tag    string `json:"tag"`
}
```

### 2. Validation (internal/validation/)

- Tag validation function: non-empty, no colons, canonicalize
- Item existence check (reuse existing)

```go
func ValidateTag(tag string) (string, error) {
    tag = canonicalize(tag)
    if tag == "" {
        return "", errors.New("tag must not be empty")
    }
    if strings.Contains(tag, ":") {
        return "", errors.New("tag must not contain ':'")
    }
    return tag, nil
}
```

### 3. Projections (internal/projections/)

- Add `item_tags_current` table creation to schema setup
- Add projection handlers for `item.tagged` and `item.untagged`
- Register handlers in the replay dispatcher

### 4. Database (internal/database/)

- Migration to add `item_tags_current` table
- Update schema version

### 5. CLI Commands (cmd/ or internal/cli/)

- `wherehouse tag <item> <tag>` -- add tag
- `wherehouse untag <item> <tag>` -- remove tag
- `wherehouse list --tag <tag>` -- filter items by tag
- `wherehouse item show <item>` -- display tags in output

### 6. Replay (internal/projections/ or internal/events/)

- Clear `item_tags_current` during full rebuild
- Process `item.tagged`/`item.untagged` during replay

---

## Query Patterns

**Find items by tag**:
```sql
SELECT i.*, l.full_path_display
FROM items_current i
JOIN item_tags_current t ON i.item_id = t.item_id
JOIN locations_current l ON i.location_id = l.location_id
WHERE t.tag = 'hand_tool'
ORDER BY i.display_name;
```

**Find items with ANY of several tags**:
```sql
SELECT DISTINCT i.*
FROM items_current i
JOIN item_tags_current t ON i.item_id = t.item_id
WHERE t.tag IN ('hand_tool', 'metric')
ORDER BY i.display_name;
```

**List all tags for an item**:
```sql
SELECT tag FROM item_tags_current
WHERE item_id = ?
ORDER BY tag;
```

**List all known tags**:
```sql
SELECT DISTINCT tag FROM item_tags_current ORDER BY tag;
```

---

## Tag Canonicalization

Tags follow the same canonicalization rules as item names:
- Trim whitespace
- Lowercase
- Collapse whitespace/separators to `_`
- No colons allowed

Examples:
- `"Hand Tool"` -> `"hand_tool"`
- `"10mm"` -> `"10mm"`
- `"Power-Tools"` -> `"power_tools"`

---

## Interaction with item.deleted

When an item is deleted via `item.deleted` event:
1. The `items_current` row is removed
2. `ON DELETE CASCADE` on the FK removes associated `item_tags_current` rows
3. No explicit tag cleanup needed in the delete handler

During replay, the same cascade applies. Tags are rebuilt from `item.tagged`/`item.untagged` events, and cleaned up when `item.deleted` events remove the parent item.

---

## Alternatives Considered

### A. JSON array column on items_current

Store tags as `TEXT` (JSON array) directly in `items_current`.

**Pros**: No new table, simpler schema
**Cons**: Cannot index individual tags efficiently, searching requires JSON functions or `LIKE` patterns, harder to query "all items with tag X"

**Rejected**: Poor query performance for the primary use case (search by tag).

### B. Tags as a separate entity with its own lifecycle events

Create `tag.created`, `tag.deleted` events and a `tags_current` projection, then link items to tags.

**Pros**: Full lifecycle tracking, could add tag descriptions/metadata
**Cons**: Over-engineered for simple string tags, more events to manage, more validation

**Rejected**: Violates simplicity principle. Tags are just strings, not first-class entities.

### C. Include tags in item.created event

Add a `tags` field to the `item.created` payload.

**Pros**: Tags set at creation time
**Cons**: Cannot modify tags without a separate event anyway, pollutes existing event schema

**Rejected**: Does not solve the general case of adding/removing tags over time.

---

## Testing Strategy

1. **Unit tests**: Tag validation (empty, colon, canonicalization)
2. **Projection tests**: Apply `item.tagged`/`item.untagged` events, verify `item_tags_current` state
3. **Replay tests**: Full rebuild produces correct tag state
4. **Cascade tests**: `item.deleted` cleans up tags
5. **Idempotency tests**: Duplicate tag add is no-op, removing non-existent tag is no-op
6. **Query tests**: Search by tag returns correct items

---

**Version**: 1.0
**Complexity**: Simple
