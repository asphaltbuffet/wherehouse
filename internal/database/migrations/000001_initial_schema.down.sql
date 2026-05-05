-- Rollback migration 000001: drop all application tables and indexes.

DROP INDEX IF EXISTS idx_entities_entity_type;
DROP INDEX IF EXISTS idx_entities_status;
DROP INDEX IF EXISTS idx_entities_parent_id;
DROP INDEX IF EXISTS idx_entities_canonical_name;
DROP TABLE IF EXISTS entities_current;
DROP INDEX IF EXISTS idx_events_entity_id;
DROP INDEX IF EXISTS idx_events_timestamp;
DROP INDEX IF EXISTS idx_events_type;
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS schema_metadata;
