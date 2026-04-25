package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Entity represents a single row in entities_current.
type Entity struct {
	EntityID          string
	DisplayName       string
	CanonicalName     string
	EntityType        EntityType
	ParentID          *string
	FullPathDisplay   string
	FullPathCanonical string
	Depth             int
	Status            EntityStatus
	StatusContext     *string
	LastEventID       int64
	UpdatedAt         time.Time
}

// GetEntity returns an entity by its ID. Returns ErrEntityNotFound if not found.
func (d *Database) GetEntity(ctx context.Context, entityID string) (*Entity, error) {
	const query = `
		SELECT
			entity_id,
			display_name,
			canonical_name,
			entity_type,
			parent_id,
			full_path_display,
			full_path_canonical,
			depth,
			status,
			status_context,
			last_event_id,
			updated_at
		FROM entities_current
		WHERE entity_id = ?
	`

	row := d.db.QueryRowContext(ctx, query, entityID)

	entity, err := scanEntity(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get entity %s: %w", entityID, ErrEntityNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get entity %s: %w", entityID, err)
	}

	return entity, nil
}

// GetEntitiesByCanonicalName returns all entities matching the given canonical name,
// ordered by full_path_canonical ASC, entity_id ASC. Returns empty slice (not error) when none match.
func (d *Database) GetEntitiesByCanonicalName(ctx context.Context, canonicalName string) ([]*Entity, error) {
	const query = `
		SELECT
			entity_id,
			display_name,
			canonical_name,
			entity_type,
			parent_id,
			full_path_display,
			full_path_canonical,
			depth,
			status,
			status_context,
			last_event_id,
			updated_at
		FROM entities_current
		WHERE canonical_name = ?
		ORDER BY full_path_canonical ASC, entity_id ASC
	`

	rows, err := d.db.QueryContext(ctx, query, canonicalName)
	if err != nil {
		return nil, fmt.Errorf("query entities by canonical name %q: %w", canonicalName, err)
	}
	defer rows.Close()

	return scanEntities(rows)
}

// GetChildren returns all direct children of the given parent entity,
// ordered by display_name ASC, entity_id ASC.
func (d *Database) GetChildren(ctx context.Context, parentID string) ([]*Entity, error) {
	const query = `
		SELECT
			entity_id,
			display_name,
			canonical_name,
			entity_type,
			parent_id,
			full_path_display,
			full_path_canonical,
			depth,
			status,
			status_context,
			last_event_id,
			updated_at
		FROM entities_current
		WHERE parent_id = ?
		ORDER BY display_name ASC, entity_id ASC
	`

	rows, err := d.db.QueryContext(ctx, query, parentID)
	if err != nil {
		return nil, fmt.Errorf("query children of entity %s: %w", parentID, err)
	}
	defer rows.Close()

	return scanEntities(rows)
}

// GetDescendants returns all descendants (any depth) of the given entity using a recursive CTE.
// Ordered by depth ASC, full_path_canonical ASC, entity_id ASC.
func (d *Database) GetDescendants(ctx context.Context, entityID string) ([]*Entity, error) {
	const query = `
		WITH RECURSIVE descendants AS (
			SELECT
				entity_id,
				display_name,
				canonical_name,
				entity_type,
				parent_id,
				full_path_display,
				full_path_canonical,
				depth,
				status,
				status_context,
				last_event_id,
				updated_at
			FROM entities_current
			WHERE parent_id = ?
			UNION ALL
			SELECT
				e.entity_id,
				e.display_name,
				e.canonical_name,
				e.entity_type,
				e.parent_id,
				e.full_path_display,
				e.full_path_canonical,
				e.depth,
				e.status,
				e.status_context,
				e.last_event_id,
				e.updated_at
			FROM entities_current e
			INNER JOIN descendants d ON e.parent_id = d.entity_id
		)
		SELECT
			entity_id,
			display_name,
			canonical_name,
			entity_type,
			parent_id,
			full_path_display,
			full_path_canonical,
			depth,
			status,
			status_context,
			last_event_id,
			updated_at
		FROM descendants
		ORDER BY depth ASC, full_path_canonical ASC, entity_id ASC
	`

	rows, err := d.db.QueryContext(ctx, query, entityID)
	if err != nil {
		return nil, fmt.Errorf("query descendants of entity %s: %w", entityID, err)
	}
	defer rows.Close()

	return scanEntities(rows)
}

// ListEntities returns all entities with optional filters.
// Pass empty string to skip a filter.
// underID: restrict to the subtree rooted at this entity (inclusive).
// entityType: filter by entity_type string value (e.g. "place", "container", "leaf").
// status: filter by status string value (e.g. "ok", "missing").
func (d *Database) ListEntities(
	ctx context.Context,
	underID string,
	entityType string,
	status string,
) ([]*Entity, error) {
	query := `
		SELECT
			entity_id,
			display_name,
			canonical_name,
			entity_type,
			parent_id,
			full_path_display,
			full_path_canonical,
			depth,
			status,
			status_context,
			last_event_id,
			updated_at
		FROM entities_current
		WHERE 1=1
	`
	var args []any

	if underID != "" {
		query += `
		AND (entity_id = ? OR full_path_canonical LIKE (SELECT full_path_canonical || '::%' FROM entities_current WHERE entity_id = ?))`
		args = append(args, underID, underID)
	}

	if entityType != "" {
		query += "\n\t\tAND entity_type = ?"
		args = append(args, entityType)
	}

	if status != "" {
		query += "\n\t\tAND status = ?"
		args = append(args, status)
	}

	query += "\n\t\tORDER BY full_path_canonical ASC, entity_id ASC"

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list entities: %w", err)
	}
	defer rows.Close()

	return scanEntities(rows)
}

// ComputeEntityPathTx computes full_path_display, full_path_canonical, and depth for a new or
// moved entity within an existing transaction. If parentID is nil, returns the name values
// directly at depth 0.
func (d *Database) ComputeEntityPathTx(
	ctx context.Context,
	tx *sql.Tx,
	displayName, canonicalName string,
	parentID *string,
) (string, string, int, error) {
	if parentID == nil {
		return displayName, canonicalName, 0, nil
	}

	const query = `
		SELECT full_path_display, full_path_canonical, depth
		FROM entities_current
		WHERE entity_id = ?
	`

	var parentPathDisplay, parentPathCanonical string
	var parentDepth int

	err := tx.QueryRowContext(ctx, query, *parentID).Scan(
		&parentPathDisplay,
		&parentPathCanonical,
		&parentDepth,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", 0, fmt.Errorf(
			"compute entity path: parent entity %s not found: %w",
			*parentID,
			ErrEntityNotFound,
		)
	}
	if err != nil {
		return "", "", 0, fmt.Errorf("compute entity path: query parent %s: %w", *parentID, err)
	}

	fullPathDisplay := parentPathDisplay + "::" + displayName
	fullPathCanonical := parentPathCanonical + "::" + canonicalName
	depth := parentDepth + 1

	return fullPathDisplay, fullPathCanonical, depth, nil
}

// scanEntity scans a single entity from a [*sql.Row].
func scanEntity(row *sql.Row) (*Entity, error) {
	var e Entity
	var entityTypeStr string
	var statusStr string
	var parentID sql.NullString
	var statusContext sql.NullString
	var updatedAtStr string

	err := row.Scan(
		&e.EntityID,
		&e.DisplayName,
		&e.CanonicalName,
		&entityTypeStr,
		&parentID,
		&e.FullPathDisplay,
		&e.FullPathCanonical,
		&e.Depth,
		&statusStr,
		&statusContext,
		&e.LastEventID,
		&updatedAtStr,
	)
	if err != nil {
		return nil, err
	}

	e.EntityType, err = ParseEntityType(entityTypeStr)
	if err != nil {
		return nil, fmt.Errorf("scan entity: %w", err)
	}

	e.Status, err = ParseEntityStatus(statusStr)
	if err != nil {
		return nil, fmt.Errorf("scan entity: %w", err)
	}

	if parentID.Valid {
		e.ParentID = &parentID.String
	}

	if statusContext.Valid {
		e.StatusContext = &statusContext.String
	}

	e.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr)
	if err != nil {
		return nil, fmt.Errorf("scan entity: parse updated_at %q: %w", updatedAtStr, err)
	}

	return &e, nil
}

// scanEntities scans multiple entities from [*sql.Rows].
func scanEntities(rows *sql.Rows) ([]*Entity, error) {
	var entities []*Entity

	for rows.Next() {
		var e Entity
		var entityTypeStr string
		var statusStr string
		var parentID sql.NullString
		var statusContext sql.NullString
		var updatedAtStr string

		err := rows.Scan(
			&e.EntityID,
			&e.DisplayName,
			&e.CanonicalName,
			&entityTypeStr,
			&parentID,
			&e.FullPathDisplay,
			&e.FullPathCanonical,
			&e.Depth,
			&statusStr,
			&statusContext,
			&e.LastEventID,
			&updatedAtStr,
		)
		if err != nil {
			return nil, fmt.Errorf("scan entity row: %w", err)
		}

		e.EntityType, err = ParseEntityType(entityTypeStr)
		if err != nil {
			return nil, fmt.Errorf("scan entity row: %w", err)
		}

		e.Status, err = ParseEntityStatus(statusStr)
		if err != nil {
			return nil, fmt.Errorf("scan entity row: %w", err)
		}

		if parentID.Valid {
			e.ParentID = &parentID.String
		}

		if statusContext.Valid {
			e.StatusContext = &statusContext.String
		}

		e.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr)
		if err != nil {
			return nil, fmt.Errorf("scan entity row: parse updated_at %q: %w", updatedAtStr, err)
		}

		entities = append(entities, &e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate entity rows: %w", err)
	}

	return entities, nil
}
