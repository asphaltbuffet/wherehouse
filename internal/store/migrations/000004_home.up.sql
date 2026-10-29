-- Migration 000004: add home_id (designated parent; ADR 0039).
-- No REFERENCES clause: SQLite cannot DROP COLUMN a column that carries a
-- foreign key, and home integrity is enforced by app guards + doctor.
-- Backfill home = parent: before this migration no event could displace an
-- entity, so this equals what a full replay produces.
ALTER TABLE entities_current ADD COLUMN home_id TEXT;
UPDATE entities_current SET home_id = parent_id;
CREATE INDEX idx_entities_home_id ON entities_current(home_id) WHERE home_id IS NOT NULL;
