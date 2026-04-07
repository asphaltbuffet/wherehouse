package remove

import (
	"context"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// ItemResult represents the outcome of removing an item.
type ItemResult struct {
	ItemID           string `json:"item_id"`
	DisplayName      string `json:"display_name"`
	PreviousLocation string `json:"previous_location"`
	EventID          int64  `json:"event_id"`
}

// removeItem moves an item to the Removed system location.
// The itemIDOrSelector may be an item ID, LOCATION:ITEM selector, or canonical name.
// Items already in Removed are rejected.
func removeItem(
	ctx context.Context,
	db removeDB,
	itemIDOrSelector, actorUserID, note string,
) (*ItemResult, error) {
	// Resolve selector to item ID
	itemID, err := cli.ResolveItemSelector(ctx, db, itemIDOrSelector, "wherehouse remove")
	if err != nil {
		return nil, fmt.Errorf("failed to resolve %q: %w", itemIDOrSelector, err)
	}

	// Get current item state
	item, err := db.GetItem(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("item not found: %w", err)
	}

	// Get current location
	location, err := db.GetLocation(ctx, item.LocationID)
	if err != nil {
		return nil, fmt.Errorf("location not found: %w", err)
	}

	// Reject if already removed
	if location.IsSystem && location.CanonicalName == "removed" {
		return nil, fmt.Errorf("item %q is already removed", item.DisplayName)
	}

	// Validate from_location matches projection (CRITICAL for event-sourcing integrity)
	if validateErr := db.ValidateFromLocation(ctx, itemID, item.LocationID); validateErr != nil {
		return nil, fmt.Errorf("projection validation failed: %w", validateErr)
	}

	// Get the Removed system location ID
	removedLoc, err := db.GetLocationByCanonicalName(ctx, "removed")
	if err != nil {
		return nil, fmt.Errorf("failed to get Removed location: %w", err)
	}

	payload := map[string]any{
		"item_id":              itemID,
		"previous_location_id": item.LocationID,
		"to_location_id":       removedLoc.LocationID,
	}

	eventID, err := db.AppendEvent(ctx, database.ItemRemovedEvent, actorUserID, payload, note)
	if err != nil {
		return nil, fmt.Errorf("failed to create item.removed event: %w", err)
	}

	return &ItemResult{
		ItemID:           itemID,
		DisplayName:      item.DisplayName,
		PreviousLocation: location.DisplayName,
		EventID:          eventID,
	}, nil
}
