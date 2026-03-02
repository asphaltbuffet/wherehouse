package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/asphaltbuffet/wherehouse/internal/database"
	"github.com/asphaltbuffet/wherehouse/internal/nanoid"
)

// LocationItemQuerier is the database query interface required by resolver functions.
// *database.Database satisfies this interface.
type LocationItemQuerier interface {
	GetLocation(ctx context.Context, locationID string) (*database.Location, error)
	GetLocationByCanonicalName(ctx context.Context, canonicalName string) (*database.Location, error)
	GetItem(ctx context.Context, itemID string) (*database.Item, error)
	GetItemsByCanonicalName(ctx context.Context, canonicalName string) ([]*database.Item, error)
}

// ResolveLocation resolves a location by ID or canonical name.
// IDs are verified against the database before being returned.
// Supports both nanoid string format and display/canonical names.
//
// Resolution order:
//  1. Try direct ID lookup in the database
//  2. If not found by ID, try canonical name lookup
//
// Returns the location ID string or error if not found.
func ResolveLocation(ctx context.Context, db LocationItemQuerier, input string) (string, error) {
	// Try direct ID lookup first
	loc, err := db.GetLocation(ctx, input)
	if err == nil {
		return loc.LocationID, nil
	}

	// Try as canonical name
	canonicalName := database.CanonicalizeString(input)
	loc, err = db.GetLocationByCanonicalName(ctx, canonicalName)
	if err != nil {
		if errors.Is(err, database.ErrLocationNotFound) {
			return "", fmt.Errorf("location %q not found", input)
		}
		return "", fmt.Errorf("failed to resolve location %q: %w", input, err)
	}

	return loc.LocationID, nil
}

// LooksLikeID checks if a string looks like a nanoid.
// Returns true if the string is exactly nanoid.IDLength characters from nanoid.Alphabet.
func LooksLikeID(s string) bool {
	if len(s) != nanoid.IDLength {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune(nanoid.Alphabet, c) {
			return false
		}
	}
	return true
}

// ResolveItemSelector resolves an item selector to an item ID.
// Supports three selector types:
//  1. ID (exact ID, verified against database)
//  2. LOCATION:ITEM (both canonical names, filters by location)
//  3. Canonical name (must match exactly 1 item)
//
// The commandName parameter is used in error messages to provide context
// (e.g., "wherehouse move", "wherehouse history").
//
// Returns the item ID string or error if not found or ambiguous.
func ResolveItemSelector(
	ctx context.Context,
	db LocationItemQuerier,
	selector string,
	commandName string,
) (string, error) {
	// Priority 1: ID lookup — try direct DB lookup for any ID-like string
	if LooksLikeID(selector) {
		item, err := db.GetItem(ctx, selector)
		if err == nil {
			return item.ItemID, nil
		}
		// ID-like string but not found, return error
		if errors.Is(err, database.ErrItemNotFound) {
			return "", fmt.Errorf("item with ID %q not found", selector)
		}
		return "", fmt.Errorf("failed to get item %q: %w", selector, err)
	}

	// Also try direct ID lookup for non-nanoid formats (e.g. legacy UUID strings)
	if !strings.Contains(selector, ":") {
		item, err := db.GetItem(ctx, selector)
		if err == nil {
			return item.ItemID, nil
		}
		if !errors.Is(err, database.ErrItemNotFound) {
			return "", fmt.Errorf("failed to get item %q: %w", selector, err)
		}
		// Not found by ID; fall through to name resolution
	}

	// Priority 2: LOCATION:ITEM selector
	if locationPart, itemPart, ok := parseItemSelector(selector); ok {
		return resolveLocationItemSelector(ctx, db, locationPart, itemPart, commandName)
	}

	// Priority 3: Canonical name (MUST be unique)
	return resolveItemByCanonicalName(ctx, db, selector, commandName)
}

// parseItemSelector parses LOCATION:ITEM syntax.
// Returns (locationPart, itemPart, true) if valid selector, ("", "", false) otherwise.
func parseItemSelector(selector string) (string, string, bool) {
	const expectedParts = 2
	parts := strings.SplitN(selector, ":", expectedParts)
	if len(parts) != expectedParts {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
}

// resolveLocationItemSelector resolves a LOCATION:ITEM selector.
// Uses commandName in error messages for context.
func resolveLocationItemSelector(
	ctx context.Context,
	db LocationItemQuerier,
	locationPart, itemPart string,
	commandName string,
) (string, error) {
	// Resolve location by canonical name
	canonicalLocation := database.CanonicalizeString(locationPart)
	location, err := db.GetLocationByCanonicalName(ctx, canonicalLocation)
	if err != nil {
		if errors.Is(err, database.ErrLocationNotFound) {
			return "", fmt.Errorf("location %q not found", locationPart)
		}
		return "", fmt.Errorf("failed to resolve location %q: %w", locationPart, err)
	}

	// Resolve item by canonical name + location filter
	canonicalItem := database.CanonicalizeString(itemPart)
	items, err := db.GetItemsByCanonicalName(ctx, canonicalItem)
	if err != nil {
		return "", fmt.Errorf("failed to resolve item %q: %w", itemPart, err)
	}

	// Filter by location
	var matches []*database.Item
	for _, item := range items {
		if item.LocationID == location.LocationID {
			matches = append(matches, item)
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("item %q not found in location %q", itemPart, locationPart)
	}
	if len(matches) > 1 {
		// Multiple items with same name in same location - should be rare but handle it
		return "", buildAmbiguousItemError(ctx, db, canonicalItem, matches, commandName)
	}

	return matches[0].ItemID, nil
}

// resolveItemByCanonicalName resolves an item by canonical name only.
// Returns error if 0 matches or 2+ matches (must be unique).
// Uses commandName in error messages for context.
func resolveItemByCanonicalName(
	ctx context.Context,
	db LocationItemQuerier,
	input string,
	commandName string,
) (string, error) {
	canonicalName := database.CanonicalizeString(input)
	items, err := db.GetItemsByCanonicalName(ctx, canonicalName)
	if err != nil {
		return "", fmt.Errorf("failed to resolve item %q: %w", input, err)
	}

	switch len(items) {
	case 0:
		return "", fmt.Errorf("item %q not found", input)
	case 1:
		return items[0].ItemID, nil
	default:
		// Multiple matches - build error with IDs and locations
		return "", buildAmbiguousItemError(ctx, db, canonicalName, items, commandName)
	}
}

// buildAmbiguousItemError builds a detailed error message for multiple matches.
// Uses commandName to provide contextual examples.
func buildAmbiguousItemError(
	ctx context.Context,
	db LocationItemQuerier,
	canonicalName string,
	items []*database.Item,
	commandName string,
) error {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("multiple items named %q found:\n", canonicalName))

	for _, item := range items {
		location, _ := db.GetLocation(ctx, item.LocationID)
		locationName := "unknown"
		if location != nil {
			locationName = location.DisplayName
		}
		sb.WriteString(fmt.Sprintf("  - %s (in %s)\n", item.ItemID, locationName))
	}

	sb.WriteString("Use --id to specify exact item:\n")
	sb.WriteString(fmt.Sprintf("  %s --id %s", commandName, items[0].ItemID))

	return errors.New(sb.String())
}
