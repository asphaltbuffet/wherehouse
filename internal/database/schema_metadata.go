package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// seedSystemLocations creates the system locations (Missing, Borrowed, Loaned, Removed) if they don't exist.
// This is called after the initial migration runs.
// It's idempotent - safe to call multiple times.
// Uses INSERT OR IGNORE to handle upgrades where only some system locations exist.
func (d *Database) seedSystemLocations(ctx context.Context) error {
	return d.ExecInTransaction(ctx, func(tx *sql.Tx) error {
		now := time.Now().UTC().Format(time.RFC3339)

		// Deterministic IDs for system locations (same across all databases)
		const (
			missingID  = "sys0000001"
			borrowedID = "sys0000002"
			loanedID   = "sys0000003"
			removedID  = "sys0000004"
		)

		// Create Missing location (if not exists)
		_, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO locations_current (
				location_id, display_name, canonical_name,
				parent_id, full_path_display, full_path_canonical,
				depth, is_system, updated_at
			) VALUES (?, ?, ?, NULL, ?, ?, 0, 1, ?)`,
			missingID, "Missing", "missing", "Missing", "missing", now,
		)
		if err != nil {
			return fmt.Errorf("failed to create Missing location: %w", err)
		}

		// Create Borrowed location (if not exists)
		_, err = tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO locations_current (
				location_id, display_name, canonical_name,
				parent_id, full_path_display, full_path_canonical,
				depth, is_system, updated_at
			) VALUES (?, ?, ?, NULL, ?, ?, 0, 1, ?)`,
			borrowedID, "Borrowed", "borrowed", "Borrowed", "borrowed", now,
		)
		if err != nil {
			return fmt.Errorf("failed to create Borrowed location: %w", err)
		}

		// Create Loaned location (if not exists)
		_, err = tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO locations_current (
				location_id, display_name, canonical_name,
				parent_id, full_path_display, full_path_canonical,
				depth, is_system, updated_at
			) VALUES (?, ?, ?, NULL, ?, ?, 0, 1, ?)`,
			loanedID, "Loaned", "loaned", "Loaned", "loaned", now,
		)
		if err != nil {
			return fmt.Errorf("failed to create Loaned location: %w", err)
		}

		// Create Removed location (if not exists)
		_, err = tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO locations_current (
				location_id, display_name, canonical_name,
				parent_id, full_path_display, full_path_canonical,
				depth, is_system, updated_at
			) VALUES (?, ?, ?, NULL, ?, ?, 0, 1, ?)`,
			removedID, "Removed", "removed", "Removed", "removed", now,
		)
		if err != nil {
			return fmt.Errorf("failed to create Removed location: %w", err)
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
