package database

import (
	"context"
	"database/sql"
	"fmt"
)

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
