package remove

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/database"
	"github.com/asphaltbuffet/wherehouse/internal/nanoid"
)

// testIDs holds unique IDs for a test run.
type testIDs struct {
	garageID   string
	removedID  string
	missingID  string
	borrowedID string
	loanedID   string
	itemID1    string
	itemID2    string
	itemID3    string
}

// setupRemoveTest creates an in-memory database with locations and items for testing.
func setupRemoveTest(t *testing.T) (*database.Database, context.Context, testIDs) {
	t.Helper()

	ctx := context.Background()
	db, err := database.Open(database.Config{
		Path:        ":memory:",
		BusyTimeout: database.DefaultBusyTimeout,
		AutoMigrate: true,
	})
	require.NoError(t, err)

	// Generate unique IDs for this test to avoid constraint violations
	prefix := nanoid.MustNew()

	ids := testIDs{
		garageID: nanoid.MustNew(),
		itemID1:  nanoid.MustNew(),
		itemID2:  nanoid.MustNew(),
		itemID3:  nanoid.MustNew(),
	}

	// Create normal location
	err = db.CreateLocation(ctx, ids.garageID, fmt.Sprintf("Garage-%s", prefix), nil, false, 0, "2025-01-01T00:00:00Z")
	require.NoError(t, err)

	// System locations are automatically created by seedSystemLocations() during database initialization.
	removedLoc, err := db.GetLocationByCanonicalName(ctx, "removed")
	require.NoError(t, err)
	require.NotNil(t, removedLoc)
	ids.removedID = removedLoc.LocationID

	missingLoc, err := db.GetLocationByCanonicalName(ctx, "missing")
	require.NoError(t, err)
	require.NotNil(t, missingLoc)
	ids.missingID = missingLoc.LocationID

	borrowedLoc, err := db.GetLocationByCanonicalName(ctx, "borrowed")
	require.NoError(t, err)
	require.NotNil(t, borrowedLoc)
	ids.borrowedID = borrowedLoc.LocationID

	loanedLoc, err := db.GetLocationByCanonicalName(ctx, "loaned")
	require.NoError(t, err)
	require.NotNil(t, loanedLoc)
	ids.loanedID = loanedLoc.LocationID

	// Create items in garage
	err = db.CreateItem(ctx, ids.itemID1, "10mm socket", ids.garageID, 1, "2025-01-01T00:00:03Z")
	require.NoError(t, err)
	err = db.CreateItem(ctx, ids.itemID2, "wrench", ids.garageID, 2, "2025-01-01T00:00:04Z")
	require.NoError(t, err)
	err = db.CreateItem(ctx, ids.itemID3, "hammer", ids.garageID, 3, "2025-01-01T00:00:05Z")
	require.NoError(t, err)

	return db, ctx, ids
}

// Test: Remove item from normal location moves it to Removed system location.
func TestRemoveItem_FromNormalLocation_MovesToRemoved(t *testing.T) {
	db, ctx, ids := setupRemoveTest(t)
	defer db.Close()

	result, err := removeItem(ctx, db, ids.itemID1, "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify result structure
	assert.Equal(t, ids.itemID1, result.ItemID)
	assert.Equal(t, "10mm socket", result.DisplayName)
	assert.Positive(t, result.EventID)
	assert.NotEmpty(t, result.PreviousLocation)

	// Verify item is now in Removed location
	item, err := db.GetItem(ctx, ids.itemID1)
	require.NoError(t, err)
	require.NotNil(t, item)
	assert.Equal(t, ids.removedID, item.LocationID)
}

// Test: Remove item from Missing system location succeeds (can remove from any location).
func TestRemoveItem_FromMissingLocation_Succeeds(t *testing.T) {
	db, ctx, ids := setupRemoveTest(t)
	defer db.Close()

	// Create item already in missing location
	missingItemID := nanoid.MustNew()
	err := db.CreateItem(ctx, missingItemID, "missing tool", ids.missingID, 4, "2025-01-01T00:00:06Z")
	require.NoError(t, err)

	result, err := removeItem(ctx, db, missingItemID, "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify item is now in Removed location
	item, err := db.GetItem(ctx, missingItemID)
	require.NoError(t, err)
	require.NotNil(t, item)
	assert.Equal(t, ids.removedID, item.LocationID)
}

// Test: Remove item from Borrowed system location succeeds.
func TestRemoveItem_FromBorrowedLocation_Succeeds(t *testing.T) {
	db, ctx, ids := setupRemoveTest(t)
	defer db.Close()

	borrowedItemID := nanoid.MustNew()
	err := db.CreateItem(ctx, borrowedItemID, "borrowed tool", ids.borrowedID, 5, "2025-01-01T00:00:07Z")
	require.NoError(t, err)

	result, err := removeItem(ctx, db, borrowedItemID, "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	item, err := db.GetItem(ctx, borrowedItemID)
	require.NoError(t, err)
	require.NotNil(t, item)
	assert.Equal(t, ids.removedID, item.LocationID)
}

// Test: Remove item from Loaned system location succeeds.
func TestRemoveItem_FromLoanedLocation_Succeeds(t *testing.T) {
	db, ctx, ids := setupRemoveTest(t)
	defer db.Close()

	loanedItemID := nanoid.MustNew()
	err := db.CreateItem(ctx, loanedItemID, "loaned tool", ids.loanedID, 6, "2025-01-01T00:00:08Z")
	require.NoError(t, err)

	result, err := removeItem(ctx, db, loanedItemID, "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	item, err := db.GetItem(ctx, loanedItemID)
	require.NoError(t, err)
	require.NotNil(t, item)
	assert.Equal(t, ids.removedID, item.LocationID)
}

// Test: Remove item already in Removed location returns error.
func TestRemoveItem_AlreadyRemoved_Error(t *testing.T) {
	db, ctx, ids := setupRemoveTest(t)
	defer db.Close()

	// Create item already in removed location
	alreadyRemovedID := nanoid.MustNew()
	err := db.CreateItem(ctx, alreadyRemovedID, "already removed", ids.removedID, 7, "2025-01-01T00:00:09Z")
	require.NoError(t, err)

	result, err := removeItem(ctx, db, alreadyRemovedID, "testuser", "")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "already removed")
}

// Test: Remove item not found returns error.
func TestRemoveItem_ItemNotFound_Error(t *testing.T) {
	db, ctx, _ := setupRemoveTest(t)
	defer db.Close()

	nonExistentID := nanoid.MustNew()
	result, err := removeItem(ctx, db, nonExistentID, "testuser", "")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")
}

// Test: Remove item with note creates event.
func TestRemoveItem_WithNote_EventCreated(t *testing.T) {
	db, ctx, ids := setupRemoveTest(t)
	defer db.Close()

	note := "broken beyond repair"
	result, err := removeItem(ctx, db, ids.itemID1, "testuser", note)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Positive(t, result.EventID)
	assert.NotEmpty(t, result.DisplayName)
}

// Test: Remove multiple items independently succeeds.
func TestRemoveItem_MultipleItems_Independent(t *testing.T) {
	db, ctx, ids := setupRemoveTest(t)
	defer db.Close()

	result1, err := removeItem(ctx, db, ids.itemID1, "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result1)

	result2, err := removeItem(ctx, db, ids.itemID2, "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result2)

	assert.NotEqual(t, result1.EventID, result2.EventID)

	item1, err := db.GetItem(ctx, ids.itemID1)
	require.NoError(t, err)
	item2, err := db.GetItem(ctx, ids.itemID2)
	require.NoError(t, err)

	// Both should be in the Removed location
	assert.Equal(t, ids.removedID, item1.LocationID)
	assert.Equal(t, ids.removedID, item2.LocationID)
}

// Test: ItemResult marshals to JSON correctly.
func TestItemResult_JSONMarshal(t *testing.T) {
	result := &ItemResult{
		ItemID:           "test-id",
		DisplayName:      "10mm socket",
		PreviousLocation: "Garage",
		EventID:          42,
	}

	jsonBytes, err := json.Marshal(result)
	require.NoError(t, err)

	var unmarshaled map[string]any
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, "test-id", unmarshaled["item_id"])
	assert.Equal(t, "10mm socket", unmarshaled["display_name"])
	assert.Equal(t, "Garage", unmarshaled["previous_location"])
	assert.InDelta(t, float64(42), unmarshaled["event_id"], 0.1)
}
