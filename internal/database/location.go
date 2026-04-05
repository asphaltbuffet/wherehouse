package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Location represents a location in the projection.
type Location struct {
	LocationID        string
	DisplayName       string
	CanonicalName     string
	ParentID          *string
	FullPathDisplay   string
	FullPathCanonical string
	Depth             int
	IsSystem          bool
	UpdatedAt         string
}

// CreateLocation creates a new location projection entry.
func (d *Database) CreateLocation(
	ctx context.Context,
	locationID, displayName string,
	parentID *string,
	isSystem bool,
	_ int64, // eventID unused - locations don't track last_event_id
	timestamp string,
) error {
	canonicalName := CanonicalizeString(displayName)

	// Compute path fields
	fullPathDisplay, fullPathCanonical, depth, err := d.computeLocationPath(ctx, displayName, canonicalName, parentID)
	if err != nil {
		return fmt.Errorf("failed to compute location path: %w", err)
	}

	const query = `
		INSERT INTO locations_current (
			location_id,
			display_name,
			canonical_name,
			parent_id,
			full_path_display,
			full_path_canonical,
			depth,
			is_system,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = d.db.ExecContext(ctx, query,
		locationID,
		displayName,
		canonicalName,
		parentID,
		fullPathDisplay,
		fullPathCanonical,
		depth,
		isSystem,
		timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to create location: %w", err)
	}

	return nil
}

// GetLocation retrieves a location by its ID.
func (d *Database) GetLocation(ctx context.Context, locationID string) (*Location, error) {
	const query = `
		SELECT
			location_id,
			display_name,
			canonical_name,
			parent_id,
			full_path_display,
			full_path_canonical,
			depth,
			is_system,
			updated_at
		FROM locations_current
		WHERE location_id = ?
	`

	var loc Location
	err := d.db.QueryRowContext(ctx, query, locationID).Scan(
		&loc.LocationID,
		&loc.DisplayName,
		&loc.CanonicalName,
		&loc.ParentID,
		&loc.FullPathDisplay,
		&loc.FullPathCanonical,
		&loc.Depth,
		&loc.IsSystem,
		&loc.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrLocationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get location: %w", err)
	}

	return &loc, nil
}

// GetLocationByCanonicalName retrieves a location by its canonical name.
// Returns ErrLocationNotFound if no location matches.
// Returns AmbiguousLocationError if multiple locations match (violates global uniqueness).
func (d *Database) GetLocationByCanonicalName(ctx context.Context, canonicalName string) (*Location, error) {
	const query = `
		SELECT
			location_id,
			display_name,
			canonical_name,
			parent_id,
			full_path_display,
			full_path_canonical,
			depth,
			is_system,
			updated_at
		FROM locations_current
		WHERE canonical_name = ?
	`

	rows, err := d.db.QueryContext(ctx, query, canonicalName)
	if err != nil {
		return nil, fmt.Errorf("failed to query location by canonical name: %w", err)
	}
	defer rows.Close()

	var locations []*Location
	for rows.Next() {
		var loc Location
		if scanErr := rows.Scan(
			&loc.LocationID,
			&loc.DisplayName,
			&loc.CanonicalName,
			&loc.ParentID,
			&loc.FullPathDisplay,
			&loc.FullPathCanonical,
			&loc.Depth,
			&loc.IsSystem,
			&loc.UpdatedAt,
		); scanErr != nil {
			return nil, fmt.Errorf("failed to scan location: %w", scanErr)
		}
		locations = append(locations, &loc)
	}

	if rowErr := rows.Err(); rowErr != nil {
		return nil, fmt.Errorf("error iterating location rows: %w", rowErr)
	}

	// Handle match count
	switch len(locations) {
	case 0:
		return nil, ErrLocationNotFound
	case 1:
		return locations[0], nil
	default:
		// Multiple matches - violates global uniqueness constraint
		matchingIDs := make([]string, len(locations))
		for i, loc := range locations {
			matchingIDs[i] = loc.LocationID
		}
		return nil, &AmbiguousLocationError{
			CanonicalName: canonicalName,
			MatchingIDs:   matchingIDs,
		}
	}
}

// UpdateLocation updates a location's basic fields (not path-related).
func (d *Database) UpdateLocation(
	ctx context.Context,
	locationID string,
	updates map[string]any,
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

	if parentID, ok := updates["parent_id"]; ok {
		setParts = append(setParts, "parent_id = ?")
		args = append(args, parentID)
	}

	if fullPathDisplay, ok := updates["full_path_display"].(string); ok {
		setParts = append(setParts, "full_path_display = ?")
		args = append(args, fullPathDisplay)
	}

	if fullPathCanonical, ok := updates["full_path_canonical"].(string); ok {
		setParts = append(setParts, "full_path_canonical = ?")
		args = append(args, fullPathCanonical)
	}

	if depth, ok := updates["depth"].(int); ok {
		setParts = append(setParts, "depth = ?")
		args = append(args, depth)
	}

	// Always update timestamp
	setParts = append(setParts, "updated_at = ?")
	args = append(args, timestamp)

	// Add WHERE clause argument
	args = append(args, locationID)

	//nolint:gosec // Safe: building SET clause from validated field names only
	query := fmt.Sprintf(`
		UPDATE locations_current
		SET %s
		WHERE location_id = ?
	`, strings.Join(setParts, ", "))

	result, err := d.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update location: %w", err)
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

// DeleteLocation removes a location from the projection.
func (d *Database) DeleteLocation(ctx context.Context, locationID string) error {
	const query = `DELETE FROM locations_current WHERE location_id = ?`

	result, err := d.db.ExecContext(ctx, query, locationID)
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

// GetRootLocations retrieves all locations with no parent (top-level),
// ordered by display_name. Includes system locations (Missing, Borrowed).
func (d *Database) GetRootLocations(ctx context.Context) ([]*Location, error) {
	const query = `
		SELECT
			location_id,
			display_name,
			canonical_name,
			parent_id,
			full_path_display,
			full_path_canonical,
			depth,
			is_system,
			updated_at
		FROM locations_current
		WHERE parent_id IS NULL
		ORDER BY display_name
	`

	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query root locations: %w", err)
	}
	defer rows.Close()

	return scanLocations(rows)
}

// GetLocationChildren retrieves all child locations of a parent.
func (d *Database) GetLocationChildren(ctx context.Context, parentID string) ([]*Location, error) {
	const query = `
		SELECT
			location_id,
			display_name,
			canonical_name,
			parent_id,
			full_path_display,
			full_path_canonical,
			depth,
			is_system,
			updated_at
		FROM locations_current
		WHERE parent_id = ?
		ORDER BY display_name
	`

	rows, err := d.db.QueryContext(ctx, query, parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query location children: %w", err)
	}
	defer rows.Close()

	return scanLocations(rows)
}

// computeLocationPath computes the full path fields for a location.
func (d *Database) computeLocationPath(
	ctx context.Context,
	displayName, canonicalName string,
	parentID *string,
) (string, string, int, error) {
	if parentID == nil {
		// Root location
		return displayName, canonicalName, 0, nil
	}

	// Get parent location
	parent, err := d.GetLocation(ctx, *parentID)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to get parent location: %w", err)
	}

	// Build paths
	fullPathDisplay := parent.FullPathDisplay + " >> " + displayName
	fullPathCanonical := parent.FullPathCanonical + ":" + canonicalName
	depth := parent.Depth + 1

	return fullPathDisplay, fullPathCanonical, depth, nil
}

// computeLocationPathTx computes the full path fields for a location within a transaction.
// This version uses the transaction connection to avoid deadlocks.
func (d *Database) computeLocationPathTx(
	ctx context.Context,
	tx *sql.Tx,
	displayName, canonicalName string,
	parentID *string,
) (string, string, int, error) {
	if parentID == nil {
		// Root location
		return displayName, canonicalName, 0, nil
	}

	// Get parent location using transaction
	var parent Location
	err := tx.QueryRowContext(ctx,
		`SELECT location_id, display_name, canonical_name, parent_id,
		        full_path_display, full_path_canonical, depth, is_system, updated_at
		 FROM locations_current WHERE location_id = ?`,
		*parentID,
	).Scan(
		&parent.LocationID, &parent.DisplayName, &parent.CanonicalName, &parent.ParentID,
		&parent.FullPathDisplay, &parent.FullPathCanonical, &parent.Depth, &parent.IsSystem,
		&parent.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return "", "", 0, ErrLocationNotFound
	}
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to get parent location: %w", err)
	}

	// Build paths
	fullPathDisplay := parent.FullPathDisplay + " >> " + displayName
	fullPathCanonical := parent.FullPathCanonical + ":" + canonicalName
	depth := parent.Depth + 1

	return fullPathDisplay, fullPathCanonical, depth, nil
}

// scanLocations is a helper to scan multiple locations from rows.
func scanLocations(rows *sql.Rows) ([]*Location, error) {
	var locations []*Location

	for rows.Next() {
		var loc Location
		err := rows.Scan(
			&loc.LocationID,
			&loc.DisplayName,
			&loc.CanonicalName,
			&loc.ParentID,
			&loc.FullPathDisplay,
			&loc.FullPathCanonical,
			&loc.Depth,
			&loc.IsSystem,
			&loc.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan location: %w", err)
		}

		locations = append(locations, &loc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating locations: %w", err)
	}

	return locations, nil
}
