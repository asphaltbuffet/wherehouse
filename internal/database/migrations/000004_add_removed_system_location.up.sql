-- Add Removed system location for tracking items/locations that have been removed
-- Migration: 000004
-- Created: 2026-04-06

-- Insert Removed system location (if it doesn't already exist)
-- Using INSERT OR IGNORE for idempotency
INSERT OR IGNORE INTO locations_current (
    location_id,
    display_name,
    canonical_name,
    parent_id,
    full_path_display,
    full_path_canonical,
    depth,
    is_system,
    updated_at
) VALUES (
    'sys0000004',
    'Removed',
    'removed',
    NULL,
    'Removed',
    'removed',
    0,
    1,
    CURRENT_TIMESTAMP
);
