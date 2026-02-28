# Tag System Architecture

**Created**: 2026-02-20
**Status**: Design proposal
**Scope**: Add many-to-many tag system to wherehouse

---

## Overview

Add flexible tagging system for items with event-sourced approach. Tags enable filtering/searching by attributes (e.g., "power-tool", "metric", "borrowed").

---

## Events

### tag.created
```json
{
  "event_type": "tag.created",
  "tag_name": "power-tool",
  "canonical_name": "power_tool",
  "note": "optional description"
}
```

**Projection**: Insert into `tags_current`

**Validation**:
- `canonical_name` must be unique (global)
- `canonical_name` must not contain `:`
- `tag_name` must not be empty

---

### item.tagged
```json
{
  "event_type": "item.tagged",
  "item_id": "uuid",
  "tag_name": "power-tool"
}
```

**Projection**: Insert into `item_tags_current`

**Validation**:
- `item_id` must exist
- `tag_name` must exist in `tags_current`
- No duplicate (item_id, tag_name) pair

---

### item.untagged
```json
{
  "event_type": "item.untagged",
  "item_id": "uuid",
  "tag_name": "power-tool"
}
```

**Projection**: Delete from `item_tags_current`

**Validation**:
- `item_id` must exist
- `tag_name` must exist
- Tag association must exist

---

### tag.deleted
```json
{
  "event_type": "tag.deleted",
  "tag_name": "obsolete-tag"
}
```

**Projection**: Delete from `tags_current` and cascade delete `item_tags_current`

**Validation**:
- `tag_name` must exist
- Warn if tag has associations (but allow deletion)

---

## Projections

### tags_current
```sql
CREATE TABLE tags_current (
  tag_name          TEXT PRIMARY KEY,
  canonical_name    TEXT NOT NULL UNIQUE,
  updated_at        TEXT NOT NULL
);

CREATE INDEX idx_tags_canonical ON tags_current(canonical_name);
```

---

### item_tags_current
```sql
CREATE TABLE item_tags_current (
  item_id     TEXT NOT NULL,
  tag_name    TEXT NOT NULL,
  updated_at  TEXT NOT NULL,

  PRIMARY KEY (item_id, tag_name),
  FOREIGN KEY (item_id) REFERENCES items_current(item_id) ON DELETE CASCADE,
  FOREIGN KEY (tag_name) REFERENCES tags_current(tag_name) ON DELETE CASCADE
);

CREATE INDEX idx_item_tags_by_item ON item_tags_current(item_id);
CREATE INDEX idx_item_tags_by_tag ON item_tags_current(tag_name);
```

**Cascade Behavior**: When item deleted, tags auto-removed by FK constraint

---

## CLI Integration

### Commands
```bash
wherehouse tag create "power-tool"           # Create tag
wherehouse tag add <ITEM> power-tool         # Tag item
wherehouse tag remove <ITEM> power-tool      # Untag item
wherehouse tag list                          # List all tags
wherehouse tag delete power-tool             # Delete tag

wherehouse find --tag power-tool             # Find items with tag
wherehouse info <ITEM>                       # Show tags in item details
```

---

## Query Patterns

**Find items by tag**:
```sql
SELECT i.* FROM items_current i
JOIN item_tags_current t ON i.item_id = t.item_id
WHERE t.tag_name = 'power-tool';
```

**Find items with multiple tags (AND)**:
```sql
SELECT i.* FROM items_current i
WHERE EXISTS (
  SELECT 1 FROM item_tags_current t1
  WHERE t1.item_id = i.item_id AND t1.tag_name = 'power-tool'
) AND EXISTS (
  SELECT 1 FROM item_tags_current t2
  WHERE t2.item_id = i.item_id AND t2.tag_name = 'metric'
);
```

**Count items per tag**:
```sql
SELECT t.tag_name, COUNT(it.item_id) as count
FROM tags_current t
LEFT JOIN item_tags_current it ON t.tag_name = it.tag_name
GROUP BY t.tag_name;
```

---

## Design Decisions

**Tag names as PKs vs UUIDs**: Use tag_name directly. Simpler, user-friendly, no renames expected.

**Canonical uniqueness**: Global uniqueness prevents ambiguity. Tags like "metric" vs "Metric" become same tag.

**No tag hierarchy**: Flat structure for v1. Future: consider parent tags if needed.

**Cascade on item deletion**: When item deleted, tags auto-removed. Event log retains history.

**No rename operation**: Delete + recreate. Tags are lightweight identifiers.

---

## Trade-offs

**Pros**:
- Simple many-to-many with standard SQL
- Event-sourced maintains full history
- Efficient queries with proper indexes
- FK cascades handle cleanup

**Cons**:
- No tag rename (must delete/recreate)
- Global tag namespace (could add prefixes if needed)
- Tag deletion orphans associations in event log (acceptable - replay still works)

---

## Implementation Notes

**Event storage**: Add `tag_name` indexed column to events table for fast tag history queries.

**Replay**: Tags processed after items/locations during rebuild. Dependencies: items → tags → item_tags.

**Validation timing**: Check tag existence before creating item.tagged event. Fail fast on missing tag or item.

---

## Future Extensions

- Tag descriptions/metadata in tags_current
- Tag colors for UI
- Tag prefixes (e.g., "category:power-tool")
- Tag usage counts (derived or cached)
