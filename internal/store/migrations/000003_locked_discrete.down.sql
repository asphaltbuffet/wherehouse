-- Reverse migration: restore entity_type column, set all rows to 'container'.
-- Note: original type information is not recoverable from locked/discrete alone.

CREATE TABLE entities_current_old (
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
    FOREIGN KEY (parent_id) REFERENCES entities_current_old(entity_id)
);

INSERT INTO entities_current_old (
    entity_id, display_name, canonical_name,
    entity_type,
    parent_id, full_path_display, full_path_canonical,
    depth, status, status_context, last_event_id, updated_at
)
SELECT
    entity_id, display_name, canonical_name,
    CASE WHEN discrete = 1 THEN 'leaf' WHEN locked = 1 THEN 'place' ELSE 'container' END AS entity_type,
    parent_id, full_path_display, full_path_canonical,
    depth, status, status_context, last_event_id, updated_at
FROM entities_current;

DROP TABLE entities_current;
ALTER TABLE entities_current_old RENAME TO entities_current;

CREATE INDEX idx_entities_canonical_name ON entities_current(canonical_name);
CREATE INDEX idx_entities_parent_id ON entities_current(parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX idx_entities_status ON entities_current(status);
CREATE INDEX idx_entities_entity_type ON entities_current(entity_type);
