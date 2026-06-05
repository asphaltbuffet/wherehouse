-- Add entity_tags projection table for tag classification labels.
-- Migration: 000002

CREATE TABLE entity_tags (
    entity_id TEXT NOT NULL REFERENCES entities_current(entity_id),
    tag       TEXT NOT NULL,
    PRIMARY KEY (entity_id, tag)
);

CREATE INDEX idx_entity_tags_tag ON entity_tags(tag);
