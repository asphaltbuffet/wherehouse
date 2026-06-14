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

// CountEntitiesByStatus returns the count of entities per status across all statuses.
// All five known statuses are always present in the returned map (zero-filled if absent).
func (s *Store) CountEntitiesByStatus(ctx context.Context) (map[string]int, error) {
	const query = `SELECT status, COUNT(*) FROM entities_current GROUP BY status`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("count entities by status: %w", err)
	}
	defer rows.Close()

	counts := map[string]int{
		"ok": 0, "missing": 0, "borrowed": 0, "loaned": 0, "removed": 0,
	}
	for rows.Next() {
		var status string
		var count int
		if scanErr := rows.Scan(&status, &count); scanErr != nil {
			return nil, fmt.Errorf("scan entity status count: %w", scanErr)
		}
		counts[status] = count
	}
	return counts, rows.Err()
}

// DeleteMetadata removes a key from the schema_metadata table. No-op if absent.
func (s *Store) DeleteMetadata(ctx context.Context, key string) error {
	const query = `DELETE FROM schema_metadata WHERE key = ?`
	_, err := s.db.ExecContext(ctx, query, key)
	if err != nil {
		return fmt.Errorf("delete metadata %q: %w", key, err)
	}
	return nil
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
