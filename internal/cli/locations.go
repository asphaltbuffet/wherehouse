package cli

import (
	"context"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/database"
	"github.com/asphaltbuffet/wherehouse/internal/nanoid"
)

// AddLocationResult holds the outcome of a single location creation.
type AddLocationResult struct {
	LocationID      string
	DisplayName     string
	FullPathDisplay string // empty if fetch failed post-creation
}

// addLocationsDB is the database interface required by AddLocations.
// *database.Database satisfies this interface.
type addLocationsDB interface {
	LocationItemQuerier
	ValidateLocationExists(ctx context.Context, locationID string) error
	ValidateUniqueLocationName(ctx context.Context, canonicalName string, excludeLocationID *string) error
	AppendEvent(
		ctx context.Context,
		eventType database.EventType,
		actorUserID string,
		payload any,
		note string,
	) (int64, error)
}

// AddLocations creates one or more named locations in the database.
// If parentName is non-empty, all locations are created as children of that parent.
// parentName may be a canonical name or ID; it is resolved via ResolveLocation.
// Fails fast on the first error (validation, uniqueness, or event insertion).
func AddLocations(ctx context.Context, names []string, parentName string) ([]AddLocationResult, error) {
	db, err := OpenDatabase(ctx)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	return addLocations(ctx, db, names, parentName)
}

// addLocations is the injectable implementation used by AddLocations and tests.
func addLocations(
	ctx context.Context,
	db addLocationsDB,
	names []string,
	parentName string,
) ([]AddLocationResult, error) {
	actorUserID := GetActorUserID(ctx)

	// Resolve parent location (if provided)
	var parentID *string
	if parentName != "" {
		resolved, resolveErr := ResolveLocation(ctx, db, parentName)
		if resolveErr != nil {
			return nil, fmt.Errorf("failed to resolve parent location %q: %w", parentName, resolveErr)
		}

		if validateErr := db.ValidateLocationExists(ctx, resolved); validateErr != nil {
			return nil, fmt.Errorf("parent location not found: %w", validateErr)
		}

		parentID = &resolved
	}

	results := make([]AddLocationResult, 0, len(names))

	for _, locationName := range names {
		// Validate no colon in name
		if validateErr := database.ValidateNoColonInName(locationName); validateErr != nil {
			return nil, validateErr // FAIL-FAST: exit on first error
		}

		// Canonicalize name
		canonicalName := database.CanonicalizeString(locationName)

		// Check uniqueness (CRITICAL: must do before event)
		if uniqueErr := db.ValidateUniqueLocationName(ctx, canonicalName, nil); uniqueErr != nil {
			return nil, fmt.Errorf("location %q already exists: %w", locationName, uniqueErr)
		}

		// Generate ID
		locationID, idErr := nanoid.New()
		if idErr != nil {
			return nil, fmt.Errorf("failed to generate ID for location %q: %w", locationName, idErr)
		}

		// Build event payload
		payload := map[string]any{
			"location_id":    locationID,
			"display_name":   locationName,
			"canonical_name": canonicalName,
			"parent_id":      parentID,
			"is_system":      false,
		}

		// Insert event and update projection atomically
		if _, insertErr := db.AppendEvent(
			ctx,
			database.LocationCreatedEvent,
			actorUserID,
			payload,
			"",
		); insertErr != nil {
			return nil, fmt.Errorf("failed to create location %q: %w", locationName, insertErr)
		}

		result := AddLocationResult{
			LocationID:  locationID,
			DisplayName: locationName,
		}

		// Get full path for display (best-effort; failure does not roll back creation)
		if loc, getErr := db.GetLocation(ctx, locationID); getErr == nil {
			result.FullPathDisplay = loc.FullPathDisplay
		}

		results = append(results, result)
	}

	return results, nil
}
