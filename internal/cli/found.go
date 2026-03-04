package cli

import (
	"context"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// foundDB is the database interface required by FoundItem.
// *database.Database satisfies this interface.
type foundDB interface {
	GetItem(ctx context.Context, itemID string) (*database.Item, error)
	GetLocation(ctx context.Context, locationID string) (*database.Location, error)
	ValidateFromLocation(ctx context.Context, itemID, expectedFromLocationID string) error
	AppendEvent(
		ctx context.Context,
		eventType database.EventType,
		actorUserID string,
		payload any,
		note string,
	) (int64, error)
}

// FoundItemResult holds the outcome of marking an item as found.
type FoundItemResult struct {
	ItemID        string
	DisplayName   string
	FoundAt       string
	HomeLocation  string
	Returned      bool
	FoundEventID  int64
	ReturnEventID *int64
	Warnings      []string
}

// FoundItem marks an item as found at the specified location.
// It fires an item.found event and, when returnToHome is true, optionally fires
// a follow-up item.moved rehome event to return the item to its home location.
//
// itemID and foundLocationID must be pre-resolved IDs (not names or selectors).
// Use ResolveItemSelector and ResolveLocation to resolve them before calling this function.
//
// Validates current item state before creating events. Non-fatal state mismatches
// (e.g. item not currently at Missing) are reported as Warnings in the result.
func FoundItem(
	ctx context.Context,
	db foundDB,
	itemID, foundLocationID string,
	returnToHome bool,
	actorUserID, note string,
) (*FoundItemResult, error) {
	// Get current item state
	item, err := db.GetItem(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("item not found: %w", err)
	}

	// Get current item location for warning checks
	currentLoc, err := db.GetLocation(ctx, item.LocationID)
	if err != nil {
		return nil, fmt.Errorf("current location not found: %w", err)
	}

	// Collect non-fatal warnings about the item's current state
	var warnings []string

	switch {
	case currentLoc.IsSystem && currentLoc.CanonicalName == "missing":
		// Normal case: item is at Missing - no warning needed
	case currentLoc.IsSystem:
		// Item is at a non-Missing system location (e.g. Borrowed)
		warnings = append(warnings, fmt.Sprintf(
			"item is currently at system location %q (not Missing)", currentLoc.DisplayName))
	default:
		// Item is at a normal (non-system) location
		warnings = append(warnings, fmt.Sprintf(
			"item is not currently missing (currently at %q)", currentLoc.DisplayName))
	}

	// Determine home location for the item.found event payload.
	// If TempOriginLocationID is NULL, use foundLocationID as a safe fallback
	// so the event handler always receives a valid home_location_id.
	homeLocationID := foundLocationID
	if item.TempOriginLocationID != nil {
		homeLocationID = *item.TempOriginLocationID
	}

	// Get location display names for the result
	foundLoc, err := db.GetLocation(ctx, foundLocationID)
	if err != nil {
		return nil, fmt.Errorf("found location details not found: %w", err)
	}

	homeLoc, err := db.GetLocation(ctx, homeLocationID)
	if err != nil {
		return nil, fmt.Errorf("home location details not found: %w", err)
	}

	// Fire item.found event
	foundPayload := map[string]any{
		"item_id":           itemID,
		"found_location_id": foundLocationID,
		"home_location_id":  homeLocationID,
	}

	foundEventID, err := db.AppendEvent(ctx, database.ItemFoundEvent, actorUserID, foundPayload, note)
	if err != nil {
		return nil, fmt.Errorf("failed to create found event: %w", err)
	}

	result := &FoundItemResult{
		ItemID:       itemID,
		DisplayName:  item.DisplayName,
		FoundAt:      foundLoc.DisplayName,
		HomeLocation: homeLoc.DisplayName,
		Returned:     false,
		FoundEventID: foundEventID,
		Warnings:     warnings,
	}

	// Handle returnToHome flag
	if returnToHome {
		switch {
		case item.TempOriginLocationID == nil:
			// Home is unknown - skip move, add warning
			result.Warnings = append(result.Warnings,
				"home location unknown - could not return item (use move command to return manually)")

		case foundLocationID == homeLocationID:
			// Already at home - skip move, add note
			result.Warnings = append(result.Warnings,
				"already at home location - return skipped")

		default:
			// Validate from_location matches projection (CRITICAL for event-sourcing)
			if validateErr := db.ValidateFromLocation(ctx, itemID, foundLocationID); validateErr != nil {
				return nil, fmt.Errorf("projection validation failed: %w", validateErr)
			}

			// Fire item.moved rehome event to return to home
			movePayload := map[string]any{
				"item_id":          itemID,
				"from_location_id": foundLocationID,
				"to_location_id":   homeLocationID,
				"move_type":        "rehome",
				"project_action":   "clear",
			}

			returnEventID, moveErr := db.AppendEvent(ctx, database.ItemMovedEvent, actorUserID, movePayload, note)
			if moveErr != nil {
				return nil, fmt.Errorf("failed to create return event: %w", moveErr)
			}

			result.Returned = true
			result.ReturnEventID = &returnEventID
		}
	}

	return result, nil
}
