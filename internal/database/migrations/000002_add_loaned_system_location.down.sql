-- Rollback: Remove Loaned system location
-- Migration: 000002
-- Created: 2026-02-25

-- Delete Loaned system location
-- Note: This will fail if there are items in the Loaned location (FK constraint)
DELETE FROM locations_current WHERE location_id = '00000000-0000-0000-0000-000000000003';
