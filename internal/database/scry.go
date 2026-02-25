package database

import (
	"context"
	"fmt"
	"sort"

	"github.com/agnivade/levenshtein"
	"github.com/goccy/go-json"
)

// similarityThreshold is the maximum Levenshtein distance for "similar item" matching.
// Lower = stricter. Defined here for tuning without interface changes.
const similarityThreshold = 3

// ScryResult contains ranked location suggestions for a missing item.
type ScryResult struct {
	ItemID        string
	DisplayName   string
	CanonicalName string

	// HomeLocation is the most authoritative suggestion.
	// Derived from temp_origin_location_id (projection) or item.created event location_id.
	// This is NEVER nil because every item was created in some location.
	HomeLocation *LocationInfo

	// FoundLocations are locations where the item was physically found before (item.found events).
	// Ordered by occurrence count descending, then most recent event_id descending.
	// Occurrences field is populated; displayed only in verbose mode.
	FoundLocations []*ScoredLocation

	// TempUseLocations are locations the item was taken to temporarily (item.moved, move_type=temporary_use).
	// Ordered by occurrence count descending, then most recent event_id descending.
	// Occurrences field is populated; displayed only in verbose mode.
	TempUseLocations []*ScoredLocation

	// SimilarItemLocations are current locations of similarly-named items (Levenshtein distance <= 3).
	// Ordered by distance ascending.
	SimilarItemLocations []*ScoredLocation
}

// ScoredLocation is a location with ranking metadata.
type ScoredLocation struct {
	Location *LocationInfo

	// Occurrences is the count of times this location appeared in history (categories 2 and 3).
	// For similar-item results, this is 0.
	// This field is used for ranking; displayed only in --verbose mode.
	Occurrences int

	// SimilarItemName is the canonical name of the similar item (category 4 only).
	// Empty for history-based results.
	SimilarItemName string

	// SimilarItemDisplayName is the display name of the similar item (category 4 only).
	// Empty for history-based results.
	SimilarItemDisplayName string

	// LevenshteinDistance is the distance from the target canonical name (category 4 only).
	// 0 for history-based results.
	LevenshteinDistance int
}

// ScryItem returns ranked location suggestions for a missing item.
// item is the already-fetched Item record; callers should not fetch it again.
// Returns a ScryResult with all suggestion categories populated.
// The HomeLocation field is guaranteed non-nil if the item has any events (it always does).
func (d *Database) ScryItem(ctx context.Context, item *Item) (*ScryResult, error) {
	result := &ScryResult{
		ItemID:        item.ItemID,
		DisplayName:   item.DisplayName,
		CanonicalName: item.CanonicalName,
	}

	// Category 1: Home location
	homeLoc, err := d.scryHomeLocation(ctx, item)
	if err != nil {
		return nil, fmt.Errorf("failed to determine home location: %w", err)
	}
	result.HomeLocation = homeLoc

	// Track seen location IDs to exclude from subsequent categories
	seen := make(map[string]bool)
	if homeLoc != nil {
		seen[homeLoc.LocationID] = true
	}

	// Category 2: Previously found locations
	foundLocs, err := d.scryFoundLocations(ctx, item.ItemID, seen)
	if err != nil {
		return nil, fmt.Errorf("failed to get found locations: %w", err)
	}
	result.FoundLocations = foundLocs

	// Category 3: Temporary use locations
	tempLocs, err := d.scryTempUseLocations(ctx, item.ItemID, seen)
	if err != nil {
		return nil, fmt.Errorf("failed to get temporary use locations: %w", err)
	}
	result.TempUseLocations = tempLocs

	// Category 4: Similar item locations
	similarLocs, err := d.scrySimilarItemLocations(ctx, item.CanonicalName, item.ItemID, seen)
	if err != nil {
		return nil, fmt.Errorf("failed to get similar item locations: %w", err)
	}
	result.SimilarItemLocations = similarLocs

	return result, nil
}

// scryHomeLocation returns the home location for an item.
// Uses temp_origin_location_id from the projection if set; otherwise falls back
// to the location_id from the item.created event.
func (d *Database) scryHomeLocation(ctx context.Context, item *Item) (*LocationInfo, error) {
	// If item is in temporary use, temp_origin_location_id is the home location
	if item.TempOriginLocationID != nil {
		loc, err := d.getLocationInfo(ctx, *item.TempOriginLocationID)
		if err != nil {
			return nil, fmt.Errorf("failed to get temp origin location: %w", err)
		}
		return loc, nil
	}

	// Fall back to item.created event location_id
	const query = `
		SELECT payload
		FROM events
		WHERE item_id = ?
		  AND event_type = 'item.created'
		ORDER BY event_id ASC
		LIMIT 1
	`

	var payload []byte
	err := d.db.QueryRowContext(ctx, query, item.ItemID).Scan(&payload)
	if err != nil {
		return nil, fmt.Errorf("failed to query item.created event: %w", err)
	}

	var data map[string]any
	if unmarshalErr := json.Unmarshal(payload, &data); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse item.created payload: %w", unmarshalErr)
	}

	locationID, ok := data["location_id"].(string)
	if !ok || locationID == "" {
		return nil, fmt.Errorf("item.created event missing location_id for item %s", item.ItemID)
	}

	loc, err := d.getLocationInfo(ctx, locationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get creation location: %w", err)
	}

	return loc, nil
}

// locationCount tracks occurrence counts and most recent event_id for a location.
type locationCount struct {
	count      int
	maxEventID int64
}

// scryFoundLocations returns locations where the item was previously found (item.found events).
// Results are ordered by occurrence count descending, then most recent event_id descending.
// System locations and locations already in seen are excluded.
func (d *Database) scryFoundLocations(
	ctx context.Context,
	itemID string,
	seen map[string]bool,
) ([]*ScoredLocation, error) {
	const query = `
		SELECT event_id, payload
		FROM events
		WHERE item_id = ?
		  AND event_type = 'item.found'
		ORDER BY event_id DESC
	`

	rows, err := d.db.QueryContext(ctx, query, itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to query item.found events: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]*locationCount)

	for rows.Next() {
		var eventID int64
		var payload []byte

		if scanErr := rows.Scan(&eventID, &payload); scanErr != nil {
			return nil, fmt.Errorf("failed to scan item.found event: %w", scanErr)
		}

		var data map[string]any
		if unmarshalErr := json.Unmarshal(payload, &data); unmarshalErr != nil {
			continue // Skip malformed payloads
		}

		foundLocID, ok := data["found_location_id"].(string)
		if !ok || foundLocID == "" {
			continue
		}

		if lc, exists := counts[foundLocID]; exists {
			lc.count++
			if eventID > lc.maxEventID {
				lc.maxEventID = eventID
			}
		} else {
			counts[foundLocID] = &locationCount{count: 1, maxEventID: eventID}
		}
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating item.found events: %w", err)
	}

	return d.buildScoredLocations(ctx, counts, seen), nil
}

// scryTempUseLocations returns locations where the item was used temporarily
// (item.moved events with move_type=temporary_use).
// Results are ordered by occurrence count descending, then most recent event_id descending.
// System locations and locations already in seen are excluded.
func (d *Database) scryTempUseLocations(
	ctx context.Context,
	itemID string,
	seen map[string]bool,
) ([]*ScoredLocation, error) {
	const query = `
		SELECT event_id, payload
		FROM events
		WHERE item_id = ?
		  AND event_type = 'item.moved'
		ORDER BY event_id DESC
	`

	rows, err := d.db.QueryContext(ctx, query, itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to query item.moved events: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]*locationCount)

	for rows.Next() {
		var eventID int64
		var payload []byte

		if scanErr := rows.Scan(&eventID, &payload); scanErr != nil {
			return nil, fmt.Errorf("failed to scan item.moved event: %w", scanErr)
		}

		var data map[string]any
		if unmarshalErr := json.Unmarshal(payload, &data); unmarshalErr != nil {
			continue // Skip malformed payloads
		}

		// Only include temporary_use moves
		moveType, _ := data["move_type"].(string)
		if moveType != "temporary_use" {
			continue
		}

		toLocID, ok := data["to_location_id"].(string)
		if !ok || toLocID == "" {
			continue
		}

		if lc, exists := counts[toLocID]; exists {
			lc.count++
			if eventID > lc.maxEventID {
				lc.maxEventID = eventID
			}
		} else {
			counts[toLocID] = &locationCount{count: 1, maxEventID: eventID}
		}
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating item.moved events: %w", err)
	}

	return d.buildScoredLocations(ctx, counts, seen), nil
}

// buildScoredLocations converts a counts map into a sorted []*ScoredLocation slice.
// Locations that are system locations or already in seen are excluded.
// Locations added to the result are also added to seen.
func (d *Database) buildScoredLocations(
	ctx context.Context,
	counts map[string]*locationCount,
	seen map[string]bool,
) []*ScoredLocation {
	// Build sorted list of location IDs
	type locEntry struct {
		locationID string
		count      int
		maxEventID int64
	}

	entries := make([]locEntry, 0, len(counts))
	for locID, lc := range counts {
		entries = append(entries, locEntry{
			locationID: locID,
			count:      lc.count,
			maxEventID: lc.maxEventID,
		})
	}

	// Sort by count desc, then maxEventID desc
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].maxEventID > entries[j].maxEventID
	})

	var results []*ScoredLocation
	for _, entry := range entries {
		if seen[entry.locationID] {
			continue
		}

		loc, err := d.getLocationInfo(ctx, entry.locationID)
		if err != nil {
			continue // Skip if location not found (deleted location, etc.)
		}

		if loc.IsSystem {
			continue // Skip system locations
		}

		seen[entry.locationID] = true
		results = append(results, &ScoredLocation{
			Location:    loc,
			Occurrences: entry.count,
		})
	}

	return results
}

// scrySimilarItemLocations returns locations of items with similar canonical names.
// Uses a LIKE pre-filter then Go-side Levenshtein checking.
// Results are ordered by Levenshtein distance ascending.
// System locations and locations already in seen are excluded.
func (d *Database) scrySimilarItemLocations(
	ctx context.Context,
	canonicalName string,
	itemID string,
	seen map[string]bool,
) ([]*ScoredLocation, error) {
	candidates, err := d.querySimilarItemCandidates(ctx, canonicalName, itemID)
	if err != nil {
		return nil, err
	}

	return filterAndSortSimilarCandidates(candidates, canonicalName, seen), nil
}

// similarCandidate holds data for a single similar-item candidate row.
type similarCandidate struct {
	location    *LocationInfo
	itemName    string // canonical name of the similar item
	displayName string // display name of the similar item
	distance    int
}

// querySimilarItemCandidates fetches and pre-filters candidates from the database.
// Returns one candidate per location (lowest Levenshtein distance wins per location).
func (d *Database) querySimilarItemCandidates(
	ctx context.Context,
	canonicalName string,
	itemID string,
) (map[string]*similarCandidate, error) {
	// SQL pre-filter: substring match to reduce candidates
	searchPattern := "%" + canonicalName + "%"

	const query = `
		SELECT i.item_id, i.canonical_name, i.display_name, i.location_id,
		       l.display_name, l.full_path_display, l.is_system
		FROM items_current i
		INNER JOIN locations_current l ON i.location_id = l.location_id
		WHERE i.canonical_name LIKE ?
		  AND i.item_id != ?
		  AND l.is_system = 0
	`

	rows, err := d.db.QueryContext(ctx, query, searchPattern, itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to query similar items: %w", err)
	}
	defer rows.Close()

	bestByLocation := make(map[string]*similarCandidate)

	for rows.Next() {
		var (
			candidateItemID  string
			candidateCanon   string
			candidateDisplay string
			locationID       string
			locDisplayName   string
			locFullPath      string
			locIsSystemInt   int
		)

		if scanErr := rows.Scan(
			&candidateItemID,
			&candidateCanon,
			&candidateDisplay,
			&locationID,
			&locDisplayName,
			&locFullPath,
			&locIsSystemInt,
		); scanErr != nil {
			return nil, fmt.Errorf("failed to scan similar item row: %w", scanErr)
		}

		dist := levenshtein.ComputeDistance(canonicalName, candidateCanon)
		if dist > similarityThreshold {
			continue
		}

		loc := &LocationInfo{
			LocationID:      locationID,
			DisplayName:     locDisplayName,
			FullPathDisplay: locFullPath,
			IsSystem:        locIsSystemInt != 0,
		}

		if existing, ok := bestByLocation[locationID]; !ok || dist < existing.distance {
			bestByLocation[locationID] = &similarCandidate{
				location:    loc,
				itemName:    candidateCanon,
				displayName: candidateDisplay,
				distance:    dist,
			}
		}
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating similar items: %w", err)
	}

	return bestByLocation, nil
}

// filterAndSortSimilarCandidates converts the bestByLocation map into a sorted
// []*ScoredLocation slice, skipping locations already in seen.
func filterAndSortSimilarCandidates(
	bestByLocation map[string]*similarCandidate,
	_ string,
	seen map[string]bool,
) []*ScoredLocation {
	candidates := make([]*similarCandidate, 0, len(bestByLocation))
	for _, c := range bestByLocation {
		candidates = append(candidates, c)
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].distance != candidates[j].distance {
			return candidates[i].distance < candidates[j].distance
		}
		return candidates[i].location.FullPathDisplay < candidates[j].location.FullPathDisplay
	})

	var results []*ScoredLocation
	for _, c := range candidates {
		if seen[c.location.LocationID] {
			continue
		}

		seen[c.location.LocationID] = true
		results = append(results, &ScoredLocation{
			Location:               c.location,
			SimilarItemName:        c.itemName,
			SimilarItemDisplayName: c.displayName,
			LevenshteinDistance:    c.distance,
		})
	}

	return results
}
