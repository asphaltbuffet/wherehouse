package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Item represents an item in the projection.
type Item struct {
	ItemID               string
	DisplayName          string
	CanonicalName        string
	LocationID           string
	InTemporaryUse       bool
	TempOriginLocationID *string
	ProjectID            *string
	LastEventID          int64
	UpdatedAt            string
}

// CreateItem creates a new item projection entry.
func (d *Database) CreateItem(
	ctx context.Context,
	itemID, displayName, locationID string,
	eventID int64,
	timestamp string,
) error {
	canonicalName := CanonicalizeString(displayName)

	const query = `
		INSERT INTO items_current (
			item_id,
			display_name,
			canonical_name,
			location_id,
			in_temporary_use,
			temp_origin_location_id,
			project_id,
			last_event_id,
			updated_at
		) VALUES (?, ?, ?, ?, 0, NULL, NULL, ?, ?)
	`

	_, err := d.db.ExecContext(ctx, query,
		itemID,
		displayName,
		canonicalName,
		locationID,
		eventID,
		timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to create item: %w", err)
	}

	return nil
}

// GetItem retrieves an item by its ID.
func (d *Database) GetItem(ctx context.Context, itemID string) (*Item, error) {
	const query = `
		SELECT
			item_id,
			display_name,
			canonical_name,
			location_id,
			in_temporary_use,
			temp_origin_location_id,
			project_id,
			last_event_id,
			updated_at
		FROM items_current
		WHERE item_id = ?
	`

	var item Item
	err := d.db.QueryRowContext(ctx, query, itemID).Scan(
		&item.ItemID,
		&item.DisplayName,
		&item.CanonicalName,
		&item.LocationID,
		&item.InTemporaryUse,
		&item.TempOriginLocationID,
		&item.ProjectID,
		&item.LastEventID,
		&item.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrItemNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get item: %w", err)
	}

	return &item, nil
}

// GetItemsByLocation retrieves all items in a specific location.
func (d *Database) GetItemsByLocation(ctx context.Context, locationID string) ([]*Item, error) {
	const query = `
		SELECT
			item_id,
			display_name,
			canonical_name,
			location_id,
			in_temporary_use,
			temp_origin_location_id,
			project_id,
			last_event_id,
			updated_at
		FROM items_current
		WHERE location_id = ?
		ORDER BY display_name
	`

	rows, err := d.db.QueryContext(ctx, query, locationID)
	if err != nil {
		return nil, fmt.Errorf("failed to query items by location: %w", err)
	}
	defer rows.Close()

	return scanItems(rows)
}

// GetItemsByProject retrieves all items associated with a project.
func (d *Database) GetItemsByProject(ctx context.Context, projectID string) ([]*Item, error) {
	const query = `
		SELECT
			item_id,
			display_name,
			canonical_name,
			location_id,
			in_temporary_use,
			temp_origin_location_id,
			project_id,
			last_event_id,
			updated_at
		FROM items_current
		WHERE project_id = ?
		ORDER BY display_name
	`

	rows, err := d.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to query items by project: %w", err)
	}
	defer rows.Close()

	return scanItems(rows)
}

// GetItemsByCanonicalName retrieves all items with a specific canonical name.
// Returns a slice because canonical names are not unique across locations.
func (d *Database) GetItemsByCanonicalName(ctx context.Context, canonicalName string) ([]*Item, error) {
	const query = `
		SELECT
			item_id,
			display_name,
			canonical_name,
			location_id,
			in_temporary_use,
			temp_origin_location_id,
			project_id,
			last_event_id,
			updated_at
		FROM items_current
		WHERE canonical_name = ?
		ORDER BY display_name
	`

	rows, err := d.db.QueryContext(ctx, query, canonicalName)
	if err != nil {
		return nil, fmt.Errorf("failed to query items by canonical name: %w", err)
	}
	defer rows.Close()

	return scanItems(rows)
}

// UpdateItem updates an item's fields.
func (d *Database) UpdateItem(
	ctx context.Context,
	itemID string,
	updates map[string]any,
	eventID int64,
	timestamp string,
) error {
	// Build dynamic update query
	var setParts []string
	var args []any

	if displayName, ok := updates["display_name"].(string); ok {
		setParts = append(setParts, "display_name = ?")
		args = append(args, displayName)

		// Update canonical name when display name changes
		canonicalName := CanonicalizeString(displayName)
		setParts = append(setParts, "canonical_name = ?")
		args = append(args, canonicalName)
	}

	if locationID, ok := updates["location_id"].(string); ok {
		setParts = append(setParts, "location_id = ?")
		args = append(args, locationID)
	}

	if inTempUse, ok := updates["in_temporary_use"].(bool); ok {
		setParts = append(setParts, "in_temporary_use = ?")
		args = append(args, inTempUse)
	}

	if tempOriginLocID, ok := updates["temp_origin_location_id"]; ok {
		setParts = append(setParts, "temp_origin_location_id = ?")
		args = append(args, tempOriginLocID)
	}

	if projectID, ok := updates["project_id"]; ok {
		setParts = append(setParts, "project_id = ?")
		args = append(args, projectID)
	}

	// Always update last_event_id and timestamp
	setParts = append(setParts, "last_event_id = ?", "updated_at = ?")
	args = append(args, eventID, timestamp)

	// Add WHERE clause argument
	args = append(args, itemID)

	//nolint:gosec // Safe: building SET clause from validated field names only
	query := fmt.Sprintf(`
		UPDATE items_current
		SET %s
		WHERE item_id = ?
	`, strings.Join(setParts, ", "))

	result, err := d.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update item: %w", err)
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

// DeleteItem removes an item from the projection.
func (d *Database) DeleteItem(ctx context.Context, itemID string) error {
	const query = `DELETE FROM items_current WHERE item_id = ?`

	result, err := d.db.ExecContext(ctx, query, itemID)
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

// GetAllItems retrieves all items from the projection table.
// Used by migration operations that need to enumerate all entity IDs.
func (d *Database) GetAllItems(ctx context.Context) ([]*Item, error) {
	const query = `
		SELECT
			item_id,
			display_name,
			canonical_name,
			location_id,
			in_temporary_use,
			temp_origin_location_id,
			project_id,
			last_event_id,
			updated_at
		FROM items_current
		ORDER BY display_name
	`

	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query all items: %w", err)
	}
	defer rows.Close()

	return scanItems(rows)
}

// scanItems is a helper to scan multiple items from rows.
func scanItems(rows *sql.Rows) ([]*Item, error) {
	var items []*Item

	for rows.Next() {
		var item Item
		err := rows.Scan(
			&item.ItemID,
			&item.DisplayName,
			&item.CanonicalName,
			&item.LocationID,
			&item.InTemporaryUse,
			&item.TempOriginLocationID,
			&item.ProjectID,
			&item.LastEventID,
			&item.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan item: %w", err)
		}

		items = append(items, &item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating items: %w", err)
	}

	return items, nil
}
