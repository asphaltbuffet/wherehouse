package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/goccy/go-json"
)

func (d *Database) handleItemCreated(ctx context.Context, tx *sql.Tx, event *Event) error {
	var payload struct {
		ItemID      string `json:"item_id"`
		DisplayName string `json:"display_name"`
		LocationID  string `json:"location_id"`
	}

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	canonicalName := CanonicalizeString(payload.DisplayName)

	const query = `
		INSERT INTO items_current (
			item_id, display_name, canonical_name, location_id,
			in_temporary_use, temp_origin_location_id, project_id,
			last_event_id, updated_at
		) VALUES (?, ?, ?, ?, 0, NULL, NULL, ?, ?)
	`

	_, err := tx.ExecContext(ctx, query,
		payload.ItemID, payload.DisplayName, canonicalName, payload.LocationID,
		event.EventID, event.TimestampUTC,
	)
	if err != nil {
		return fmt.Errorf("failed to insert item: %w", err)
	}

	return nil
}

func (d *Database) handleItemMoved(ctx context.Context, tx *sql.Tx, event *Event) error {
	var payload struct {
		ItemID         string  `json:"item_id"`
		FromLocationID string  `json:"from_location_id"`
		ToLocationID   string  `json:"to_location_id"`
		MoveType       string  `json:"move_type"`
		ProjectAction  *string `json:"project_action"`
		ProjectID      *string `json:"project_id"`
	}

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Get current item state
	var inTempUse bool
	var tempOriginLocID *string
	var currentProjectID *string
	err := tx.QueryRowContext(ctx,
		"SELECT in_temporary_use, temp_origin_location_id, project_id FROM items_current WHERE item_id = ?",
		payload.ItemID,
	).Scan(&inTempUse, &tempOriginLocID, &currentProjectID)
	if err != nil {
		return fmt.Errorf("failed to get item for move: %w", err)
	}

	// Apply move type logic
	switch payload.MoveType {
	case "temporary_use":
		if !inTempUse {
			// First temporary move - set origin
			inTempUse = true
			tempOriginLocID = &payload.FromLocationID
		}
		// Else preserve existing temp_origin_location_id
	case "rehome":
		// Clear temporary use state
		inTempUse = false
		tempOriginLocID = nil
	}

	// Apply project action
	projectAction := "clear"
	if payload.ProjectAction != nil {
		projectAction = *payload.ProjectAction
	}

	switch projectAction {
	case "set":
		currentProjectID = payload.ProjectID
	case "keep":
		// Keep current project_id unchanged
	case "clear":
		currentProjectID = nil
	}

	const query = `
		UPDATE items_current
		SET location_id = ?, in_temporary_use = ?, temp_origin_location_id = ?,
		    project_id = ?, last_event_id = ?, updated_at = ?
		WHERE item_id = ?
	`

	_, err = tx.ExecContext(ctx, query,
		payload.ToLocationID, inTempUse, tempOriginLocID,
		currentProjectID, event.EventID, event.TimestampUTC, payload.ItemID,
	)
	if err != nil {
		return fmt.Errorf("failed to move item: %w", err)
	}

	return nil
}

func (d *Database) handleItemMissing(ctx context.Context, tx *sql.Tx, event *Event) error {
	var payload struct {
		ItemID             string `json:"item_id"`
		PreviousLocationID string `json:"previous_location_id"`
	}

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Get system "Missing" location
	var missingLocID string
	err := tx.QueryRowContext(ctx,
		"SELECT location_id FROM locations_current WHERE canonical_name = 'missing' AND is_system = 1",
	).Scan(&missingLocID)
	if err != nil {
		return fmt.Errorf("failed to get Missing location: %w", err)
	}

	const query = `
		UPDATE items_current
		SET location_id = ?, last_event_id = ?, updated_at = ?
		WHERE item_id = ?
	`

	_, err = tx.ExecContext(ctx, query,
		missingLocID, event.EventID, event.TimestampUTC, payload.ItemID,
	)
	if err != nil {
		return fmt.Errorf("failed to mark item missing: %w", err)
	}

	return nil
}

func (d *Database) handleItemBorrowed(ctx context.Context, tx *sql.Tx, event *Event) error {
	var payload struct {
		ItemID         string `json:"item_id"`
		FromLocationID string `json:"from_location_id"`
		BorrowedBy     string `json:"borrowed_by"`
	}

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Get system "Borrowed" location
	var borrowedLocID string
	err := tx.QueryRowContext(ctx,
		"SELECT location_id FROM locations_current WHERE canonical_name = 'borrowed' AND is_system = 1",
	).Scan(&borrowedLocID)
	if err != nil {
		return fmt.Errorf("failed to get Borrowed location: %w", err)
	}

	const query = `
		UPDATE items_current
		SET location_id = ?, last_event_id = ?, updated_at = ?
		WHERE item_id = ?
	`

	_, err = tx.ExecContext(ctx, query,
		borrowedLocID, event.EventID, event.TimestampUTC, payload.ItemID,
	)
	if err != nil {
		return fmt.Errorf("failed to mark item borrowed: %w", err)
	}

	return nil
}

func (d *Database) handleItemFound(ctx context.Context, tx *sql.Tx, event *Event) error {
	var payload struct {
		ItemID          string `json:"item_id"`
		FoundLocationID string `json:"found_location_id"`
		HomeLocationID  string `json:"home_location_id"`
	}

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	const query = `
		UPDATE items_current
		SET location_id = ?, in_temporary_use = 1, temp_origin_location_id = ?,
		    last_event_id = ?, updated_at = ?
		WHERE item_id = ?
	`

	_, err := tx.ExecContext(ctx, query,
		payload.FoundLocationID, payload.HomeLocationID,
		event.EventID, event.TimestampUTC, payload.ItemID,
	)
	if err != nil {
		return fmt.Errorf("failed to mark item found: %w", err)
	}

	return nil
}

func (d *Database) handleItemLoaned(ctx context.Context, tx *sql.Tx, event *Event) error {
	var payload struct {
		ItemID         string `json:"item_id"`
		FromLocationID string `json:"from_location_id"`
		LoanedTo       string `json:"loaned_to"`
	}

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Get system "Loaned" location
	var loanedLocID string
	err := tx.QueryRowContext(ctx,
		"SELECT location_id FROM locations_current WHERE canonical_name = 'loaned' AND is_system = 1",
	).Scan(&loanedLocID)
	if err != nil {
		return fmt.Errorf("failed to get Loaned location: %w", err)
	}

	// Update projection: move item to Loaned location
	// Preserve temporary use state and project association (like item.missing pattern)
	const query = `
		UPDATE items_current
		SET location_id = ?, last_event_id = ?, updated_at = ?
		WHERE item_id = ?
	`

	_, err = tx.ExecContext(ctx, query,
		loanedLocID, event.EventID, event.TimestampUTC, payload.ItemID,
	)
	if err != nil {
		return fmt.Errorf("failed to mark item loaned: %w", err)
	}

	return nil
}

func (d *Database) handleItemRemoved(ctx context.Context, tx *sql.Tx, event *Event) error {
	var payload struct {
		ItemID             string `json:"item_id"`
		PreviousLocationID string `json:"previous_location_id"`
	}

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Get system "Removed" location
	var removedLocID string
	err := tx.QueryRowContext(ctx,
		"SELECT location_id FROM locations_current WHERE canonical_name = 'removed' AND is_system = 1",
	).Scan(&removedLocID)
	if err != nil {
		return fmt.Errorf("failed to get Removed location: %w", err)
	}

	const query = `
		UPDATE items_current
		SET location_id = ?, last_event_id = ?, updated_at = ?
		WHERE item_id = ?
	`

	_, err = tx.ExecContext(ctx, query,
		removedLocID, event.EventID, event.TimestampUTC, payload.ItemID,
	)
	if err != nil {
		return fmt.Errorf("failed to mark item removed: %w", err)
	}

	return nil
}

func (d *Database) handleItemDeleted(ctx context.Context, tx *sql.Tx, event *Event) error {
	var payload struct {
		ItemID string `json:"item_id"`
	}

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	const query = `DELETE FROM items_current WHERE item_id = ?`

	result, err := tx.ExecContext(ctx, query, payload.ItemID)
	if err != nil {
		return fmt.Errorf("failed to delete item: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrItemNotFound
	}

	return nil
}
