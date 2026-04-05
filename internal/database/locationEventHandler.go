package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/goccy/go-json"
)

func (d *Database) handleLocationCreated(ctx context.Context, tx *sql.Tx, event *Event) error {
	var payload struct {
		LocationID  string  `json:"location_id"`
		DisplayName string  `json:"display_name"`
		ParentID    *string `json:"parent_id"`
		IsSystem    *bool   `json:"is_system"`
	}

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	isSystem := false
	if payload.IsSystem != nil {
		isSystem = *payload.IsSystem
	}

	canonicalName := CanonicalizeString(payload.DisplayName)
	fullPathDisplay, fullPathCanonical, depth, err := d.computeLocationPathTx(
		ctx,
		tx,
		payload.DisplayName,
		canonicalName,
		payload.ParentID,
	)
	if err != nil {
		return fmt.Errorf("failed to compute location path: %w", err)
	}

	const query = `
		INSERT INTO locations_current (
			location_id, display_name, canonical_name, parent_id,
			full_path_display, full_path_canonical, depth, is_system, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = tx.ExecContext(ctx, query,
		payload.LocationID, payload.DisplayName, canonicalName, payload.ParentID,
		fullPathDisplay, fullPathCanonical, depth, isSystem, event.TimestampUTC,
	)
	if err != nil {
		return fmt.Errorf("failed to insert location: %w", err)
	}

	return nil
}

func (d *Database) handleLocationRenamed(ctx context.Context, tx *sql.Tx, event *Event) error {
	var payload struct {
		LocationID string `json:"location_id"`
		NewName    string `json:"new_name"`
	}

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	canonicalName := CanonicalizeString(payload.NewName)

	// Get location to retrieve parent_id
	var parentID *string
	err := tx.QueryRowContext(ctx, "SELECT parent_id FROM locations_current WHERE location_id = ?", payload.LocationID).
		Scan(&parentID)
	if err != nil {
		return fmt.Errorf("failed to get location for rename: %w", err)
	}

	// Recompute paths
	fullPathDisplay, fullPathCanonical, depth, err := d.computeLocationPathTx(
		ctx,
		tx,
		payload.NewName,
		canonicalName,
		parentID,
	)
	if err != nil {
		return fmt.Errorf("failed to compute location path: %w", err)
	}

	const query = `
		UPDATE locations_current
		SET display_name = ?, canonical_name = ?, full_path_display = ?, full_path_canonical = ?, depth = ?, updated_at = ?
		WHERE location_id = ?
	`

	_, err = tx.ExecContext(
		ctx,
		query,
		payload.NewName,
		canonicalName,
		fullPathDisplay,
		fullPathCanonical,
		depth,
		event.TimestampUTC,
		payload.LocationID,
	)
	if err != nil {
		return fmt.Errorf("failed to rename location: %w", err)
	}

	// Update all descendant paths recursively
	return d.updateDescendantPaths(ctx, tx, payload.LocationID, event.TimestampUTC)
}

func (d *Database) handleLocationReparented(ctx context.Context, tx *sql.Tx, event *Event) error {
	var payload struct {
		LocationID   string  `json:"location_id"`
		FromParentID *string `json:"from_parent_id"`
		ToParentID   *string `json:"to_parent_id"`
	}

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Get location's display and canonical names
	var displayName, canonicalName string
	err := tx.QueryRowContext(ctx,
		"SELECT display_name, canonical_name FROM locations_current WHERE location_id = ?",
		payload.LocationID,
	).Scan(&displayName, &canonicalName)
	if err != nil {
		return fmt.Errorf("failed to get location for reparent: %w", err)
	}

	// Recompute paths with new parent
	fullPathDisplay, fullPathCanonical, depth, err := d.computeLocationPathTx(
		ctx,
		tx,
		displayName,
		canonicalName,
		payload.ToParentID,
	)
	if err != nil {
		return fmt.Errorf("failed to compute location path: %w", err)
	}

	const query = `
		UPDATE locations_current
		SET parent_id = ?, full_path_display = ?, full_path_canonical = ?, depth = ?, updated_at = ?
		WHERE location_id = ?
	`

	_, err = tx.ExecContext(ctx, query,
		payload.ToParentID, fullPathDisplay, fullPathCanonical, depth, event.TimestampUTC, payload.LocationID,
	)
	if err != nil {
		return fmt.Errorf("failed to reparent location: %w", err)
	}

	// Update all descendant paths recursively
	return d.updateDescendantPaths(ctx, tx, payload.LocationID, event.TimestampUTC)
}

func (d *Database) handleLocationDeleted(ctx context.Context, tx *sql.Tx, event *Event) error {
	var payload struct {
		LocationID string `json:"location_id"`
	}

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	const query = `DELETE FROM locations_current WHERE location_id = ?`

	result, err := tx.ExecContext(ctx, query, payload.LocationID)
	if err != nil {
		return fmt.Errorf("failed to delete location: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrLocationNotFound
	}

	return nil
}

// updateDescendantPaths recursively updates full paths for all descendant locations.
func (d *Database) updateDescendantPaths(
	ctx context.Context,
	tx *sql.Tx,
	parentLocationID string,
	timestamp string,
) error {
	// Get all direct children
	const childQuery = `SELECT location_id, display_name, canonical_name FROM locations_current WHERE parent_id = ?`
	rows, err := tx.QueryContext(ctx, childQuery, parentLocationID)
	if err != nil {
		return fmt.Errorf("failed to query children: %w", err)
	}
	defer rows.Close()

	type child struct {
		locationID    string
		displayName   string
		canonicalName string
	}

	var children []child
	for rows.Next() {
		var c child
		if err = rows.Scan(&c.locationID, &c.displayName, &c.canonicalName); err != nil {
			return fmt.Errorf("failed to scan child: %w", err)
		}
		children = append(children, c)
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate children: %w", err)
	}

	// Update each child
	for _, c := range children {
		fullPathDisplay, fullPathCanonical, depth, childErr := d.computeLocationPathTx(
			ctx,
			tx,
			c.displayName,
			c.canonicalName,
			&parentLocationID,
		)
		if childErr != nil {
			return fmt.Errorf("failed to compute child path: %w", childErr)
		}

		const updateQuery = `
			UPDATE locations_current
			SET full_path_display = ?, full_path_canonical = ?, depth = ?, updated_at = ?
			WHERE location_id = ?
		`

		_, childErr = tx.ExecContext(ctx, updateQuery,
			fullPathDisplay, fullPathCanonical, depth, timestamp, c.locationID,
		)
		if childErr != nil {
			return fmt.Errorf("failed to update child location: %w", childErr)
		}

		// Recursively update this child's descendants
		if childErr = d.updateDescendantPaths(ctx, tx, c.locationID, timestamp); childErr != nil {
			return childErr
		}
	}

	return nil
}
