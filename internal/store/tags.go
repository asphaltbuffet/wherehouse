package store

import (
	"context"
	"fmt"
	"strings"
)

// InsertTagTx inserts a tag for entityID within the given transaction. Duplicate inserts are a no-op.
func (s *Store) InsertTagTx(ctx context.Context, tx Tx, entityID, tag string) error {
	_, err := tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO entity_tags (entity_id, tag) VALUES (?, ?)`,
		entityID, tag,
	)
	if err != nil {
		return fmt.Errorf("insert tag %q for entity %q: %w", tag, entityID, err)
	}
	return nil
}

// DeleteTagTx deletes a tag for entityID within the given transaction. Deleting a missing tag is a no-op.
func (s *Store) DeleteTagTx(ctx context.Context, tx Tx, entityID, tag string) error {
	_, err := tx.ExecContext(ctx,
		`DELETE FROM entity_tags WHERE entity_id = ? AND tag = ?`,
		entityID, tag,
	)
	if err != nil {
		return fmt.Errorf("delete tag %q for entity %q: %w", tag, entityID, err)
	}
	return nil
}

// GetTagsByEntity returns all tags for entityID, sorted alphabetically.
func (s *Store) GetTagsByEntity(ctx context.Context, entityID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT tag FROM entity_tags WHERE entity_id = ? ORDER BY tag ASC`,
		entityID,
	)
	if err != nil {
		return nil, fmt.Errorf("get tags for entity %q: %w", entityID, err)
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if scanErr := rows.Scan(&tag); scanErr != nil {
			return nil, fmt.Errorf("scan tag: %w", scanErr)
		}
		tags = append(tags, tag)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tags: %w", err)
	}
	if tags == nil {
		return []string{}, nil
	}
	return tags, nil
}

// TruncateTagsTx deletes all rows from entity_tags within the given transaction.
func (s *Store) TruncateTagsTx(ctx context.Context, tx Tx) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM entity_tags`); err != nil {
		return fmt.Errorf("truncate entity_tags: %w", err)
	}
	return nil
}

// GetTagsByEntities returns a map of entityID → sorted tag slice for all given IDs.
// IDs absent from entity_tags are not present in the returned map (nil slice on lookup).
// Returns an empty map immediately when entityIDs is empty.
func (s *Store) GetTagsByEntities(ctx context.Context, entityIDs []string) (map[string][]string, error) {
	if len(entityIDs) == 0 {
		return map[string][]string{}, nil
	}

	placeholders := strings.Repeat("?,", len(entityIDs)-1) + "?"
	//nolint:gosec // placeholders is "?,?,..." — all values are parameterized, no user input in the query string
	q := `SELECT entity_id, tag FROM entity_tags WHERE entity_id IN (` + placeholders + `) ORDER BY entity_id, tag ASC`

	args := make([]any, len(entityIDs))
	for i, id := range entityIDs {
		args[i] = id
	}

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("get tags for entities: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var entityID, tag string
		if scanErr := rows.Scan(&entityID, &tag); scanErr != nil {
			return nil, fmt.Errorf("scan tag row: %w", scanErr)
		}
		result[entityID] = append(result[entityID], tag)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tag rows: %w", err)
	}
	return result, nil
}
