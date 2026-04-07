package remove

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/database"
	"github.com/asphaltbuffet/wherehouse/internal/nanoid"
)

// Test: Removed items do not appear in SearchByName results.
func TestRemovedItem_NotInSearchResults(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(database.Config{
		Path:        ":memory:",
		BusyTimeout: database.DefaultBusyTimeout,
		AutoMigrate: true,
	})
	require.NoError(t, err)
	defer db.Close()

	garageID := nanoid.MustNew()
	err = db.CreateLocation(ctx, garageID, "Garage", nil, false, 0, "2025-01-01T00:00:00Z")
	require.NoError(t, err)

	// Create two items with similar names
	itemID1 := nanoid.MustNew()
	err = db.CreateItem(ctx, itemID1, "10mm socket", garageID, 1, "2025-01-01T00:00:01Z")
	require.NoError(t, err)

	itemID2 := nanoid.MustNew()
	err = db.CreateItem(ctx, itemID2, "10mm socket deep", garageID, 2, "2025-01-01T00:00:02Z")
	require.NoError(t, err)

	// Both should appear in search before removal
	results, err := db.SearchByName(ctx, "10mm socket", 10)
	require.NoError(t, err)

	itemResults := 0
	for _, r := range results {
		if r.Type == "item" {
			itemResults++
		}
	}
	assert.GreaterOrEqual(t, itemResults, 2, "both items should appear in search before removal")

	// Remove the first item
	_, err = removeItem(ctx, db, itemID1, "testuser", "")
	require.NoError(t, err)

	// Removed item should NOT appear in search
	results, err = db.SearchByName(ctx, "10mm socket", 10)
	require.NoError(t, err)

	for _, r := range results {
		if r.Type == "item" && r.ItemID != nil && *r.ItemID == itemID1 {
			t.Errorf("removed item %s should not appear in search results", itemID1)
		}
	}

	// The other item should still appear
	found := false
	for _, r := range results {
		if r.Type == "item" && r.ItemID != nil && *r.ItemID == itemID2 {
			found = true
		}
	}
	assert.True(t, found, "non-removed item should still appear in search results")
}

// Test: Removed items do not appear in GetItemsByLocation for the location.
func TestRemovedItem_NotInGetItemsByLocation(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(database.Config{
		Path:        ":memory:",
		BusyTimeout: database.DefaultBusyTimeout,
		AutoMigrate: true,
	})
	require.NoError(t, err)
	defer db.Close()

	garageID := nanoid.MustNew()
	err = db.CreateLocation(ctx, garageID, "Garage", nil, false, 0, "2025-01-01T00:00:00Z")
	require.NoError(t, err)

	itemID := nanoid.MustNew()
	err = db.CreateItem(ctx, itemID, "wrench", garageID, 1, "2025-01-01T00:00:01Z")
	require.NoError(t, err)

	// Item appears before removal
	items, err := db.GetItemsByLocation(ctx, garageID)
	require.NoError(t, err)
	assert.Len(t, items, 1)

	// Remove the item
	_, err = removeItem(ctx, db, itemID, "testuser", "")
	require.NoError(t, err)

	// Item no longer in original location
	items, err = db.GetItemsByLocation(ctx, garageID)
	require.NoError(t, err)
	assert.Empty(t, items)

	// Item also not visible in Removed system location results
	removedLoc, err := db.GetLocationByCanonicalName(ctx, "removed")
	require.NoError(t, err)
	require.NotNil(t, removedLoc)

	items, err = db.GetItemsByLocation(ctx, removedLoc.LocationID)
	require.NoError(t, err)
	// Items in Removed location should be excluded from normal listing
	assert.Empty(t, items)
}
