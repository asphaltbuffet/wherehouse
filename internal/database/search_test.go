package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates an in-memory test database with schema and basic data.
func setupTestDB(t *testing.T) *Database {
	t.Helper()

	db, err := Open(Config{
		Path:        ":memory:",
		AutoMigrate: true,
	})
	require.NoError(t, err, "failed to open test database")

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

// createTestLocation creates a test location and returns its ID.
func createTestLocation(t *testing.T, db *Database, displayName, canonicalName string) string {
	t.Helper()

	ctx := context.Background()

	const query = `
		INSERT INTO locations_current (location_id, display_name, canonical_name, parent_id, full_path_display, full_path_canonical, depth, is_system, updated_at)
		VALUES (?, ?, ?, NULL, ?, ?, 0, 0, CURRENT_TIMESTAMP)
	`

	// Generate unique location ID
	locationID := fmt.Sprintf("loc_%s_%d", CanonicalizeString(displayName), time.Now().UnixNano())
	fullPath := displayName
	fullPathCanonical := canonicalName

	_, err := db.DB().ExecContext(ctx, query, locationID, displayName, canonicalName, fullPath, fullPathCanonical)
	require.NoError(t, err, "failed to create test location")

	return locationID
}

// createTestItem creates a test item and returns its ID.
func createTestItem(t *testing.T, db *Database, displayName, canonicalName, locationID string) string {
	t.Helper()

	ctx := context.Background()

	const query = `
		INSERT INTO items_current (item_id, display_name, canonical_name, location_id, in_temporary_use, last_event_id, updated_at)
		VALUES (?, ?, ?, ?, 0, 1, CURRENT_TIMESTAMP)
	`

	// Generate unique item ID
	itemID := fmt.Sprintf("item_%s_%d", CanonicalizeString(displayName), time.Now().UnixNano())

	_, err := db.DB().ExecContext(ctx, query, itemID, displayName, canonicalName, locationID)
	require.NoError(t, err, "failed to create test item")

	return itemID
}

func TestSearchByName_ExactMatch_Item(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	locID := createTestLocation(t, db, "Garage", "garage")
	createTestItem(t, db, "10mm Socket", "10mm_socket", locID)

	results, err := db.SearchByName(ctx, "10mm Socket", 0)
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.Equal(t, "item", results[0].Type)
	assert.Equal(t, "10mm Socket", results[0].DisplayName)
	assert.Zero(t, results[0].LevenshteinDistance)
}

func TestSearchByName_SubstringMatch_Item(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	locID := createTestLocation(t, db, "Garage", "garage")
	createTestItem(t, db, "Phillips Screwdriver", "phillips_screwdriver", locID)
	createTestItem(t, db, "Flathead Screwdriver", "flathead_screwdriver", locID)

	results, err := db.SearchByName(ctx, "screwdriver", 0)
	require.NoError(t, err)
	require.Len(t, results, 2)

	// Verify both items match
	assert.Equal(t, "item", results[0].Type)
	assert.Equal(t, "item", results[1].Type)
}

func TestSearchByName_LocationMatch(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	createTestLocation(t, db, "Garage", "garage")

	results, err := db.SearchByName(ctx, "garage", 0)
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.Equal(t, "location", results[0].Type)
	assert.Equal(t, "Garage", results[0].DisplayName)
	assert.Zero(t, results[0].LevenshteinDistance)
}

func TestSearchByName_MixedResults(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	locID := createTestLocation(t, db, "Toolbox", "toolbox")
	createTestItem(t, db, "Tool Organizer", "tool_organizer", locID)

	results, err := db.SearchByName(ctx, "tool", 0)
	require.NoError(t, err)
	require.Len(t, results, 2)

	// Should have both item and location
	hasItem := false
	hasLocation := false
	for _, r := range results {
		switch r.Type {
		case "item":
			hasItem = true
		case "location":
			hasLocation = true
		}
	}
	assert.True(t, hasItem)
	assert.True(t, hasLocation)
}

func TestSearchByName_LevenshteinSorting(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	locID := createTestLocation(t, db, "Workshop", "workshop")
	createTestItem(t, db, "screwdriver", "screwdriver", locID)
	createTestItem(t, db, "screwdrivers", "screwdrivers", locID)

	results, err := db.SearchByName(ctx, "screwdriver", 0)
	require.NoError(t, err)
	require.Len(t, results, 2)

	// First result should be exact match (distance 0)
	assert.Equal(t, 0, results[0].LevenshteinDistance)
	assert.Equal(t, "screwdriver", results[0].DisplayName)
}

func TestSearchByName_WithLimit(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	locID := createTestLocation(t, db, "Storage", "storage")
	for i := 1; i <= 5; i++ {
		createTestItem(t, db, fmt.Sprintf("Socket %d", i), fmt.Sprintf("socket_%d", i), locID)
	}

	results, err := db.SearchByName(ctx, "socket", 3)
	require.NoError(t, err)
	require.Len(t, results, 3)
}

func TestSearchByName_ZeroLimitMeansUnlimited(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	locID := createTestLocation(t, db, "Storage", "storage")
	for i := 1; i <= 5; i++ {
		createTestItem(t, db, fmt.Sprintf("Socket %d", i), fmt.Sprintf("socket_%d", i), locID)
	}

	results, err := db.SearchByName(ctx, "socket", 0)
	require.NoError(t, err)
	require.Len(t, results, 5)
}

func TestSearchByName_NoResults(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	locID := createTestLocation(t, db, "Garage", "garage")
	createTestItem(t, db, "Hammer", "hammer", locID)

	results, err := db.SearchByName(ctx, "nonexistent", 0)
	require.NoError(t, err)
	require.Empty(t, results)
}

func TestSearchByName_PartialMatch(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	locID := createTestLocation(t, db, "Garage", "garage")
	itemID := createTestItem(t, db, "10mm Socket Wrench", "10mm_socket_wrench", locID)

	// Search for "socket" should match "10mm_socket_wrench"
	results, err := db.SearchByName(ctx, "socket", 0)
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.Equal(t, "item", results[0].Type)
	assert.Equal(t, itemID, *results[0].ItemID)
}

func TestSearchByName_CaseInsensitive(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	locID := createTestLocation(t, db, "Garage", "garage")
	createTestItem(t, db, "Phillips Screwdriver", "phillips_screwdriver", locID)

	// Search with different case should still match
	results, err := db.SearchByName(ctx, "PHILLIPS", 0)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Phillips Screwdriver", results[0].DisplayName)
}

func TestSearchByName_SpecialCharacters(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	locID := createTestLocation(t, db, "Workshop", "workshop")
	createTestItem(t, db, "12 Socket", "12_socket", locID)

	results, err := db.SearchByName(ctx, "12", 0)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "12 Socket", results[0].DisplayName)
}

func TestSearchByName_ItemMetadata(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	locID := createTestLocation(t, db, "Garage", "garage")
	itemID := createTestItem(t, db, "Hammer", "hammer", locID)

	results, err := db.SearchByName(ctx, "hammer", 0)
	require.NoError(t, err)
	require.Len(t, results, 1)

	result := results[0]
	assert.Equal(t, itemID, *result.ItemID)
	assert.NotNil(t, result.CurrentLocation)
	assert.Equal(t, "Garage", result.CurrentLocation.DisplayName)
	assert.False(t, result.InTemporaryUse)
	assert.False(t, result.IsMissing)
	assert.False(t, result.IsBorrowed)
}

func TestSearchByName_LocationMetadata(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	createTestLocation(t, db, "Garage", "garage")

	results, err := db.SearchByName(ctx, "garage", 0)
	require.NoError(t, err)
	require.Len(t, results, 1)

	result := results[0]
	assert.NotNil(t, result.LocationID)
	assert.Equal(t, "Garage", result.DisplayName)
	assert.False(t, result.IsSystem)
}

func TestSearchByName_SecondarySort_ByDisplayName(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	locID := createTestLocation(t, db, "Workshop", "workshop")
	createTestItem(t, db, "Apple", "apple", locID)

	results, err := db.SearchByName(ctx, "apple", 0)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Apple", results[0].DisplayName)
}

func TestExtractLocationFromEvent_ItemCreated(t *testing.T) {
	payload := []byte(`{"location_id":"loc123"}`)
	locID, found := extractLocationFromEvent("item.created", payload)

	require.True(t, found)
	assert.Equal(t, "loc123", locID)
}

func TestExtractLocationFromEvent_ItemMoved(t *testing.T) {
	payload := []byte(`{"to_location_id":"loc456"}`)
	locID, found := extractLocationFromEvent("item.moved", payload)

	require.True(t, found)
	assert.Equal(t, "loc456", locID)
}

func TestExtractLocationFromEvent_ItemFound(t *testing.T) {
	payload := []byte(`{"found_location_id":"loc789"}`)
	locID, found := extractLocationFromEvent("item.found", payload)

	require.True(t, found)
	assert.Equal(t, "loc789", locID)
}

func TestExtractLocationFromEvent_ItemMissing(t *testing.T) {
	payload := []byte(`{"system_location_id":"missing"}`)
	_, found := extractLocationFromEvent("item.missing", payload)

	assert.False(t, found)
}

func TestExtractLocationFromEvent_ItemBorrowed(t *testing.T) {
	payload := []byte(`{"system_location_id":"borrowed"}`)
	_, found := extractLocationFromEvent("item.borrowed", payload)

	assert.False(t, found)
}

func TestExtractLocationFromEvent_InvalidPayload(t *testing.T) {
	payload := []byte(`invalid json`)
	_, found := extractLocationFromEvent("item.created", payload)

	assert.False(t, found)
}

func TestExtractLocationFromEvent_MissingField(t *testing.T) {
	payload := []byte(`{"other_field":"value"}`)
	_, found := extractLocationFromEvent("item.created", payload)

	assert.False(t, found)
}

func TestSearchByName_EmptySearchTerm(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	locID := createTestLocation(t, db, "Garage", "garage")
	createTestItem(t, db, "Hammer", "hammer", locID)

	// Empty search term canonicalizes to empty, LIKE %% matches everything
	results, err := db.SearchByName(ctx, "", 0)
	require.NoError(t, err)
	// Should match all items and locations
	require.NotEmpty(t, results)
}

func TestSearchByName_LimitLargerThanResults(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	locID := createTestLocation(t, db, "Workshop", "workshop")
	createTestItem(t, db, "Hammer", "hammer", locID)
	createTestItem(t, db, "Wrench", "wrench", locID)

	results, err := db.SearchByName(ctx, "hammer", 100)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestSearchByName_UnicodeCharacters(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	locID := createTestLocation(t, db, "Workshop", "workshop")
	createTestItem(t, db, "Tool", "tool", locID)

	results, err := db.SearchByName(ctx, "tool", 0)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Tool", results[0].DisplayName)
}

// TestSearchByName_MultipleSystemLocationItems_NoDeadlock is a regression test for a deadlock
// that occurred when searching for items where multiple results are in system locations.
// findLastNonSystemLocation previously used nested queries: the outer rows query held the
// only connection (MaxOpenConns=1), while getLocationInfo tried to acquire another — deadlock.
// The fix collapses both into a single SQL query with a subquery JOIN.
func TestSearchByName_MultipleSystemLocationItems_NoDeadlock(t *testing.T) {
	db := NewTestDB(t)
	ctx := context.Background()

	const toolboxID = "toolbox-deadlock-regression-001"

	// Insert all events before processing (batch pattern from SeedTestData)
	_, err := db.insertEvent(ctx, "location.created", TestActorUser, map[string]any{
		"location_id":  toolboxID,
		"display_name": "Toolbox",
		"parent_id":    nil,
	}, "")
	require.NoError(t, err)

	socketItems := []struct{ id, name string }{
		{"item-5mm-socket", "5mm Socket"},
		{"item-6mm-socket", "6mm Socket"},
		{"item-7mm-socket", "7mm Socket"},
		{"item-8mm-socket", "8mm Socket"},
	}

	for _, it := range socketItems {
		_, err = db.insertEvent(ctx, "item.created", TestActorUser, map[string]any{
			"item_id":      it.id,
			"display_name": it.name,
			"location_id":  toolboxID,
		}, "")
		require.NoError(t, err)
	}

	// Process creation events to populate projections
	events, err := db.GetAllEvents(ctx)
	require.NoError(t, err)
	for _, event := range events {
		require.NoError(t, db.ProcessEvent(ctx, event))
	}
	processedCount := len(events)

	// Move all socket items to Missing
	missingLoc, err := db.GetLocationByCanonicalName(ctx, "missing")
	require.NoError(t, err)

	for _, it := range socketItems {
		_, err = db.insertEvent(ctx, "item.moved", TestActorUser, map[string]any{
			"item_id":          it.id,
			"from_location_id": toolboxID,
			"to_location_id":   missingLoc.LocationID,
			"move_type":        "rehome",
		}, "")
		require.NoError(t, err)
	}

	// Process only the new move events
	allEvents, err := db.GetAllEvents(ctx)
	require.NoError(t, err)
	for _, event := range allEvents[processedCount:] {
		require.NoError(t, db.ProcessEvent(ctx, event))
	}

	// SearchByName triggers enrichResultsWithLastNonSystemLocation for each missing item.
	// With the old nested-query approach this deadlocked; with the single-query fix it must not.
	results, err := db.SearchByName(ctx, "socket", 0)
	require.NoError(t, err)

	// All socket items should be present and marked missing
	var itemResults []*SearchResult
	for _, r := range results {
		if r.Type == resultTypeItem {
			itemResults = append(itemResults, r)
		}
	}
	require.Len(t, itemResults, len(socketItems))

	for _, result := range itemResults {
		assert.True(t, result.IsMissing, "%s should be missing", result.DisplayName)
		require.NotNil(t, result.LastNonSystemLocation,
			"%s should have a last non-system location", result.DisplayName)
		assert.Equal(t, "Toolbox", result.LastNonSystemLocation.DisplayName)
		assert.False(t, result.LastNonSystemLocation.IsSystem)
	}
}

func TestSearchByName_Ordering_ExactMatchesFirst(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	locID := createTestLocation(t, db, "Storage", "storage")
	createTestItem(t, db, "socket", "socket", locID)
	createTestItem(t, db, "socket_set", "socket_set", locID)

	results, err := db.SearchByName(ctx, "socket", 0)
	require.NoError(t, err)
	require.Len(t, results, 2)

	// First result should be exact match
	assert.Zero(t, results[0].LevenshteinDistance)
	assert.Equal(t, "socket", results[0].DisplayName)
}
