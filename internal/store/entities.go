package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// InsertEntity inserts a new entity projection row.
func (t Tx) InsertEntity(ctx context.Context, e *inventory.Entity) error {
	const query = `
		INSERT INTO entities_current (
			entity_id, display_name, canonical_name, locked, discrete,
			parent_id, full_path_display, full_path_canonical,
			depth, status, status_context, last_event_id, updated_at, home_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := t.q.ExecContext(ctx, query,
		e.EntityID, e.DisplayName, e.CanonicalName, e.Locked, e.Discrete,
		e.ParentID, e.FullPathDisplay, e.FullPathCanonical,
		e.Depth, e.Status, e.StatusContext, e.LastEventID,
		e.UpdatedAt.UTC().Format(time.RFC3339), e.HomeID,
	)
	if err != nil {
		return fmt.Errorf("insert entity %s: %w", e.EntityID, err)
	}
	return nil
}

// UpdateEntity updates an existing entity projection row.
func (t Tx) UpdateEntity(ctx context.Context, e *inventory.Entity) error {
	const query = `
		UPDATE entities_current SET
			display_name = ?, canonical_name = ?, locked = ?, discrete = ?,
			parent_id = ?, full_path_display = ?, full_path_canonical = ?,
			depth = ?, status = ?, status_context = ?,
			last_event_id = ?, updated_at = ?, home_id = ?
		WHERE entity_id = ?`

	_, err := t.q.ExecContext(ctx, query,
		e.DisplayName, e.CanonicalName, e.Locked, e.Discrete,
		e.ParentID, e.FullPathDisplay, e.FullPathCanonical,
		e.Depth, e.Status, e.StatusContext,
		e.LastEventID, e.UpdatedAt.UTC().Format(time.RFC3339), e.HomeID,
		e.EntityID,
	)
	if err != nil {
		return fmt.Errorf("update entity %s: %w", e.EntityID, err)
	}
	return nil
}

// GetLiveEntity retrieves a single non-removed entity by ID.
func (r Reader) GetLiveEntity(ctx context.Context, entityID string) (*inventory.Entity, error) {
	const query = `
		SELECT entity_id, display_name, canonical_name, locked, discrete,
		       parent_id, full_path_display, full_path_canonical,
		       depth, status, status_context, last_event_id, updated_at, home_id
		FROM entities_current WHERE entity_id = ? AND status != 'removed'`

	row := r.q.QueryRowContext(ctx, query, entityID)
	e, err := scanEntity(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get entity %s: %w", entityID, err)
	}
	return e, nil
}

// GetEntityAnyStatus retrieves a single entity by ID regardless of status, including removed entities.
func (r Reader) GetEntityAnyStatus(ctx context.Context, entityID string) (*inventory.Entity, error) {
	const query = `
		SELECT entity_id, display_name, canonical_name, locked, discrete,
		       parent_id, full_path_display, full_path_canonical,
		       depth, status, status_context, last_event_id, updated_at, home_id
		FROM entities_current WHERE entity_id = ?`

	row := r.q.QueryRowContext(ctx, query, entityID)
	e, err := scanEntity(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get entity %s: %w", entityID, err)
	}
	return e, nil
}

// GetLiveEntitiesByCanonicalName retrieves all live (non-removed) entities with a
// given canonical name, ordered by full_path_canonical ASC, entity_id ASC.
func (r Reader) GetLiveEntitiesByCanonicalName(ctx context.Context, canonical string) ([]*inventory.Entity, error) {
	const query = `
		SELECT entity_id, display_name, canonical_name, locked, discrete,
		       parent_id, full_path_display, full_path_canonical,
		       depth, status, status_context, last_event_id, updated_at, home_id
		FROM entities_current WHERE canonical_name = ? AND status != 'removed'
		ORDER BY full_path_canonical ASC, entity_id ASC`

	rows, err := r.q.QueryContext(ctx, query, canonical)
	if err != nil {
		return nil, fmt.Errorf("query by canonical name %q: %w", canonical, err)
	}
	defer rows.Close()
	return scanEntities(rows)
}

// GetEntitiesByCanonicalNameAnyStatus retrieves all entities with a given canonical name,
// including removed entities, ordered by last_event_id DESC.
func (r Reader) GetEntitiesByCanonicalNameAnyStatus(
	ctx context.Context, canonical string,
) ([]*inventory.Entity, error) {
	const query = `
		SELECT entity_id, display_name, canonical_name, locked, discrete,
		       parent_id, full_path_display, full_path_canonical,
		       depth, status, status_context, last_event_id, updated_at, home_id
		FROM entities_current WHERE canonical_name = ?
		ORDER BY full_path_canonical ASC, last_event_id DESC`

	rows, err := r.q.QueryContext(ctx, query, canonical)
	if err != nil {
		return nil, fmt.Errorf("query by canonical name %q: %w", canonical, err)
	}
	defer rows.Close()
	return scanEntities(rows)
}

// ChildRow is the result of GetLiveChildren: the entity plus whether it has non-removed children of its own.
type ChildRow struct {
	Entity      *inventory.Entity
	HasChildren bool
}

// GetLiveChildren retrieves direct non-removed children of a parent entity, each annotated
// with whether it has non-removed children of its own. Ordered by display_name ASC, entity_id ASC.
func (r Reader) GetLiveChildren(ctx context.Context, parentID string) ([]ChildRow, error) {
	const query = `
		SELECT ec.entity_id, ec.display_name, ec.canonical_name, ec.locked, ec.discrete,
		       ec.parent_id, ec.full_path_display, ec.full_path_canonical,
		       ec.depth, ec.status, ec.status_context, ec.last_event_id, ec.updated_at, ec.home_id,
		       EXISTS (
		           SELECT 1 FROM entities_current c
		           WHERE c.parent_id = ec.entity_id AND c.status != 'removed'
		       ) AS has_children
		FROM entities_current ec
		WHERE ec.parent_id = ? AND ec.status != 'removed'
		ORDER BY ec.display_name ASC, ec.entity_id ASC`

	rows, err := r.q.QueryContext(ctx, query, parentID)
	if err != nil {
		return nil, fmt.Errorf("get children of %s: %w", parentID, err)
	}
	defer rows.Close()
	return scanChildRows(rows)
}

const countLiveChildrenQuery = `SELECT COUNT(*) FROM entities_current WHERE parent_id = ? AND status != 'removed'`

// CountLiveChildren returns the number of non-removed direct children of parentID.
func (r Reader) CountLiveChildren(ctx context.Context, parentID string) (int, error) {
	var n int
	if err := r.q.QueryRowContext(ctx, countLiveChildrenQuery, parentID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count children of %s: %w", parentID, err)
	}
	return n, nil
}

// ListLiveEntities retrieves all live (non-removed) entities ordered by
// full_path_display ASC, entity_id ASC.
func (r Reader) ListLiveEntities(ctx context.Context) ([]*inventory.Entity, error) {
	const query = `
		SELECT entity_id, display_name, canonical_name, locked, discrete,
		       parent_id, full_path_display, full_path_canonical,
		       depth, status, status_context, last_event_id, updated_at, home_id
		FROM entities_current
		WHERE status != 'removed'
		ORDER BY full_path_display ASC, entity_id ASC`

	rows, err := r.q.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list entities: %w", err)
	}
	defer rows.Close()
	return scanEntities(rows)
}

// ListEntitiesAnyStatus returns all entities in the projection, including removed ones.
func (r Reader) ListEntitiesAnyStatus(ctx context.Context) ([]*inventory.Entity, error) {
	const query = `
		SELECT entity_id, display_name, canonical_name, locked, discrete,
		       parent_id, full_path_display, full_path_canonical,
		       depth, status, status_context, last_event_id, updated_at, home_id
		FROM entities_current
		ORDER BY full_path_display ASC, entity_id ASC`

	rows, err := r.q.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list all entities: %w", err)
	}
	defer rows.Close()
	return scanEntities(rows)
}

// ComputeEntityPath computes full_path_display, full_path_canonical, and depth
// for a new entity given its parent.
func (r Reader) ComputeEntityPath(
	ctx context.Context,
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
	scanErr := r.q.QueryRowContext(ctx, query, *parentID).
		Scan(&parentDisplay, &parentCanonical, &parentDepth)
	if scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			return "", "", 0, fmt.Errorf("parent entity %q: %w", *parentID, ErrNotFound)
		}
		return "", "", 0, fmt.Errorf("query parent %s: %w", *parentID, scanErr)
	}

	display, err := entitypath.AppendTo(parentDisplay, displayName)
	if err != nil {
		return "", "", 0, fmt.Errorf("build display path: %w", err)
	}

	canonical, err := entitypath.AppendTo(parentCanonical, canonicalName)
	if err != nil {
		return "", "", 0, fmt.Errorf("build canonical path: %w", err)
	}

	return display.String(), canonical.String(), parentDepth + 1, nil
}

type scanFunc func(dest ...any) error

func scanEntity(scan scanFunc) (*inventory.Entity, error) {
	var e inventory.Entity
	var updatedAtStr string
	if err := scan(
		&e.EntityID, &e.DisplayName, &e.CanonicalName, &e.Locked, &e.Discrete,
		&e.ParentID, &e.FullPathDisplay, &e.FullPathCanonical,
		&e.Depth, &e.Status, &e.StatusContext, &e.LastEventID, &updatedAtStr, &e.HomeID,
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

// GetDescendantsAnyStatus retrieves all descendants of a given entity, including
// removed ones, using path prefix matching. Ordered by depth ASC, display_name
// ASC, entity_id ASC. Path propagation relies on removed descendants being
// included so their stored paths keep following their ancestors.
func (r Reader) GetDescendantsAnyStatus(ctx context.Context, entityID string) ([]*inventory.Entity, error) {
	const pathQuery = `SELECT full_path_canonical FROM entities_current WHERE entity_id = ?`
	var prefix string
	if err := r.q.QueryRowContext(ctx, pathQuery, entityID).Scan(&prefix); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get path for %s: %w", entityID, err)
	}

	const query = `
		SELECT entity_id, display_name, canonical_name, locked, discrete,
		       parent_id, full_path_display, full_path_canonical,
		       depth, status, status_context, last_event_id, updated_at, home_id
		FROM entities_current WHERE full_path_canonical LIKE ? ESCAPE '\'
		ORDER BY depth ASC, display_name ASC, entity_id ASC`

	rows, err := r.q.QueryContext(ctx, query, escapeLIKE(prefix)+":%")
	if err != nil {
		return nil, fmt.Errorf("get descendants of %s: %w", entityID, err)
	}
	defer rows.Close()
	return scanEntities(rows)
}

// TruncateEntities deletes all rows from entities_current.
func (t Tx) TruncateEntities(ctx context.Context) error {
	if _, err := t.q.ExecContext(ctx, `DELETE FROM entities_current`); err != nil {
		return fmt.Errorf("truncate entities_current: %w", err)
	}
	return nil
}

func escapeLIKE(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// CountLiveDisplacedWithHome counts non-removed entities whose home is homeID but
// which currently sit elsewhere (CONTEXT.md "Home": such a home may not be
// removed or made discrete). IS NOT treats NULL parent (root) correctly.
func (r Reader) CountLiveDisplacedWithHome(ctx context.Context, homeID string) (int, error) {
	const q = `
		SELECT COUNT(*) FROM entities_current
		WHERE home_id = ? AND status != 'removed' AND parent_id IS NOT home_id`
	var n int
	if err := r.q.QueryRowContext(ctx, q, homeID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count displaced with home %s: %w", homeID, err)
	}
	return n, nil
}
