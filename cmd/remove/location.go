package remove

import (
	"context"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// LocationResult represents the outcome of removing a location.
type LocationResult struct {
	LocationID  string `json:"location_id"`
	DisplayName string `json:"display_name"`
	EventID     int64  `json:"event_id"`
}

// removeLocation removes an empty, non-system location.
// System locations cannot be removed. Non-system locations must be empty (no items, no children).
func removeLocation(
	ctx context.Context,
	db removeDB,
	locationID, actorUserID, note string,
) (*LocationResult, error) {
	// Get the location
	loc, err := db.GetLocation(ctx, locationID)
	if err != nil {
		return nil, fmt.Errorf("location not found: %w", err)
	}

	// Reject system locations
	if loc.IsSystem {
		return nil, fmt.Errorf("cannot remove system location %q", loc.DisplayName)
	}

	// Check for items
	items, err := db.GetItemsByLocation(ctx, locationID)
	if err != nil {
		return nil, fmt.Errorf("failed to check items in location: %w", err)
	}
	if len(items) > 0 {
		return nil, fmt.Errorf("location %q is not empty: contains %d item(s)", loc.DisplayName, len(items))
	}

	// Check for child locations
	children, err := db.GetLocationChildren(ctx, locationID)
	if err != nil {
		return nil, fmt.Errorf("failed to check child locations: %w", err)
	}
	if len(children) > 0 {
		return nil, fmt.Errorf("location %q is not empty: contains %d sub-location(s)", loc.DisplayName, len(children))
	}

	payload := map[string]any{
		"location_id": locationID,
	}

	eventID, err := db.AppendEvent(ctx, database.LocationRemovedEvent, actorUserID, payload, note)
	if err != nil {
		return nil, fmt.Errorf("failed to create location.removed event: %w", err)
	}

	return &LocationResult{
		LocationID:  locationID,
		DisplayName: loc.DisplayName,
		EventID:     eventID,
	}, nil
}
