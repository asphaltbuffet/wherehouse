package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"

	"github.com/agnivade/levenshtein"
	"github.com/goccy/go-json"
)

const resultTypeItem = "item"

// LocationInfo contains basic information about a location.
type LocationInfo struct {
	LocationID      string
	DisplayName     string
	FullPathDisplay string
	IsSystem        bool
}

// SearchResult represents a single search result (item or location).
type SearchResult struct {
	Type              string // "item" or "location"
	ItemID            *string
	LocationID        *string
	DisplayName       string
	CanonicalName     string
	CurrentLocation   *LocationInfo // For items only
	FullPath          string        // Display path
	FullPathCanonical string
	IsSystem          bool // For locations
	InTemporaryUse    bool // For items
	IsMissing         bool // Derived from location_id
	IsBorrowed        bool // Derived from location_id
	IsLoaned          bool // Derived from location_id
	// LastNonSystemLocation is populated for missing/borrowed items
	LastNonSystemLocation *LocationInfo
	LevenshteinDistance   int // For result sorting
}

// SearchByName searches for items and locations by canonical name using substring matching.
// Results are ranked by Levenshtein distance (exact matches first) and limited to the specified count.
// A limit of 0 means unlimited results.
func (d *Database) SearchByName(
	ctx context.Context,
	searchTerm string,
	limit int,
) ([]*SearchResult, error) {
	// Canonicalize search term for consistent matching
	canonicalSearchTerm := CanonicalizeString(searchTerm)
	searchPattern := "%" + canonicalSearchTerm + "%"

	// Get system location IDs for missing/borrowed/loaned detection
	missingLocationID, borrowedLocationID, loanedLocationID, err := d.GetSystemLocationIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get system location IDs: %w", err)
	}

	// Execute UNION query for items and locations
	const query = `
		SELECT
			'item' as result_type,
			i.item_id,
			NULL as location_id,
			i.display_name,
			i.canonical_name,
			i.in_temporary_use,
			l.location_id as current_location_id,
			l.display_name as current_location_name,
			l.full_path_display as current_location_path,
			l.is_system as current_location_is_system,
			'' as full_path_canonical,
			0 as is_system
		FROM items_current i
		INNER JOIN locations_current l ON i.location_id = l.location_id
		WHERE i.canonical_name LIKE ?

		UNION ALL

		SELECT
			'location' as result_type,
			NULL as item_id,
			l.location_id,
			l.display_name,
			l.canonical_name,
			0 as in_temporary_use,
			NULL as current_location_id,
			NULL as current_location_name,
			l.full_path_display as current_location_path,
			0 as current_location_is_system,
			l.full_path_canonical,
			l.is_system
		FROM locations_current l
		WHERE l.canonical_name LIKE ?
	`

	rows, err := d.db.QueryContext(ctx, query, searchPattern, searchPattern)
	if err != nil {
		return nil, fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()

	// Scan all results
	results, err := d.scanSearchResults(
		rows,
		canonicalSearchTerm,
		missingLocationID,
		borrowedLocationID,
		loanedLocationID,
	)
	if err != nil {
		return nil, err
	}

	// Enrich results with last non-system locations
	d.enrichResultsWithLastNonSystemLocation(ctx, results)

	// Sort by Levenshtein distance (ascending), then by display name
	sort.Slice(results, func(i, j int) bool {
		if results[i].LevenshteinDistance != results[j].LevenshteinDistance {
			return results[i].LevenshteinDistance < results[j].LevenshteinDistance
		}
		return results[i].DisplayName < results[j].DisplayName
	})

	// Apply limit if specified
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// scanSearchResults scans rows from a search query and constructs SearchResult objects.
func (d *Database) scanSearchResults(
	rows *sql.Rows,
	canonicalSearchTerm string,
	missingLocationID string,
	borrowedLocationID string,
	loanedLocationID string,
) ([]*SearchResult, error) {
	var results []*SearchResult

	for rows.Next() {
		var (
			result                  SearchResult
			itemID                  sql.NullString
			locationID              sql.NullString
			inTemporaryUse          int
			currentLocationID       sql.NullString
			currentLocationName     sql.NullString
			currentLocationPath     sql.NullString
			currentLocationIsSystem int
			isSystem                int
		)

		scanErr := rows.Scan(
			&result.Type,
			&itemID,
			&locationID,
			&result.DisplayName,
			&result.CanonicalName,
			&inTemporaryUse,
			&currentLocationID,
			&currentLocationName,
			&currentLocationPath,
			&currentLocationIsSystem,
			&result.FullPathCanonical,
			&isSystem,
		)
		if scanErr != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", scanErr)
		}

		// Set type-specific fields
		if result.Type == resultTypeItem {
			if itemID.Valid {
				result.ItemID = &itemID.String
			}
			result.InTemporaryUse = inTemporaryUse != 0

			// Set current location
			if currentLocationID.Valid {
				result.CurrentLocation = &LocationInfo{
					LocationID:      currentLocationID.String,
					DisplayName:     currentLocationName.String,
					FullPathDisplay: currentLocationPath.String,
					IsSystem:        currentLocationIsSystem != 0,
				}

				// Check if item is in system location
				result.IsMissing = currentLocationID.String == missingLocationID
				result.IsBorrowed = currentLocationID.String == borrowedLocationID
				result.IsLoaned = currentLocationID.String == loanedLocationID
			}
		} else {
			if locationID.Valid {
				result.LocationID = &locationID.String
			}
			result.IsSystem = isSystem != 0
			result.FullPath = currentLocationPath.String
		}

		// Calculate Levenshtein distance
		result.LevenshteinDistance = levenshtein.ComputeDistance(canonicalSearchTerm, result.CanonicalName)

		results = append(results, &result)
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("error iterating search results: %w", rowsErr)
	}

	return results, nil
}

// enrichResultsWithLastNonSystemLocation populates LastNonSystemLocation for items in system locations.
func (d *Database) enrichResultsWithLastNonSystemLocation(ctx context.Context, results []*SearchResult) {
	for _, result := range results {
		if result.Type == resultTypeItem && (result.IsMissing || result.IsBorrowed || result.IsLoaned) &&
			result.ItemID != nil {
			lastLoc, locErr := d.findLastNonSystemLocation(ctx, *result.ItemID)
			if locErr == nil && lastLoc != nil {
				result.LastNonSystemLocation = lastLoc
			}
			// Ignore errors - item may not have non-system location history
		}
	}
}

// GetSystemLocationIDs retrieves the UUIDs of the Missing, Borrowed, and Loaned system locations.
// Returns (missingID, borrowedID, loanedID, error).
func (d *Database) GetSystemLocationIDs(ctx context.Context) (string, string, string, error) {
	// Check cache first
	if d.missingLocationID != "" && d.borrowedLocationID != "" && d.loanedLocationID != "" {
		return d.missingLocationID, d.borrowedLocationID, d.loanedLocationID, nil
	}

	// Query system locations
	d.systemLocationsOnce.Do(func() {
		_ = d.initSystemLocations(ctx) // Error will be returned below if cache is still empty
	})

	if d.missingLocationID == "" || d.borrowedLocationID == "" || d.loanedLocationID == "" {
		return "", "", "", errors.New("system locations not found in database")
	}

	return d.missingLocationID, d.borrowedLocationID, d.loanedLocationID, nil
}

// findLastNonSystemLocation returns the most recent non-system location for an item
// by joining events with locations_current in a single query.
// Returns nil if no non-system location is found in the event history.
func (d *Database) findLastNonSystemLocation(
	ctx context.Context,
	itemID string,
) (*LocationInfo, error) {
	// Extract location IDs from relevant event payloads using JSON functions,
	// join with locations_current to filter system locations, and return the most recent.
	// Using a single query avoids nested connections against the single-connection pool.
	const query = `
		SELECT l.location_id, l.display_name, l.full_path_display, l.is_system
		FROM (
			SELECT
				CASE event_type
					WHEN 'item.created' THEN json_extract(payload, '$.location_id')
					WHEN 'item.moved'   THEN json_extract(payload, '$.to_location_id')
					WHEN 'item.found'   THEN json_extract(payload, '$.found_location_id')
				END AS location_id,
				event_id
			FROM events
			WHERE item_id = ?
			  AND event_type IN ('item.created', 'item.moved', 'item.found')
		) extracted
		JOIN locations_current l ON extracted.location_id = l.location_id
		WHERE l.is_system = 0
		ORDER BY extracted.event_id DESC
		LIMIT 1
	`

	var loc LocationInfo
	var isSystem int

	err := d.db.QueryRowContext(ctx, query, itemID).Scan(
		&loc.LocationID,
		&loc.DisplayName,
		&loc.FullPathDisplay,
		&isSystem,
	)
	if errors.Is(err, sql.ErrNoRows) {
		// No non-system location found (edge case: item created directly in Missing)
		return nil, nil //nolint:nilnil // returning nil for both is intentional - no error and no location
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find last non-system location: %w", err)
	}

	loc.IsSystem = isSystem != 0

	return &loc, nil
}

// extractLocationFromEvent extracts the location_id from an event payload based on event type.
// Returns (locationID, found) where found is false if the event doesn't set a location.
func extractLocationFromEvent(eventType string, payload []byte) (string, bool) {
	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		return "", false
	}

	switch eventType {
	case "item.created":
		if locID, ok := data["location_id"].(string); ok {
			return locID, true
		}
	case "item.moved":
		if locID, ok := data["to_location_id"].(string); ok {
			return locID, true
		}
	case "item.found":
		if locID, ok := data["found_location_id"].(string); ok {
			return locID, true
		}
	case "item.missing", "item.borrowed", "item.loaned":
		// These events set system locations - skip them
		return "", false
	}

	return "", false
}

// getLocationInfo retrieves basic location information by ID.
func (d *Database) getLocationInfo(ctx context.Context, locationID string) (*LocationInfo, error) {
	const query = `
		SELECT location_id, display_name, full_path_display, is_system
		FROM locations_current
		WHERE location_id = ?
	`

	var loc LocationInfo
	var isSystem int

	err := d.db.QueryRowContext(ctx, query, locationID).Scan(
		&loc.LocationID,
		&loc.DisplayName,
		&loc.FullPathDisplay,
		&isSystem,
	)
	if err == sql.ErrNoRows {
		return nil, ErrLocationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get location info: %w", err)
	}

	loc.IsSystem = isSystem != 0

	return &loc, nil
}
