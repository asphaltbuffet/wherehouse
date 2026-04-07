-- Remove project-related schema
-- Migration: 000005
-- Created: 2026-04-06
--
-- Projects were never implemented. This migration removes:
--   - projects_current table
--   - project_id column from items_current
--   - project_id column from events
--
-- SQLite ALTER TABLE DROP COLUMN requires 3.35.0+.
-- We recreate tables without the column where DROP COLUMN is not usable
-- (items_current has a FK + CHECK constraint referencing project_id).

-- Step 1: Drop the index on items_current.project_id
DROP INDEX IF EXISTS idx_items_project_id;

-- Step 2: Recreate items_current without project_id
--         (SQLite cannot drop a column that is referenced in a FOREIGN KEY)
CREATE TABLE items_current_new (
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

INSERT INTO items_current_new
    SELECT item_id, display_name, canonical_name, location_id,
           in_temporary_use, temp_origin_location_id, last_event_id, updated_at
    FROM items_current;

DROP TABLE items_current;
ALTER TABLE items_current_new RENAME TO items_current;

-- Restore indexes
CREATE INDEX idx_items_canonical_name ON items_current(canonical_name);
CREATE INDEX idx_items_location_id ON items_current(location_id);
CREATE INDEX idx_items_in_temp_use ON items_current(in_temporary_use) WHERE in_temporary_use = 1;
CREATE INDEX idx_items_location_canonical ON items_current(location_id, canonical_name);

-- Step 3: Drop the index on events.project_id, then remove the column
DROP INDEX IF EXISTS idx_events_project_id;
ALTER TABLE events DROP COLUMN project_id;

-- Step 4: Drop the projects_current table (and its index)
DROP INDEX IF EXISTS idx_projects_status;
DROP TABLE IF EXISTS projects_current;
