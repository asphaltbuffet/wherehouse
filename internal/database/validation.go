package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ValidateFromLocation verifies that an item or location is in the expected location before an event.
// This is critical for detecting projection corruption and concurrent modifications.
// Returns ErrInvalidFromLocation if the current location doesn't match the expected from_location_id.
func (d *Database) ValidateFromLocation(ctx context.Context, itemID, expectedFromLocationID string) error {
	item, err := d.GetItem(ctx, itemID)
	if err != nil {
		return fmt.Errorf("failed to get item for from_location validation: %w", err)
	}

	if item.LocationID != expectedFromLocationID {
		return &InvalidFromLocationError{
			ItemID:           itemID,
			ExpectedLocation: expectedFromLocationID,
			ActualLocation:   item.LocationID,
		}
	}

	return nil
}

// DetectLocationCycle checks if setting a location's parent to newParentID would create a cycle.
// A cycle occurs when a location would become its own ancestor.
// Returns ErrLocationCycle if a cycle would be created.
func (d *Database) DetectLocationCycle(ctx context.Context, locationID string, newParentID *string) error {
	// NULL parent is always safe (root location)
	if newParentID == nil {
		return nil
	}

	// Cannot be your own parent
	if locationID == *newParentID {
		return &LocationCycleError{
			LocationID: locationID,
			ParentID:   *newParentID,
			Cycle:      []string{locationID, *newParentID},
		}
	}

	// Walk up from new parent to root, checking for locationID in the chain
	visited := make(map[string]bool)
	current := *newParentID
	path := []string{locationID, current}

	for current != "" {
		// Detect infinite loop in existing tree structure (malformed data)
		if visited[current] {
			return &LocationCycleError{
				LocationID: locationID,
				ParentID:   *newParentID,
				Cycle:      path,
			}
		}
		visited[current] = true

		// Check if we've reached the location being reparented
		if current == locationID {
			return &LocationCycleError{
				LocationID: locationID,
				ParentID:   *newParentID,
				Cycle:      path,
			}
		}

		// Get parent of current location
		loc, err := d.GetLocation(ctx, current)
		if err != nil {
			return fmt.Errorf("failed to get location in cycle detection: %w", err)
		}

		if loc.ParentID == nil {
			// Reached root, no cycle
			break
		}

		current = *loc.ParentID
		path = append(path, current)
	}

	return nil
}

// ValidateUniqueLocationName checks if a location's canonical name is unique within its parent.
// Location names must be globally unique according to the schema's UNIQUE constraint.
// Returns ErrDuplicateLocation if a location with this canonical name already exists.
func (d *Database) ValidateUniqueLocationName(
	ctx context.Context,
	canonicalName string,
	excludeLocationID *string,
) error {
	query := `
		SELECT location_id
		FROM locations_current
		WHERE canonical_name = ?
	`
	args := []any{canonicalName}

	if excludeLocationID != nil {
		query += " AND location_id != ?"
		args = append(args, *excludeLocationID)
	}

	var existingID string
	err := d.db.QueryRowContext(ctx, query, args...).Scan(&existingID)

	if err == nil {
		// Found existing location with same canonical name
		return &DuplicateLocationError{
			CanonicalName: canonicalName,
			ExistingID:    existingID,
		}
	}

	if err == sql.ErrNoRows {
		// No duplicate found, name is unique
		return nil
	}

	// Database error
	return fmt.Errorf("failed to check unique location name: %w", err)
}

// ValidateUniqueItemName checks if an item's canonical name is unique within its location.
// Item names are unique per location (not globally unique like locations).
// Returns ErrDuplicateItem if an item with this canonical name already exists in the location.
func (d *Database) ValidateUniqueItemName(
	ctx context.Context,
	locationID, canonicalName string,
	excludeItemID *string,
) error {
	query := `
		SELECT item_id
		FROM items_current
		WHERE location_id = ? AND canonical_name = ?
	`
	args := []any{locationID, canonicalName}

	if excludeItemID != nil {
		query += " AND item_id != ?"
		args = append(args, *excludeItemID)
	}

	var existingID string
	err := d.db.QueryRowContext(ctx, query, args...).Scan(&existingID)

	if err == nil {
		// Found existing item with same canonical name in this location
		return &DuplicateItemError{
			LocationID:    locationID,
			CanonicalName: canonicalName,
			ExistingID:    existingID,
		}
	}

	if err == sql.ErrNoRows {
		// No duplicate found, name is unique within location
		return nil
	}

	// Database error
	return fmt.Errorf("failed to check unique item name: %w", err)
}

// ValidateLocationExists checks if a location exists.
// Returns ErrLocationNotFound if location doesn't exist.
func (d *Database) ValidateLocationExists(ctx context.Context, locationID string) error {
	_, err := d.GetLocation(ctx, locationID)
	return err
}

// ValidateItemExists checks if an item exists.
// Returns ErrItemNotFound if item doesn't exist.
func (d *Database) ValidateItemExists(ctx context.Context, itemID string) error {
	_, err := d.GetItem(ctx, itemID)
	return err
}

// ValidateLocationEmpty checks if a location has no children and no items.
// This is required before deleting a location.
// Returns an error if the location has children or items.
func (d *Database) ValidateLocationEmpty(ctx context.Context, locationID string) error {
	// Check for child locations
	const childQuery = `SELECT COUNT(*) FROM locations_current WHERE parent_id = ?`
	var childCount int
	err := d.db.QueryRowContext(ctx, childQuery, locationID).Scan(&childCount)
	if err != nil {
		return fmt.Errorf("failed to check for child locations: %w", err)
	}

	if childCount > 0 {
		return fmt.Errorf("location has %d child locations (must be empty to delete)", childCount)
	}

	// Check for items in location
	const itemQuery = `SELECT COUNT(*) FROM items_current WHERE location_id = ?`
	var itemCount int
	err = d.db.QueryRowContext(ctx, itemQuery, locationID).Scan(&itemCount)
	if err != nil {
		return fmt.Errorf("failed to check for items in location: %w", err)
	}

	if itemCount > 0 {
		return fmt.Errorf("location has %d items (must be empty to delete)", itemCount)
	}

	return nil
}

// ValidateSystemLocation checks if a location is a system location.
// System locations (Missing, Borrowed) cannot be modified or deleted.
// Returns an error if the location is a system location.
func (d *Database) ValidateSystemLocation(ctx context.Context, locationID string) error {
	loc, err := d.GetLocation(ctx, locationID)
	if err != nil {
		return err
	}

	if loc.IsSystem {
		return fmt.Errorf("cannot modify system location %q (%s)", loc.DisplayName, loc.CanonicalName)
	}

	return nil
}

// ValidateNoColonInName checks if a name contains a colon character.
// Colons are reserved for the selector syntax (LOCATION:ITEM).
// Returns an error if the name contains a colon.
func ValidateNoColonInName(name string) error {
	if strings.Contains(name, ":") {
		return fmt.Errorf("name cannot contain colon character (reserved for selector syntax): %q", name)
	}
	return nil
}

// ValidateFromParent verifies that a location's parent matches the expected from_parent_id.
// This is critical for location.reparented events to detect projection corruption.
// Returns an error if the current parent doesn't match the expected from_parent_id.
func (d *Database) ValidateFromParent(ctx context.Context, locationID string, expectedFromParentID *string) error {
	loc, err := d.GetLocation(ctx, locationID)
	if err != nil {
		return fmt.Errorf("failed to get location for from_parent validation: %w", err)
	}

	// Compare parent IDs (handle NULL cases)
	if expectedFromParentID == nil {
		if loc.ParentID != nil {
			return fmt.Errorf("location parent mismatch: expected NULL, got %q", *loc.ParentID)
		}
	} else {
		if loc.ParentID == nil {
			return fmt.Errorf("location parent mismatch: expected %q, got NULL", *expectedFromParentID)
		}
		if *loc.ParentID != *expectedFromParentID {
			return fmt.Errorf("location parent mismatch: expected %q, got %q", *expectedFromParentID, *loc.ParentID)
		}
	}

	return nil
}

// ValidateItemLoaned validates that an item can be loaned.
// Checks: item exists, from_location matches projection, loaned_to is non-empty.
// Re-loaning is allowed (item can be loaned from Loaned location).
// Returns an error if validation fails.
func (d *Database) ValidateItemLoaned(ctx context.Context, itemID, fromLocationID, loanedTo string) error {
	// Check item exists and get current location
	item, err := d.GetItem(ctx, itemID)
	if err != nil {
		return fmt.Errorf("failed to get item for loan validation: %w", err)
	}

	// Validate from_location matches projection (critical integrity check)
	if item.LocationID != fromLocationID {
		return &InvalidFromLocationError{
			ItemID:           itemID,
			ExpectedLocation: fromLocationID,
			ActualLocation:   item.LocationID,
		}
	}

	// Validate loaned_to is not empty (trimmed, no whitespace-only strings)
	trimmedLoanedTo := strings.TrimSpace(loanedTo)
	if trimmedLoanedTo == "" {
		return errors.New("loaned_to cannot be empty or whitespace-only")
	}

	// Note: Re-loaning is explicitly allowed - no check for already being in Loaned location

	return nil
}
