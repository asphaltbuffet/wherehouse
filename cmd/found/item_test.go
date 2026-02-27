package found

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// testIDs holds unique IDs for a test run.
type testIDs struct {
	garageID   string
	shelfID    string
	toteID     string
	missingID  string
	borrowedID string
	itemID1    string
	itemID2    string
	itemID3    string
}

// setupFoundTest creates a test database with locations and items.
func setupFoundTest(t *testing.T) (*database.Database, context.Context, testIDs) {
	t.Helper()

	ctx := context.Background()
	db, err := database.Open(database.Config{
		Path:        ":memory:",
		BusyTimeout: database.DefaultBusyTimeout,
		AutoMigrate: true,
	})
	require.NoError(t, err)

	// Generate unique IDs for this test to avoid constraint violations
	prefix := uuid.New().String()[:8]

	ids := testIDs{
		garageID:   uuid.New().String(),
		shelfID:    uuid.New().String(),
		toteID:     uuid.New().String(),
		missingID:  uuid.New().String(),
		borrowedID: uuid.New().String(),
		itemID1:    uuid.New().String(),
		itemID2:    uuid.New().String(),
		itemID3:    uuid.New().String(),
	}

	// Create normal locations with unique display names
	err = db.CreateLocation(ctx, ids.garageID, fmt.Sprintf("Garage-%s", prefix), nil, false, 0, "2025-01-01T00:00:00Z")
	require.NoError(t, err)
	err = db.CreateLocation(
		ctx,
		ids.shelfID,
		fmt.Sprintf("Shelf-%s", prefix),
		nil,
		false,
		0,
		"2025-01-01T00:00:01Z",
	)
	require.NoError(t, err)
	err = db.CreateLocation(ctx, ids.toteID, fmt.Sprintf("Tote F-%s", prefix), nil, false, 0, "2025-01-01T00:00:02Z")
	require.NoError(t, err)

	// System locations (Missing and Borrowed) are automatically created by seedSystemLocations()
	// during database initialization. We just need to retrieve them.
	missingLoc, err := db.GetLocationByCanonicalName(ctx, "missing")
	require.NoError(t, err)
	require.NotNil(t, missingLoc)
	ids.missingID = missingLoc.LocationID

	borrowedLoc, err := db.GetLocationByCanonicalName(ctx, "borrowed")
	require.NoError(t, err)
	require.NotNil(t, borrowedLoc)
	ids.borrowedID = borrowedLoc.LocationID

	// Create items in missing location (typical starting state for found command)
	err = db.CreateItem(ctx, ids.itemID1, "10mm socket", ids.missingID, 1, "2025-01-01T00:00:05Z")
	require.NoError(t, err)
	err = db.CreateItem(ctx, ids.itemID2, "wrench", ids.missingID, 2, "2025-01-01T00:00:06Z")
	require.NoError(t, err)
	err = db.CreateItem(ctx, ids.itemID3, "hammer", ids.missingID, 3, "2025-01-01T00:00:07Z")
	require.NoError(t, err)

	return db, ctx, ids
}

// =====================================================================
// HAPPY PATH TESTS
// =====================================================================

// Test 1: Basic found - item at Missing, found at valid location, no return.
func TestFoundItem_ItemAtMissing_Success(t *testing.T) {
	db, ctx, ids := setupFoundTest(t)
	defer db.Close()

	result, err := foundItem(ctx, db, ids.itemID1, ids.garageID, false, "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify result structure
	assert.Equal(t, ids.itemID1, result.ItemID)
	assert.Equal(t, "10mm socket", result.DisplayName)
	assert.Positive(t, result.FoundEventID)
	assert.False(t, result.Returned)
	assert.Nil(t, result.ReturnEventID)
	assert.Empty(t, result.Warnings) // No warning for item at Missing

	// Verify item projection was updated
	item, err := db.GetItem(ctx, ids.itemID1)
	require.NoError(t, err)
	require.NotNil(t, item)

	assert.Equal(t, ids.garageID, item.LocationID)
	assert.True(t, item.InTemporaryUse)
	assert.NotNil(t, item.TempOriginLocationID)
}

// Test 2: Found + return - item at Missing with known home, returns to home.
func TestFoundItem_WithReturn_Success(t *testing.T) {
	db, ctx, ids := setupFoundTest(t)
	defer db.Close()

	// Use a temporary_use move to establish home location properly
	// Move item from Missing to Tote F with temporary_use
	tempMovePayload := map[string]any{
		"item_id":          ids.itemID1,
		"from_location_id": ids.missingID,
		"to_location_id":   ids.toteID,
		"move_type":        "temporary_use",
		"project_action":   "clear",
	}
	_, err := db.AppendEvent(ctx, "item.moved", "testuser", tempMovePayload, "")
	require.NoError(t, err)

	// Now move from Tote F to Shelf (still in temporary use, origin stays Missing)
	tempMovePayload2 := map[string]any{
		"item_id":          ids.itemID1,
		"from_location_id": ids.toteID,
		"to_location_id":   ids.shelfID,
		"move_type":        "temporary_use",
		"project_action":   "clear",
	}
	_, err = db.AppendEvent(ctx, "item.moved", "testuser", tempMovePayload2, "")
	require.NoError(t, err)

	// Move back to Missing to simulate found state
	normalMovePayload := map[string]any{
		"item_id":          ids.itemID1,
		"from_location_id": ids.shelfID,
		"to_location_id":   ids.missingID,
		"move_type":        "normal",
		"project_action":   "clear",
	}
	_, err = db.AppendEvent(ctx, "item.moved", "testuser", normalMovePayload, "")
	require.NoError(t, err)

	// Get item to verify TempOriginLocationID is set to Missing
	item, err := db.GetItem(ctx, ids.itemID1)
	require.NoError(t, err)
	require.NotNil(t, item.TempOriginLocationID)
	assert.Equal(t, ids.missingID, *item.TempOriginLocationID)

	// Now find it at garage and return it (should go back to Missing)
	result, err := foundItem(ctx, db, ids.itemID1, ids.garageID, true, "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify both events were fired
	assert.Equal(t, ids.itemID1, result.ItemID)
	assert.Positive(t, result.FoundEventID)
	assert.True(t, result.Returned)
	assert.NotNil(t, result.ReturnEventID)
	assert.Greater(t, *result.ReturnEventID, result.FoundEventID)
	assert.Empty(t, result.Warnings) // No warning for item at Missing with known home

	// Verify final item location is back at Missing
	finalItem, err := db.GetItem(ctx, ids.itemID1)
	require.NoError(t, err)
	require.NotNil(t, finalItem)

	assert.Equal(t, ids.missingID, finalItem.LocationID)
	assert.False(t, finalItem.InTemporaryUse)
	assert.Nil(t, finalItem.TempOriginLocationID)
}

// Test 3: Found + return, already at home - skips move, adds note.
func TestFoundItem_WithReturn_AlreadyAtHome(t *testing.T) {
	db, ctx, ids := setupFoundTest(t)
	defer db.Close()

	// Use item.found to set the temp_origin_location_id to the found location
	foundPayload1 := map[string]any{
		"item_id":           ids.itemID1,
		"found_location_id": ids.garageID,
		"home_location_id":  ids.garageID,
	}
	_, err := db.AppendEvent(ctx, "item.found", "testuser", foundPayload1, "")
	require.NoError(t, err)

	// Get item to verify TempOriginLocationID is set to Garage
	item, err := db.GetItem(ctx, ids.itemID1)
	require.NoError(t, err)
	require.Equal(t, ids.garageID, item.LocationID)
	require.NotNil(t, item.TempOriginLocationID)
	assert.Equal(t, ids.garageID, *item.TempOriginLocationID)

	// Now find it again at garage with --return
	// Since foundLocationID (Garage) == homeLocationID (Garage from temp_origin), should skip move
	result, err := foundItem(ctx, db, ids.itemID1, ids.garageID, true, "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify only one found event, no return event
	assert.False(t, result.Returned)
	assert.Nil(t, result.ReturnEventID)

	// Verify warnings: first from initial state check (not at missing), then from --return check
	require.Len(t, result.Warnings, 2)
	assert.Contains(t, result.Warnings[0], "item is not currently missing")
	assert.Contains(t, result.Warnings[1], "already at home location")
}

// Test 4: Found + return with NULL home - fires found, skips move, warns.
func TestFoundItem_WithReturn_NullHome(t *testing.T) {
	db, ctx, ids := setupFoundTest(t)
	defer db.Close()

	// Item is at Missing with no TempOriginLocationID (NULL home)
	// This is the default state for items created in Missing

	result, err := foundItem(ctx, db, ids.itemID1, ids.garageID, true, "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify found event only, no return event
	assert.Positive(t, result.FoundEventID)
	assert.False(t, result.Returned)
	assert.Nil(t, result.ReturnEventID)

	// Verify warning about unknown home
	assert.NotEmpty(t, result.Warnings)
	assert.NotEmpty(t, result.Warnings)
	assert.Contains(t, result.Warnings[0], "home location unknown")
}

// =====================================================================
// WARNING CASES (warn and proceed)
// =====================================================================

// Test 5: Warning - item at normal location (not Missing).
func TestFoundItem_ItemAtNormalLocation_Warns(t *testing.T) {
	db, ctx, ids := setupFoundTest(t)
	defer db.Close()

	// Move item from Missing to Garage (establish it as not at Missing)
	movePayload := map[string]any{
		"item_id":          ids.itemID1,
		"from_location_id": ids.missingID,
		"to_location_id":   ids.garageID,
		"move_type":        "normal",
		"project_action":   "clear",
	}
	_, err := db.AppendEvent(ctx, "item.moved", "testuser", movePayload, "")
	require.NoError(t, err)

	// Now find it (still at garage, not missing)
	result, err := foundItem(ctx, db, ids.itemID1, ids.shelfID, false, "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify found event was still created despite warning
	assert.Positive(t, result.FoundEventID)

	// Verify warning about item not being missing
	assert.NotEmpty(t, result.Warnings)
	assert.Contains(t, result.Warnings[0], "item is not currently missing")
}

// Test 6: Warning - item at Borrowed (system, non-Missing).
func TestFoundItem_ItemAtBorrowed_Warns(t *testing.T) {
	db, ctx, ids := setupFoundTest(t)
	defer db.Close()

	// Move item from Missing to Borrowed
	movePayload := map[string]any{
		"item_id":          ids.itemID1,
		"from_location_id": ids.missingID,
		"to_location_id":   ids.borrowedID,
		"move_type":        "normal",
		"project_action":   "clear",
	}
	_, err := db.AppendEvent(ctx, "item.moved", "testuser", movePayload, "")
	require.NoError(t, err)

	// Now find it (at Borrowed)
	result, err := foundItem(ctx, db, ids.itemID1, ids.garageID, false, "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify found event was created
	assert.Positive(t, result.FoundEventID)

	// Verify warning about system location
	assert.NotEmpty(t, result.Warnings)
	assert.Contains(t, result.Warnings[0], "item is currently at system location")
}

// =====================================================================
// ERROR CASES (hard fail)
// =====================================================================

// Test 7: Error - item selector not found.
func TestFoundItem_ItemNotFound_Error(t *testing.T) {
	db, ctx, ids := setupFoundTest(t)
	defer db.Close()

	nonExistentID := uuid.New().String()
	result, err := foundItem(ctx, db, nonExistentID, ids.garageID, false, "testuser", "")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "item not found")
}

// Test 8: Error - found location (--in) is Missing (system location).
func TestFoundItem_FoundAtSystemLocationMissing_Error(t *testing.T) {
	db, ctx, ids := setupFoundTest(t)
	defer db.Close()

	err := validateNotSystemLocation(ctx, db, ids.missingID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot record item as found at system location")
}

// Test 9: Error - found location (--in) is Borrowed (system location).
func TestFoundItem_FoundAtSystemLocationBorrowed_Error(t *testing.T) {
	db, ctx, ids := setupFoundTest(t)
	defer db.Close()

	err := validateNotSystemLocation(ctx, db, ids.borrowedID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot record item as found at system location")
}

// =====================================================================
// EVENT STATE VERIFICATION TESTS
// =====================================================================

// Test 10: item.found sets in_temporary_use flag.
func TestFoundItem_SetsInTemporaryUse(t *testing.T) {
	db, ctx, ids := setupFoundTest(t)
	defer db.Close()

	result, err := foundItem(ctx, db, ids.itemID1, ids.garageID, false, "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	item, err := db.GetItem(ctx, ids.itemID1)
	require.NoError(t, err)
	require.NotNil(t, item)

	assert.True(t, item.InTemporaryUse)
}

// Test 11: item.found sets temp_origin_location_id.
func TestFoundItem_SetsTemporaryOrigin(t *testing.T) {
	db, ctx, ids := setupFoundTest(t)
	defer db.Close()

	result, err := foundItem(ctx, db, ids.itemID1, ids.garageID, false, "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	item, err := db.GetItem(ctx, ids.itemID1)
	require.NoError(t, err)
	require.NotNil(t, item)

	// Home should be fallback to found location (since item had NULL TempOriginLocationID)
	assert.NotNil(t, item.TempOriginLocationID)
	assert.Equal(t, ids.garageID, *item.TempOriginLocationID)
}

// Test 12: item.found + item.moved clears temp state.
func TestFoundItem_WithReturn_ClearsTempState(t *testing.T) {
	db, ctx, ids := setupFoundTest(t)
	defer db.Close()

	// Establish home location using temporary_use
	tempMovePayload := map[string]any{
		"item_id":          ids.itemID1,
		"from_location_id": ids.missingID,
		"to_location_id":   ids.toteID,
		"move_type":        "temporary_use",
		"project_action":   "clear",
	}
	_, err := db.AppendEvent(ctx, "item.moved", "testuser", tempMovePayload, "")
	require.NoError(t, err)

	// Move back to Missing
	normalMovePayload := map[string]any{
		"item_id":          ids.itemID1,
		"from_location_id": ids.toteID,
		"to_location_id":   ids.missingID,
		"move_type":        "normal",
		"project_action":   "clear",
	}
	_, err = db.AppendEvent(ctx, "item.moved", "testuser", normalMovePayload, "")
	require.NoError(t, err)

	// Verify state before found
	itemBefore, err := db.GetItem(ctx, ids.itemID1)
	require.NoError(t, err)
	assert.True(t, itemBefore.InTemporaryUse)
	assert.NotNil(t, itemBefore.TempOriginLocationID)

	result, err := foundItem(ctx, db, ids.itemID1, ids.garageID, true, "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Returned)

	// After return move, verify temp state is cleared
	item, err := db.GetItem(ctx, ids.itemID1)
	require.NoError(t, err)
	require.NotNil(t, item)

	assert.False(t, item.InTemporaryUse)
	assert.Nil(t, item.TempOriginLocationID)
}

// Test 13: Event log has correct count.
func TestFoundItem_EventCount(t *testing.T) {
	db, ctx, ids := setupFoundTest(t)
	defer db.Close()

	// Count events before
	var countBefore int64
	err := db.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM events").Scan(&countBefore)
	require.NoError(t, err)

	// Found without return = 1 event
	result, err := foundItem(ctx, db, ids.itemID1, ids.garageID, false, "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	var countAfter int64
	err = db.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM events").Scan(&countAfter)
	require.NoError(t, err)

	assert.Equal(t, countBefore+1, countAfter, "Expected 1 event for found without return")

	// Now test with return
	// Establish home with temporary_use move
	tempMovePayload := map[string]any{
		"item_id":          ids.itemID2,
		"from_location_id": ids.missingID,
		"to_location_id":   ids.toteID,
		"move_type":        "temporary_use",
		"project_action":   "clear",
	}
	_, err = db.AppendEvent(ctx, "item.moved", "testuser", tempMovePayload, "")
	require.NoError(t, err)

	normalMovePayload := map[string]any{
		"item_id":          ids.itemID2,
		"from_location_id": ids.toteID,
		"to_location_id":   ids.missingID,
		"move_type":        "normal",
		"project_action":   "clear",
	}
	_, err = db.AppendEvent(ctx, "item.moved", "testuser", normalMovePayload, "")
	require.NoError(t, err)

	var countBeforeReturn int64
	err = db.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM events").Scan(&countBeforeReturn)
	require.NoError(t, err)

	// Found with return = 2 events
	result2, err := foundItem(ctx, db, ids.itemID2, ids.garageID, true, "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result2)
	require.True(t, result2.Returned)

	var countAfterReturn int64
	err = db.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM events").Scan(&countAfterReturn)
	require.NoError(t, err)

	assert.Equal(t, countBeforeReturn+2, countAfterReturn, "Expected 2 events for found with return")
}

// =====================================================================
// EDGE CASES
// =====================================================================

// Test 14: Item already at found location (not Missing).
func TestFoundItem_ItemAlreadyAtFoundLocation_Warns(t *testing.T) {
	db, ctx, ids := setupFoundTest(t)
	defer db.Close()

	// Move item to garage
	movePayload := map[string]any{
		"item_id":          ids.itemID1,
		"from_location_id": ids.missingID,
		"to_location_id":   ids.garageID,
		"move_type":        "normal",
		"project_action":   "clear",
	}
	_, err := db.AppendEvent(ctx, "item.moved", "testuser", movePayload, "")
	require.NoError(t, err)

	// Find it at garage (where it already is)
	result, err := foundItem(ctx, db, ids.itemID1, ids.garageID, false, "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should still fire event despite location unchanged
	assert.Positive(t, result.FoundEventID)

	// Should warn about not being missing
	assert.NotEmpty(t, result.Warnings)
	assert.Contains(t, result.Warnings[0], "item is not currently missing")
}

// Test 15: With note - event is created with note.
func TestFoundItem_WithNote_EventCreated(t *testing.T) {
	db, ctx, ids := setupFoundTest(t)
	defer db.Close()

	note := "found behind workbench"
	result, err := foundItem(ctx, db, ids.itemID1, ids.garageID, false, "testuser", note)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify event was created (note acceptance is implicit)
	assert.Positive(t, result.FoundEventID)
}

// Test 16: Different actors create events with correct attribution.
func TestFoundItem_DifferentActors_EventCreated(t *testing.T) {
	db, ctx, ids := setupFoundTest(t)
	defer db.Close()

	actors := []string{"alice", "bob", "charlie"}
	for i, actor := range actors {
		itemID := ids.itemID1
		switch i {
		case 1:
			itemID = ids.itemID2
		case 2:
			itemID = ids.itemID3
		}

		result, err := foundItem(ctx, db, itemID, ids.garageID, false, actor, "")
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Positive(t, result.FoundEventID)
	}
}

// =====================================================================
// OUTPUT FORMAT TESTS
// =====================================================================

// Test 17: Result struct marshals to JSON correctly.
func TestResult_JSONMarshal(t *testing.T) {
	result := &Result{
		ItemID:       "550e8400-e29b-41d4-a716-446655440000",
		DisplayName:  "10mm socket",
		FoundAt:      "Garage",
		HomeLocation: "Tote F",
		Returned:     false,
		FoundEventID: 42,
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(result)
	require.NoError(t, err)

	// Unmarshal to verify structure
	var unmarshaled map[string]any
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	require.NoError(t, err)

	// Verify fields
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", unmarshaled["item_id"])
	assert.Equal(t, "10mm socket", unmarshaled["display_name"])
	assert.Equal(t, "Garage", unmarshaled["found_at"])
	assert.Equal(t, "Tote F", unmarshaled["home_location"])
	assert.False(t, unmarshaled["returned"].(bool))
	assert.InDelta(t, float64(42), unmarshaled["found_event_id"], 0.1)
}

// Test 18: Result struct has correct JSON field names.
func TestResult_JSONFieldNames(t *testing.T) {
	result := &Result{
		ItemID:        "test-id",
		DisplayName:   "test-item",
		FoundAt:       "test-location",
		HomeLocation:  "test-home",
		Returned:      true,
		FoundEventID:  99,
		ReturnEventID: intPtr(100),
	}

	jsonBytes, err := json.Marshal(result)
	require.NoError(t, err)

	jsonStr := string(jsonBytes)
	assert.Contains(t, jsonStr, "item_id")
	assert.Contains(t, jsonStr, "display_name")
	assert.Contains(t, jsonStr, "found_at")
	assert.Contains(t, jsonStr, "home_location")
	assert.Contains(t, jsonStr, "returned")
	assert.Contains(t, jsonStr, "found_event_id")
	assert.Contains(t, jsonStr, "return_event_id")
}

// Test 19: JSON output includes warnings array.
func TestResult_JSONWithWarnings(t *testing.T) {
	result := &Result{
		ItemID:       "test-id",
		DisplayName:  "test-item",
		FoundAt:      "Garage",
		HomeLocation: "Garage",
		Returned:     false,
		FoundEventID: 42,
		Warnings:     []string{"home location unknown - could not return item (use move command to return manually)"},
	}

	jsonBytes, err := json.Marshal(result)
	require.NoError(t, err)

	var unmarshaled map[string]any
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	require.NoError(t, err)

	// Verify warnings are present
	warnings, ok := unmarshaled["warnings"]
	assert.True(t, ok)
	assert.NotEmpty(t, warnings)
}

// Test 20: JSON output with return event ID.
func TestResult_JSONWithReturnEventID(t *testing.T) {
	returnEventID := int64(43)
	result := &Result{
		ItemID:        "test-id",
		DisplayName:   "test-item",
		FoundAt:       "Garage",
		HomeLocation:  "Tote F",
		Returned:      true,
		FoundEventID:  42,
		ReturnEventID: &returnEventID,
	}

	jsonBytes, err := json.Marshal(result)
	require.NoError(t, err)

	var unmarshaled map[string]any
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	require.NoError(t, err)

	// Verify return event ID is present and has correct value
	assert.InDelta(t, float64(43), unmarshaled["return_event_id"], 0.1)
}

// Test 21: Success message formatting without return.
func TestFormatSuccessMessage_WithoutReturn(t *testing.T) {
	result := &Result{
		DisplayName:  "10mm socket",
		FoundAt:      "Garage",
		HomeLocation: "Tote F",
		Returned:     false,
	}

	msg := formatSuccessMessage(result)
	assert.Contains(t, msg, "Found")
	assert.Contains(t, msg, "10mm socket")
	assert.Contains(t, msg, "Garage")
	assert.Contains(t, msg, "Tote F")
	assert.Contains(t, msg, "home:")
	assert.NotContains(t, msg, "returned to")
}

// Test 22: Success message formatting with return.
func TestFormatSuccessMessage_WithReturn(t *testing.T) {
	result := &Result{
		DisplayName:  "10mm socket",
		FoundAt:      "Garage",
		HomeLocation: "Tote F",
		Returned:     true,
	}

	msg := formatSuccessMessage(result)
	assert.Contains(t, msg, "Found")
	assert.Contains(t, msg, "10mm socket")
	assert.Contains(t, msg, "Garage")
	assert.Contains(t, msg, "Tote F")
	assert.Contains(t, msg, "returned to")
	assert.NotContains(t, msg, "home:")
}

// =====================================================================
// HELPER FUNCTION
// =====================================================================

// intPtr returns a pointer to an int64 value (helper for tests).
func intPtr(i int64) *int64 {
	return &i
}
