-- Rollback migration 006: remove entities_current and restore old projection tables.
-- Note: data cannot be recovered from entities_current back to items_current/locations_current.
-- This down migration restores the schema structure only (empty tables).

DROP TABLE IF EXISTS entities_current;

-- Restore locations_current so downstream down migrations (000005, etc.) can run.
CREATE TABLE IF NOT EXISTS locations_current (
    location_id          TEXT PRIMARY KEY,
    display_name         TEXT NOT NULL,
    canonical_name       TEXT NOT NULL,
    parent_id            TEXT,
    full_path_display    TEXT NOT NULL,
    full_path_canonical  TEXT NOT NULL,
    depth                INTEGER NOT NULL DEFAULT 0,
    is_system            INTEGER NOT NULL DEFAULT 0,
    updated_at           TEXT NOT NULL,
    FOREIGN KEY (parent_id) REFERENCES locations_current(location_id)
);

-- Restore items_current so downstream down migrations (000005, etc.) can run.
CREATE TABLE IF NOT EXISTS items_current (
    item_id                  TEXT PRIMARY KEY,
    display_name             TEXT NOT NULL,
    canonical_name           TEXT NOT NULL,
    location_id              TEXT NOT NULL,
    in_temporary_use         INTEGER NOT NULL DEFAULT 0,
    temp_origin_location_id  TEXT,
    last_event_id            INTEGER NOT NULL,
    updated_at               TEXT NOT NULL,
    FOREIGN KEY (location_id) REFERENCES locations_current(location_id),
    FOREIGN KEY (temp_origin_location_id) REFERENCES locations_current(location_id),
    CHECK (in_temporary_use IN (0, 1))
);

-- Recreate events table to restore item_id/location_id columns.
CREATE TABLE events_old (
    event_id         INTEGER PRIMARY KEY AUTOINCREMENT,
    event_type       TEXT NOT NULL,
    timestamp_utc    TEXT NOT NULL,
    actor_user_id    TEXT NOT NULL,
    payload          TEXT NOT NULL,
    note             TEXT,
    item_id          TEXT,
    location_id      TEXT
);

INSERT INTO events_old (event_id, event_type, timestamp_utc, actor_user_id, payload, note, item_id, location_id)
SELECT event_id, event_type, timestamp_utc, actor_user_id, payload, note, NULL, NULL
FROM events;

DROP TABLE events;
ALTER TABLE events_old RENAME TO events;

CREATE INDEX idx_events_type ON events(event_type);
CREATE INDEX idx_events_timestamp ON events(timestamp_utc);
