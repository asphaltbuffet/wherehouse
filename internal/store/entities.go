package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// InsertEntityTx inserts a new entity projection row inside an existing transaction.
func (s *Store) InsertEntityTx(ctx context.Context, tx Tx, e *inventory.Entity) error {
	const query = `
		INSERT INTO entities_current (
			entity_id, display_name, canonical_name, entity_type,
			parent_id, full_path_display, full_path_canonical,
			depth, status, status_context, last_event_id, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := tx.ExecContext(ctx, query,
		e.EntityID, e.DisplayName, e.CanonicalName, e.EntityType,
		e.ParentID, e.FullPathDisplay, e.FullPathCanonical,
		e.Depth, e.Status, e.StatusContext, e.LastEventID,
		e.UpdatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert entity %s: %w", e.EntityID, err)
	}
	return nil
}

// UpdateEntityTx updates an existing entity projection row inside an existing transaction.
func (s *Store) UpdateEntityTx(ctx context.Context, tx Tx, e *inventory.Entity) error {
	const query = `
		UPDATE entities_current SET
			display_name = ?, canonical_name = ?, entity_type = ?,
			parent_id = ?, full_path_display = ?, full_path_canonical = ?,
			depth = ?, status = ?, status_context = ?,
			last_event_id = ?, updated_at = ?
		WHERE entity_id = ?`

	_, err := tx.ExecContext(ctx, query,
		e.DisplayName, e.CanonicalName, e.EntityType,
		e.ParentID, e.FullPathDisplay, e.FullPathCanonical,
		e.Depth, e.Status, e.StatusContext,
		e.LastEventID, e.UpdatedAt.UTC().Format(time.RFC3339),
		e.EntityID,
	)
	if err != nil {
		return fmt.Errorf("update entity %s: %w", e.EntityID, err)
	}
	return nil
}

// GetEntity retrieves a single entity by ID.
func (s *Store) GetEntity(ctx context.Context, entityID string) (*inventory.Entity, error) {
	const query = `
		SELECT entity_id, display_name, canonical_name, entity_type,
		       parent_id, full_path_display, full_path_canonical,
		       depth, status, status_context, last_event_id, updated_at
		FROM entities_current WHERE entity_id = ?`

	row := s.db.QueryRowContext(ctx, query, entityID)
	e, err := scanEntity(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get entity %s: %w", entityID, err)
	}
	return e, nil
}

// GetEntitiesByCanonicalName retrieves all entities with a given canonical name,
// ordered by full_path_canonical ASC, entity_id ASC.
func (s *Store) GetEntitiesByCanonicalName(ctx context.Context, canonical string) ([]*inventory.Entity, error) {
	const query = `
		SELECT entity_id, display_name, canonical_name, entity_type,
		       parent_id, full_path_display, full_path_canonical,
		       depth, status, status_context, last_event_id, updated_at
		FROM entities_current WHERE canonical_name = ?
		ORDER BY full_path_canonical ASC, entity_id ASC`

	rows, err := s.db.QueryContext(ctx, query, canonical)
	if err != nil {
		return nil, fmt.Errorf("query by canonical name %q: %w", canonical, err)
	}
	defer rows.Close()
	return scanEntities(rows)
}

// GetChildren retrieves direct children of a parent entity,
// ordered by display_name ASC, entity_id ASC.
func (s *Store) GetChildren(ctx context.Context, parentID string) ([]*inventory.Entity, error) {
	const query = `
		SELECT entity_id, display_name, canonical_name, entity_type,
		       parent_id, full_path_display, full_path_canonical,
		       depth, status, status_context, last_event_id, updated_at
		FROM entities_current WHERE parent_id = ?
		ORDER BY display_name ASC, entity_id ASC`

	rows, err := s.db.QueryContext(ctx, query, parentID)
	if err != nil {
		return nil, fmt.Errorf("get children of %s: %w", parentID, err)
	}
	defer rows.Close()
	return scanEntities(rows)
}

// GetDescendants retrieves all descendants using path prefix matching,
// ordered by depth ASC, display_name ASC, entity_id ASC.
func (s *Store) GetDescendants(ctx context.Context, entityID string) ([]*inventory.Entity, error) {
	parent, err := s.GetEntity(ctx, entityID)
	if err != nil {
		return nil, err
	}

	prefix := parent.FullPathCanonical + ":"
	const query = `
		SELECT entity_id, display_name, canonical_name, entity_type,
		       parent_id, full_path_display, full_path_canonical,
		       depth, status, status_context, last_event_id, updated_at
		FROM entities_current WHERE full_path_canonical LIKE ?
		ORDER BY depth ASC, display_name ASC, entity_id ASC`

	rows, err := s.db.QueryContext(ctx, query, prefix+"%")
	if err != nil {
		return nil, fmt.Errorf("get descendants of %s: %w", entityID, err)
	}
	defer rows.Close()
	return scanEntities(rows)
}

// ListEntities retrieves all entities ordered by full_path_display ASC, entity_id ASC.
func (s *Store) ListEntities(ctx context.Context) ([]*inventory.Entity, error) {
	const query = `
		SELECT entity_id, display_name, canonical_name, entity_type,
		       parent_id, full_path_display, full_path_canonical,
		       depth, status, status_context, last_event_id, updated_at
		FROM entities_current
		ORDER BY full_path_display ASC, entity_id ASC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list entities: %w", err)
	}
	defer rows.Close()
	return scanEntities(rows)
}

// ComputeEntityPathTx computes full_path_display, full_path_canonical, and depth
// for a new entity given its parent. Runs inside an existing transaction.
// ComputeEntityPathTx computes full_path_display, full_path_canonical, and depth
// for a new entity given its parent. Runs inside an existing transaction.
func (s *Store) ComputeEntityPathTx(
	ctx context.Context,
	tx Tx,
	displayName, canonicalName string,
	parentID *string,
) (string, string, int, error) {
	if parentID == nil {
		return displayName, canonicalName, 0, nil
	}

	const query = `
		SELECT full_path_display, full_path_canonical, depth
		FROM entities_current WHERE entity_id = ?`

	var parentDisplay, parentCanonical string
	var parentDepth int
	scanErr := tx.QueryRowContext(ctx, query, *parentID).
		Scan(&parentDisplay, &parentCanonical, &parentDepth)
	if scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			return "", "", 0, fmt.Errorf("parent entity %q: %w", *parentID, ErrNotFound)
		}
		return "", "", 0, fmt.Errorf("query parent %s: %w", *parentID, scanErr)
	}

	return strings.Join([]string{parentDisplay, displayName}, ":"),
		strings.Join([]string{parentCanonical, canonicalName}, ":"),
		parentDepth + 1,
		nil
}

type scanFunc func(dest ...any) error

func scanEntity(scan scanFunc) (*inventory.Entity, error) {
	var e inventory.Entity
	var updatedAtStr string
	if err := scan(
		&e.EntityID, &e.DisplayName, &e.CanonicalName, &e.EntityType,
		&e.ParentID, &e.FullPathDisplay, &e.FullPathCanonical,
		&e.Depth, &e.Status, &e.StatusContext, &e.LastEventID, &updatedAtStr,
	); err != nil {
		return nil, err
	}
	t, err := time.Parse(time.RFC3339, updatedAtStr)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at %q: %w", updatedAtStr, err)
	}
	e.UpdatedAt = t
	return &e, nil
}

func scanEntities(rows *sql.Rows) ([]*inventory.Entity, error) {
	var entities []*inventory.Entity
	for rows.Next() {
		e, err := scanEntity(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("scan entity: %w", err)
		}
		entities = append(entities, e)
	}
	return entities, rows.Err()
}

// GetEntityTx retrieves a single entity by ID inside an existing transaction.
func (s *Store) GetEntityTx(ctx context.Context, tx Tx, entityID string) (*inventory.Entity, error) {
	const query = `
		SELECT entity_id, display_name, canonical_name, entity_type,
		       parent_id, full_path_display, full_path_canonical,
		       depth, status, status_context, last_event_id, updated_at
		FROM entities_current WHERE entity_id = ?`

	row := tx.QueryRowContext(ctx, query, entityID)
	e, err := scanEntity(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get entity tx %s: %w", entityID, err)
	}
	return e, nil
}

// GetDescendantsTx retrieves all descendants of a given entity inside an existing transaction,
// using path prefix matching. Ordered by depth ASC, display_name ASC, entity_id ASC.
func (s *Store) GetDescendantsTx(ctx context.Context, tx Tx, entityID string) ([]*inventory.Entity, error) {
	const pathQuery = `SELECT full_path_canonical FROM entities_current WHERE entity_id = ?`
	var prefix string
	if err := tx.QueryRowContext(ctx, pathQuery, entityID).Scan(&prefix); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get path for %s: %w", entityID, err)
	}

	const query = `
		SELECT entity_id, display_name, canonical_name, entity_type,
		       parent_id, full_path_display, full_path_canonical,
		       depth, status, status_context, last_event_id, updated_at
		FROM entities_current WHERE full_path_canonical LIKE ?
		ORDER BY depth ASC, display_name ASC, entity_id ASC`

	rows, err := tx.QueryContext(ctx, query, prefix+":%")
	if err != nil {
		return nil, fmt.Errorf("get descendants of %s: %w", entityID, err)
	}
	defer rows.Close()
	return scanEntities(rows)
}
