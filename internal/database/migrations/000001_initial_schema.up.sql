-- Initial Schema for Wherehouse Event-Sourced Inventory System
-- Migration: 000001
-- Created: 2026-02-21

-- Events table (source of truth)
CREATE TABLE events (
    event_id         INTEGER PRIMARY KEY AUTOINCREMENT,
    event_type       TEXT NOT NULL,
    timestamp_utc    TEXT NOT NULL,
    actor_user_id    TEXT NOT NULL,
    payload          TEXT NOT NULL,
    note             TEXT,
    item_id          TEXT,
    location_id      TEXT,
    project_id       TEXT
);

CREATE INDEX idx_events_type ON events(event_type);
CREATE INDEX idx_events_timestamp ON events(timestamp_utc);
CREATE INDEX idx_events_item_id ON events(item_id) WHERE item_id IS NOT NULL;
CREATE INDEX idx_events_location_id ON events(location_id) WHERE location_id IS NOT NULL;
CREATE INDEX idx_events_project_id ON events(project_id) WHERE project_id IS NOT NULL;

-- Locations projection
CREATE TABLE locations_current (
    location_id          TEXT PRIMARY KEY,
    display_name         TEXT NOT NULL,
    canonical_name       TEXT NOT NULL,
    parent_id            TEXT,
    full_path_display    TEXT NOT NULL,
    full_path_canonical  TEXT NOT NULL,
    depth                INTEGER NOT NULL,
    is_system            INTEGER NOT NULL DEFAULT 0,
    updated_at           TEXT NOT NULL,
    FOREIGN KEY (parent_id) REFERENCES locations_current(location_id),
    CHECK (is_system IN (0, 1)),
    CHECK (depth >= 0)
);

CREATE UNIQUE INDEX idx_locations_canonical_parent ON locations_current(canonical_name, IFNULL(parent_id, ''));
CREATE INDEX idx_locations_parent_id ON locations_current(parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX idx_locations_full_path_canonical ON locations_current(full_path_canonical);
CREATE INDEX idx_locations_is_system ON locations_current(is_system) WHERE is_system = 1;

-- Items projection
CREATE TABLE items_current (
    item_id                  TEXT PRIMARY KEY,
    display_name             TEXT NOT NULL,
    canonical_name           TEXT NOT NULL,
    location_id              TEXT NOT NULL,
    in_temporary_use         INTEGER NOT NULL DEFAULT 0,
    temp_origin_location_id  TEXT,
    project_id               TEXT,
    last_event_id            INTEGER NOT NULL,
    updated_at               TEXT NOT NULL,
    FOREIGN KEY (location_id) REFERENCES locations_current(location_id),
    FOREIGN KEY (temp_origin_location_id) REFERENCES locations_current(location_id),
    FOREIGN KEY (project_id) REFERENCES projects_current(project_id),
    CHECK (in_temporary_use IN (0, 1))
);

CREATE INDEX idx_items_canonical_name ON items_current(canonical_name);
CREATE INDEX idx_items_location_id ON items_current(location_id);
CREATE INDEX idx_items_project_id ON items_current(project_id) WHERE project_id IS NOT NULL;
CREATE INDEX idx_items_in_temp_use ON items_current(in_temporary_use) WHERE in_temporary_use = 1;
CREATE INDEX idx_items_location_canonical ON items_current(location_id, canonical_name);

-- Projects projection
CREATE TABLE projects_current (
    project_id  TEXT PRIMARY KEY,
    status      TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    CHECK (status IN ('active', 'completed'))
);

CREATE INDEX idx_projects_status ON projects_current(status);

-- Application metadata
CREATE TABLE schema_metadata (
    key    TEXT PRIMARY KEY,
    value  TEXT NOT NULL
);

INSERT INTO schema_metadata (key, value) VALUES
    ('created_at', CURRENT_TIMESTAMP),
    ('app_version', '1.0.0');
