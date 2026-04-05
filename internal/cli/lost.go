package cli

import (
	"context"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// LostItemResult holds the outcome of marking an item as lost.
type LostItemResult struct {
	ItemID           string
	DisplayName      string
	PreviousLocation string
	EventID          int64
}

// LostItemOptions configures optional behavior for LostItem.
type LostItemOptions struct {
	// Note is an optional free-text note attached to the event.
	Note string
}

// lostDB is the database interface required by LostItem.
// *database.Database satisfies this interface.
type lostDB interface {
	LocationItemQuerier
	ValidateFromLocation(ctx context.Context, itemID, expectedFromLocationID string) error
	AppendEvent(
		ctx context.Context,
		eventType database.EventType,
		actorUserID string,
		payload any,
		note string,
	) (int64, error)
}

// LostItem marks an item as lost (moved to the system Missing location).
// Validates item state (must not already be missing) before creating the ItemMissing event.
// The itemSelector may be an ID, a LOCATION:ITEM selector, or a canonical name.
// actorUserID is the user performing the action.
func LostItem(
	ctx context.Context,
	db lostDB,
	itemSelector string,
	actorUserID string,
	opts LostItemOptions,
) (*LostItemResult, error) {
	// Resolve item selector to an item ID
	itemID, err := ResolveItemSelector(ctx, db, itemSelector, "wherehouse lost")
	if err != nil {
		return nil, fmt.Errorf("failed to resolve %q: %w", itemSelector, err)
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

	// ERROR if already missing (prevents duplicate events)
	if location.IsSystem && location.CanonicalName == "missing" {
		return nil, fmt.Errorf("item %q is already marked as missing", item.DisplayName)
	}

	// Borrowed items CAN be marked as missing (no special handling needed)

	// Validate from_location matches projection (CRITICAL for event-sourcing)
	if validateErr := db.ValidateFromLocation(ctx, itemID, item.LocationID); validateErr != nil {
		return nil, fmt.Errorf("projection validation failed: %w", validateErr)
	}

	// Build event payload
	payload := map[string]any{
		"item_id":              itemID,
		"previous_location_id": item.LocationID,
	}

	// Insert event and update projection atomically
	eventID, err := db.AppendEvent(ctx, database.ItemMissingEvent, actorUserID, payload, opts.Note)
	if err != nil {
		return nil, fmt.Errorf("failed to create marked_missing event: %w", err)
	}

	// Build result
	result := &LostItemResult{
		ItemID:           itemID,
		DisplayName:      item.DisplayName,
		PreviousLocation: location.DisplayName,
		EventID:          eventID,
	}

	return result, nil
}
