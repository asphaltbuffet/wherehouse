-- Replace item/location split with unified entity model.
-- Migration: 000006

-- Drop old projection tables.
DROP TABLE IF EXISTS items_current;
DROP TABLE IF EXISTS locations_current;

-- Create unified entity projection.
CREATE TABLE entities_current (
    entity_id           TEXT PRIMARY KEY NOT NULL,
    display_name        TEXT NOT NULL,
    canonical_name      TEXT NOT NULL,
    entity_type         TEXT NOT NULL CHECK (entity_type IN ('place', 'container', 'leaf')),
    parent_id           TEXT,
    full_path_display   TEXT NOT NULL,
    full_path_canonical TEXT NOT NULL,
    depth               INTEGER NOT NULL DEFAULT 0 CHECK (depth >= 0),
    status              TEXT NOT NULL DEFAULT 'ok' CHECK (status IN ('ok', 'borrowed', 'missing', 'loaned', 'removed')),
    status_context      TEXT,
    last_event_id       INTEGER NOT NULL,
    updated_at          TEXT NOT NULL,
    FOREIGN KEY (parent_id) REFERENCES entities_current(entity_id)
);

CREATE INDEX idx_entities_canonical_name ON entities_current(canonical_name);
CREATE INDEX idx_entities_parent_id ON entities_current(parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX idx_entities_status ON entities_current(status);
CREATE INDEX idx_entities_entity_type ON entities_current(entity_type);

-- Recreate events table to replace item_id/location_id/project_id with entity_id.
-- (SQLite requires table recreation for structural column changes.)
CREATE TABLE events_new (
    event_id         INTEGER PRIMARY KEY AUTOINCREMENT,
    event_type       TEXT NOT NULL,
    timestamp_utc    TEXT NOT NULL,
    actor_user_id    TEXT NOT NULL,
    payload          TEXT NOT NULL,
    note             TEXT,
    entity_id        TEXT
);

-- Copy existing events (entity_id will be NULL for old events -- that's fine).
INSERT INTO events_new (event_id, event_type, timestamp_utc, actor_user_id, payload, note, entity_id)
SELECT event_id, event_type, timestamp_utc, actor_user_id, payload, note, NULL
FROM events;

-- Swap tables.
DROP TABLE events;
ALTER TABLE events_new RENAME TO events;

-- Recreate indexes on events.
CREATE INDEX idx_events_type ON events(event_type);
CREATE INDEX idx_events_timestamp ON events(timestamp_utc);
CREATE INDEX idx_events_entity_id ON events(entity_id) WHERE entity_id IS NOT NULL;
