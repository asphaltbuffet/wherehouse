-- Migration 000003: ID format migration to nanoid
--
-- This migration serves as a schema version marker only.
-- No DDL changes are required because all ID columns are TEXT (format-agnostic).
--
-- The actual data transformation (rewriting UUID IDs to nanoid IDs) is performed
-- by the Go command: wherehouse migrate database
--
-- Running `wherehouse migrate database` is opt-in and must be done separately.
-- The application continues to work after this schema version is applied, with
-- new entities receiving nanoid IDs while old entities retain their UUID IDs
-- until the migration command is run.

SELECT 1; -- no-op version marker
