package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// GetMetadata retrieves a value from the schema_metadata table by key.
func (s *Store) GetMetadata(ctx context.Context, key string) (string, error) {
	const query = `SELECT value FROM schema_metadata WHERE key = ?`
	var val string
	err := s.db.QueryRowContext(ctx, query, key).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get metadata %q: %w", key, err)
	}
	return val, nil
}

// SetMetadata upserts a key/value pair in the schema_metadata table.
func (s *Store) SetMetadata(ctx context.Context, key, value string) error {
	const query = `
		INSERT INTO schema_metadata (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`
	_, err := s.db.ExecContext(ctx, query, key, value)
	if err != nil {
		return fmt.Errorf("set metadata %q: %w", key, err)
	}
	return nil
}
