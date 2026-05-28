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
// GetEntity retrieves a single non-removed entity by ID.
func (s *Store) GetEntity(ctx context.Context, entityID string) (*inventory.Entity, error) {
	const query = `
		SELECT entity_id, display_name, canonical_name, entity_type,
		       parent_id, full_path_display, full_path_canonical,
		       depth, status, status_context, last_event_id, updated_at
		FROM entities_current WHERE entity_id = ? AND status != 'removed'`

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
// GetEntitiesByCanonicalName retrieves all non-removed entities with a given canonical name,
// ordered by full_path_canonical ASC, entity_id ASC.
func (s *Store) GetEntitiesByCanonicalName(ctx context.Context, canonical string) ([]*inventory.Entity, error) {
	const query = `
		SELECT entity_id, display_name, canonical_name, entity_type,
		       parent_id, full_path_display, full_path_canonical,
		       depth, status, status_context, last_event_id, updated_at
		FROM entities_current WHERE canonical_name = ? AND status != 'removed'
		ORDER BY full_path_canonical ASC, entity_id ASC`

	rows, err := s.db.QueryContext(ctx, query, canonical)
	if err != nil {
		return nil, fmt.Errorf("query by canonical name %q: %w", canonical, err)
	}
	defer rows.Close()
	return scanEntities(rows)
}

// ChildRow is the result of GetChildren: the entity plus whether it has non-removed children of its own.
type ChildRow struct {
	Entity      *inventory.Entity
	HasChildren bool
}

// GetChildren retrieves direct non-removed children of a parent entity, each annotated
// with whether it has non-removed children of its own. Ordered by display_name ASC, entity_id ASC.
func (s *Store) GetChildren(ctx context.Context, parentID string) ([]ChildRow, error) {
	const query = `
		SELECT ec.entity_id, ec.display_name, ec.canonical_name, ec.entity_type,
		       ec.parent_id, ec.full_path_display, ec.full_path_canonical,
		       ec.depth, ec.status, ec.status_context, ec.last_event_id, ec.updated_at,
		       EXISTS (
		           SELECT 1 FROM entities_current c
		           WHERE c.parent_id = ec.entity_id AND c.status != 'removed'
		       ) AS has_children
		FROM entities_current ec
		WHERE ec.parent_id = ? AND ec.status != 'removed'
		ORDER BY ec.display_name ASC, ec.entity_id ASC`

	rows, err := s.db.QueryContext(ctx, query, parentID)
	if err != nil {
		return nil, fmt.Errorf("get children of %s: %w", parentID, err)
	}
	defer rows.Close()
	return scanChildRows(rows)
}

// GetDescendants retrieves all descendants using path prefix matching,
// ordered by depth ASC, display_name ASC, entity_id ASC.
// GetDescendants retrieves all non-removed descendants using path prefix matching,
// ordered by depth ASC, display_name ASC, entity_id ASC.
func (s *Store) GetDescendants(ctx context.Context, entityID string) ([]*inventory.Entity, error) {
	parent, err := s.GetEntity(ctx, entityID)
	if err != nil {
		return nil, err
	}

	const query = `
		SELECT entity_id, display_name, canonical_name, entity_type,
		       parent_id, full_path_display, full_path_canonical,
		       depth, status, status_context, last_event_id, updated_at
		FROM entities_current WHERE full_path_canonical LIKE ? ESCAPE '\' AND status != 'removed'
		ORDER BY depth ASC, display_name ASC, entity_id ASC`

	rows, err := s.db.QueryContext(ctx, query, escapeLIKE(parent.FullPathCanonical)+":%")
	if err != nil {
		return nil, fmt.Errorf("get descendants of %s: %w", entityID, err)
	}
	defer rows.Close()
	return scanEntities(rows)
}

// ListEntities retrieves all entities ordered by full_path_display ASC, entity_id ASC.
// ListEntities retrieves all non-removed entities ordered by full_path_display ASC, entity_id ASC.
func (s *Store) ListEntities(ctx context.Context) ([]*inventory.Entity, error) {
	const query = `
		SELECT entity_id, display_name, canonical_name, entity_type,
		       parent_id, full_path_display, full_path_canonical,
		       depth, status, status_context, last_event_id, updated_at
		FROM entities_current
		WHERE status != 'removed'
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

func scanChildRows(rows *sql.Rows) ([]ChildRow, error) {
	var children []ChildRow
	for rows.Next() {
		var hasChildren bool
		e, err := scanEntity(func(dest ...any) error {
			return rows.Scan(append(dest, &hasChildren)...)
		})
		if err != nil {
			return nil, fmt.Errorf("scan child row: %w", err)
		}
		children = append(children, ChildRow{Entity: e, HasChildren: hasChildren})
	}
	return children, rows.Err()
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
		FROM entities_current WHERE full_path_canonical LIKE ? ESCAPE '\'
		ORDER BY depth ASC, display_name ASC, entity_id ASC`

	rows, err := tx.QueryContext(ctx, query, escapeLIKE(prefix)+":%")
	if err != nil {
		return nil, fmt.Errorf("get descendants of %s: %w", entityID, err)
	}
	defer rows.Close()
	return scanEntities(rows)
}

func escapeLIKE(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
