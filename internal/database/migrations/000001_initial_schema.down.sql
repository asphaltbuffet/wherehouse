-- Rollback Initial Schema
-- Migration: 000001
-- Created: 2026-02-21

DROP TABLE IF EXISTS schema_metadata;
DROP TABLE IF EXISTS items_current;
DROP TABLE IF EXISTS locations_current;
DROP TABLE IF EXISTS projects_current;
DROP TABLE IF EXISTS events;
