-- Add Loaned system location for tracking items loaned to other people
-- Migration: 000002
-- Created: 2026-02-25

-- Insert Loaned system location (if it doesn't already exist)
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
    '00000000-0000-0000-0000-000000000003',  -- Deterministic UUID for Loaned
    'Loaned',
    'loaned',
    NULL,
    'Loaned',
    'loaned',
    0,
    1,
    CURRENT_TIMESTAMP
);
