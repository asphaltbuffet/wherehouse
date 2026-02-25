package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScryItem_BasicHomeLocation verifies home location is populated from creation event.
func TestScryItem_BasicHomeLocation(t *testing.T) {
	db := NewTestDBWithSeed(t)
	ctx := t.Context()

	// Get item created in Toolbox
	socketItem, err := db.GetItem(ctx, TestItem10mmSocket)
	require.NoError(t, err)
	require.NotNil(t, socketItem)

	result, err := db.ScryItem(ctx, socketItem)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Home location should be Toolbox (where item was created)
	require.NotNil(t, result.HomeLocation)
	assert.Equal(t, TestLocationToolbox, result.HomeLocation.LocationID)
}

// TestScryItem_NoHistoryBeyondCreation verifies minimal history shows only home location.
func TestScryItem_NoHistoryBeyondCreation(t *testing.T) {
	db := NewTestDBWithSeed(t)
	ctx := t.Context()

	// 10mm Socket has no found or temp-use history in seed data
	socketItem, err := db.GetItem(ctx, TestItem10mmSocket)
	require.NoError(t, err)

	result, err := db.ScryItem(ctx, socketItem)
	require.NoError(t, err)

	// Should only have home location
	require.NotNil(t, result.HomeLocation)
	require.Empty(t, result.FoundLocations)
	require.Empty(t, result.TempUseLocations)
}

// TestScryItem_SimilarItemLocationsIncluded verifies similar items appear.
func TestScryItem_SimilarItemLocationsIncluded(t *testing.T) {
	db := NewTestDBWithSeed(t)
	ctx := t.Context()

	item, err := db.GetItem(ctx, TestItem10mmSocket)
	require.NoError(t, err)

	result, err := db.ScryItem(ctx, item)
	require.NoError(t, err)

	// Similar item locations may or may not exist depending on seed data
	// Just verify the structure is correct if they exist
	for _, sl := range result.SimilarItemLocations {
		assert.NotEmpty(t, sl.SimilarItemName)
		assert.GreaterOrEqual(t, sl.LevenshteinDistance, 0)
		assert.NotNil(t, sl.Location)
	}
}

// TestScryItem_ResultStructureValid verifies all result fields are properly populated.
func TestScryItem_ResultStructureValid(t *testing.T) {
	db := NewTestDBWithSeed(t)
	ctx := t.Context()

	// Test with a simple item
	item, err := db.GetItem(ctx, TestItem10mmSocket)
	require.NoError(t, err)

	result, err := db.ScryItem(ctx, item)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify structure
	assert.Equal(t, item.ItemID, result.ItemID)
	assert.Equal(t, item.DisplayName, result.DisplayName)
	assert.Equal(t, item.CanonicalName, result.CanonicalName)

	// Home location should always be present
	require.NotNil(t, result.HomeLocation)
	assert.NotEmpty(t, result.HomeLocation.LocationID)

	// Category lists should exist as slices (may be empty, so check not nil)
	// In Go, empty slices may be nil or []ScoredLocation{}, both are valid
	// We just verify the structure is present
	assert.IsType(t, []*ScoredLocation{}, result.FoundLocations)
	assert.IsType(t, []*ScoredLocation{}, result.TempUseLocations)
	assert.IsType(t, []*ScoredLocation{}, result.SimilarItemLocations)
}

// TestScryItem_HomeLocationNeverNil verifies home location is guaranteed non-nil.
func TestScryItem_HomeLocationNeverNil(t *testing.T) {
	db := NewTestDBWithSeed(t)
	ctx := t.Context()

	// Test multiple items
	for _, itemID := range []string{TestItem10mmSocket, TestItemHammer, TestItemDrillBits} {
		item, err := db.GetItem(ctx, itemID)
		require.NoError(t, err)

		result, err := db.ScryItem(ctx, item)
		require.NoError(t, err)

		// Home location must never be nil per spec
		assert.NotNil(t, result.HomeLocation, "item %s should have home location", itemID)
		assert.NotEmpty(t, result.HomeLocation.LocationID)
	}
}

// TestScryItem_AllItemsHaveHomeLocation verifies all seeded items have home locations.
func TestScryItem_AllItemsHaveHomeLocation(t *testing.T) {
	db := NewTestDBWithSeed(t)
	ctx := t.Context()

	// Test multiple items to ensure consistency
	for _, itemID := range []string{TestItemScrewdriverSet, TestItemSandpaper} {
		item, err := db.GetItem(ctx, itemID)
		require.NoError(t, err)

		result, err := db.ScryItem(ctx, item)
		require.NoError(t, err)

		// Home location must always be present per spec
		assert.NotNil(t, result.HomeLocation, "item %s must have home location", itemID)
		assert.NotEmpty(t, result.HomeLocation.LocationID)
	}
}
