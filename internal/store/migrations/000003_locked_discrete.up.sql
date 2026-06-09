-- Migration 000003: replace entity_type with locked and discrete flags.
-- SQLite does not support DROP COLUMN before 3.35, so we rebuild the table.

CREATE TABLE entities_current_new (
    entity_id           TEXT PRIMARY KEY NOT NULL,
    display_name        TEXT NOT NULL,
    canonical_name      TEXT NOT NULL,
    locked              INTEGER NOT NULL DEFAULT 0,
    discrete            INTEGER NOT NULL DEFAULT 0,
    parent_id           TEXT,
    full_path_display   TEXT NOT NULL,
    full_path_canonical TEXT NOT NULL,
    depth               INTEGER NOT NULL DEFAULT 0 CHECK (depth >= 0),
    status              TEXT NOT NULL DEFAULT 'ok' CHECK (status IN ('ok', 'borrowed', 'missing', 'loaned', 'removed')),
    status_context      TEXT,
    last_event_id       INTEGER NOT NULL,
    updated_at          TEXT NOT NULL,
    FOREIGN KEY (parent_id) REFERENCES entities_current_new(entity_id)
);

INSERT INTO entities_current_new (
    entity_id, display_name, canonical_name,
    locked, discrete,
    parent_id, full_path_display, full_path_canonical,
    depth, status, status_context, last_event_id, updated_at
)
SELECT
    entity_id, display_name, canonical_name,
    CASE WHEN entity_type = 'place' THEN 1 ELSE 0 END AS locked,
    CASE WHEN entity_type = 'leaf' THEN 1 ELSE 0 END AS discrete,
    parent_id, full_path_display, full_path_canonical,
    depth, status, status_context, last_event_id, updated_at
FROM entities_current;

DROP TABLE entities_current;
ALTER TABLE entities_current_new RENAME TO entities_current;

CREATE INDEX idx_entities_canonical_name ON entities_current(canonical_name);
CREATE INDEX idx_entities_parent_id ON entities_current(parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX idx_entities_status ON entities_current(status);
CREATE INDEX idx_entities_locked ON entities_current(locked) WHERE locked = 1;
