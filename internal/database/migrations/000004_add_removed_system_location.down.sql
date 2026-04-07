-- Remove the Removed system location (rollback)
DELETE FROM locations_current WHERE location_id = 'sys0000004';
