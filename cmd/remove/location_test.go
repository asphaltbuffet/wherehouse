package remove

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/database"
	"github.com/asphaltbuffet/wherehouse/internal/nanoid"
)

// setupRemoveLocationTest creates an in-memory database with locations for testing.
func setupRemoveLocationTest(t *testing.T) (*database.Database, context.Context, struct {
	parentID string
	emptyID  string
	childID  string
}) {
	t.Helper()

	ctx := context.Background()
	db, err := database.Open(database.Config{
		Path:        ":memory:",
		BusyTimeout: database.DefaultBusyTimeout,
		AutoMigrate: true,
	})
	require.NoError(t, err)

	ids := struct {
		parentID string
		emptyID  string
		childID  string
	}{
		parentID: nanoid.MustNew(),
		emptyID:  nanoid.MustNew(),
		childID:  nanoid.MustNew(),
	}

	err = db.CreateLocation(ctx, ids.parentID, "Garage", nil, false, 0, "2025-01-01T00:00:00Z")
	require.NoError(t, err)

	err = db.CreateLocation(ctx, ids.emptyID, "Empty Shelf", nil, false, 0, "2025-01-01T00:00:01Z")
	require.NoError(t, err)

	childParentID := ids.parentID
	err = db.CreateLocation(ctx, ids.childID, "Child Shelf", &childParentID, false, 1, "2025-01-01T00:00:02Z")
	require.NoError(t, err)

	return db, ctx, ids
}

// Test: Remove empty non-system location marks it as removed.
func TestRemoveLocation_EmptyLocation_Succeeds(t *testing.T) {
	db, ctx, ids := setupRemoveLocationTest(t)
	defer db.Close()

	result, err := removeLocation(ctx, db, ids.emptyID, "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, ids.emptyID, result.LocationID)
	assert.Equal(t, "Empty Shelf", result.DisplayName)
	assert.Positive(t, result.EventID)
}

// Test: Remove location with items returns error.
func TestRemoveLocation_WithItems_Error(t *testing.T) {
	db, ctx, ids := setupRemoveLocationTest(t)
	defer db.Close()

	// Add an item to the garage
	itemID := nanoid.MustNew()
	err := db.CreateItem(ctx, itemID, "tool", ids.parentID, 1, "2025-01-01T00:00:03Z")
	require.NoError(t, err)

	result, err := removeLocation(ctx, db, ids.parentID, "testuser", "")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not empty")
}

// Test: Remove location with child locations returns error.
func TestRemoveLocation_WithChildren_Error(t *testing.T) {
	db, ctx, ids := setupRemoveLocationTest(t)
	defer db.Close()

	// Garage has a child location (childID)
	result, err := removeLocation(ctx, db, ids.parentID, "testuser", "")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not empty")
}

// Test: Remove system location returns error.
func TestRemoveLocation_SystemLocation_Error(t *testing.T) {
	db, ctx, _ := setupRemoveLocationTest(t)
	defer db.Close()

	// Get the Missing system location
	missingLoc, err := db.GetLocationByCanonicalName(ctx, "missing")
	require.NoError(t, err)
	require.NotNil(t, missingLoc)

	result, err := removeLocation(ctx, db, missingLoc.LocationID, "testuser", "")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "system")
}

// Test: Remove the Removed system location itself returns error.
func TestRemoveLocation_RemovedSystemLocation_Error(t *testing.T) {
	db, ctx, _ := setupRemoveLocationTest(t)
	defer db.Close()

	removedLoc, err := db.GetLocationByCanonicalName(ctx, "removed")
	require.NoError(t, err)
	require.NotNil(t, removedLoc)

	result, err := removeLocation(ctx, db, removedLoc.LocationID, "testuser", "")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "system")
}

// Test: Remove location not found returns error.
func TestRemoveLocation_LocationNotFound_Error(t *testing.T) {
	db, ctx, _ := setupRemoveLocationTest(t)
	defer db.Close()

	nonExistentID := nanoid.MustNew()
	result, err := removeLocation(ctx, db, nonExistentID, "testuser", "")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")
}

// Test: Remove location event payload includes previous_parent_id.
func TestRemoveLocation_PayloadIncludesPreviousParentID(t *testing.T) {
	db, ctx, ids := setupRemoveLocationTest(t)
	defer db.Close()

	// ids.childID has ids.parentID as parent — verify payload captures that
	result, err := removeLocation(ctx, db, ids.childID, "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	// Retrieve the event and inspect its payload
	event, err := db.GetEventByID(ctx, result.EventID)
	require.NoError(t, err)

	var payload map[string]any
	err = json.Unmarshal(event.Payload, &payload)
	require.NoError(t, err)

	// previous_parent_id must be present (can be null for root, but child has parent)
	_, hasPreviousParentID := payload["previous_parent_id"]
	assert.True(t, hasPreviousParentID, "location.removed event payload must include previous_parent_id")
	assert.Equal(t, ids.parentID, payload["previous_parent_id"], "previous_parent_id must match actual parent")
}

// Test: Remove root location (no parent) — payload has null previous_parent_id.
func TestRemoveLocation_RootLocation_PayloadHasNullPreviousParentID(t *testing.T) {
	db, ctx, ids := setupRemoveLocationTest(t)
	defer db.Close()

	result, err := removeLocation(ctx, db, ids.emptyID, "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	event, err := db.GetEventByID(ctx, result.EventID)
	require.NoError(t, err)

	var payload map[string]any
	err = json.Unmarshal(event.Payload, &payload)
	require.NoError(t, err)

	_, hasPreviousParentID := payload["previous_parent_id"]
	assert.True(t, hasPreviousParentID, "previous_parent_id key must always be present in payload")
	assert.Nil(t, payload["previous_parent_id"], "root location has no parent")
}

// Test: LocationResult marshals to JSON correctly.
func TestLocationResult_JSONMarshal(t *testing.T) {
	result := &LocationResult{
		LocationID:  "test-loc-id",
		DisplayName: "Empty Shelf",
		EventID:     99,
	}

	jsonBytes, err := json.Marshal(result)
	require.NoError(t, err)

	var unmarshaled map[string]any
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, "test-loc-id", unmarshaled["location_id"])
	assert.Equal(t, "Empty Shelf", unmarshaled["display_name"])
	assert.InDelta(t, float64(99), unmarshaled["event_id"], 0.1)
}
