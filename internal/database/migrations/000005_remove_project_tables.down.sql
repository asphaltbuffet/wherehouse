-- Rollback: restore project-related schema
-- Migration: 000005 (down)

-- Restore projects_current table
CREATE TABLE projects_current (
    project_id  TEXT PRIMARY KEY,
    status      TEXT NOT NULL,
    updated_at  TEXT NOT NULL,
    CHECK (status IN ('active', 'completed'))
);

CREATE INDEX idx_projects_status ON projects_current(status);

-- Restore project_id column in events
ALTER TABLE events ADD COLUMN project_id TEXT;
CREATE INDEX idx_events_project_id ON events(project_id) WHERE project_id IS NOT NULL;

-- Restore project_id in items_current
CREATE TABLE items_current_new (
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

INSERT INTO items_current_new
    SELECT item_id, display_name, canonical_name, location_id,
           in_temporary_use, temp_origin_location_id, NULL, last_event_id, updated_at
    FROM items_current;

DROP TABLE items_current;
ALTER TABLE items_current_new RENAME TO items_current;

CREATE INDEX idx_items_canonical_name ON items_current(canonical_name);
CREATE INDEX idx_items_location_id ON items_current(location_id);
CREATE INDEX idx_items_project_id ON items_current(project_id) WHERE project_id IS NOT NULL;
CREATE INDEX idx_items_in_temp_use ON items_current(in_temporary_use) WHERE in_temporary_use = 1;
CREATE INDEX idx_items_location_canonical ON items_current(location_id, canonical_name);
