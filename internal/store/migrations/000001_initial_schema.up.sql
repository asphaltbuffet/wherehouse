-- Initial schema for Wherehouse (unified entity model).
-- Migration: 000001

CREATE TABLE events (
    event_id         INTEGER PRIMARY KEY AUTOINCREMENT,
    event_type       TEXT NOT NULL,
    timestamp_utc    TEXT NOT NULL,
    actor_user_id    TEXT NOT NULL,
    payload          TEXT NOT NULL,
    note             TEXT,
    entity_id        TEXT
);

CREATE INDEX idx_events_type ON events(event_type);
CREATE INDEX idx_events_timestamp ON events(timestamp_utc);
CREATE INDEX idx_events_entity_id ON events(entity_id) WHERE entity_id IS NOT NULL;

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

CREATE TABLE schema_metadata (
    key    TEXT PRIMARY KEY,
    value  TEXT NOT NULL
);

INSERT INTO schema_metadata (key, value) VALUES
    ('created_at', CURRENT_TIMESTAMP),
    ('app_version', '1.0.0');
