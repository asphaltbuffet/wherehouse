package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// seedSystemLocations creates the system locations (Missing, Borrowed) if they don't exist.
// This is called after the initial migration runs.
// It's idempotent - safe to call multiple times.
func (d *Database) seedSystemLocations(ctx context.Context) error {
	return d.ExecInTransaction(ctx, func(tx *sql.Tx) error {
		// Check if system locations already exist
		var count int
		err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM locations_current WHERE is_system = 1").Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check for system locations: %w", err)
		}

		// Already seeded
		if count > 0 {
			return nil
		}

		now := time.Now().UTC().Format(time.RFC3339)

		// Create Missing location
		missingID := uuid.NewString()
		_, err = tx.ExecContext(ctx, `
			INSERT INTO locations_current (
				location_id, display_name, canonical_name,
				parent_id, full_path_display, full_path_canonical,
				depth, is_system, updated_at
			) VALUES (?, ?, ?, NULL, ?, ?, 0, 1, ?)`,
			missingID, "Missing", "missing", "Missing", "missing", now,
		)
		if err != nil {
			return fmt.Errorf("failed to create Missing location: %w", err)
		}

		// Create Borrowed location
		borrowedID := uuid.NewString()
		_, err = tx.ExecContext(ctx, `
			INSERT INTO locations_current (
				location_id, display_name, canonical_name,
				parent_id, full_path_display, full_path_canonical,
				depth, is_system, updated_at
			) VALUES (?, ?, ?, NULL, ?, ?, 0, 1, ?)`,
			borrowedID, "Borrowed", "borrowed", "Borrowed", "borrowed", now,
		)
		if err != nil {
			return fmt.Errorf("failed to create Borrowed location: %w", err)
		}

		return nil
	})
}

// GetMetadata retrieves a value from schema_metadata by key.
func (d *Database) GetMetadata(ctx context.Context, key string) (string, error) {
	var value string
	err := d.db.QueryRowContext(ctx, "SELECT value FROM schema_metadata WHERE key = ?", key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("metadata key %q not found", key)
		}
		return "", fmt.Errorf("failed to get metadata: %w", err)
	}
	return value, nil
}

// SetMetadata sets a value in schema_metadata.
func (d *Database) SetMetadata(ctx context.Context, key, value string) error {
	_, err := d.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO schema_metadata (key, value)
		VALUES (?, ?)
	`, key, value)
	if err != nil {
		return fmt.Errorf("failed to set metadata: %w", err)
	}
	return nil
}
