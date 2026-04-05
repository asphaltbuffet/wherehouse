package move

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
	toolboxID  string
	deskID     string
	missingID  string
	borrowedID string
	itemID1    string
	itemID2    string
	itemID3    string
}

// setupMoveTest creates a test database with locations and items.
func setupMoveTest(t *testing.T) (*database.Database, context.Context, testIDs) {
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
		toolboxID:  uuid.New().String(),
		deskID:     uuid.New().String(),
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
		ids.toolboxID,
		fmt.Sprintf("Tool Box-%s", prefix),
		nil,
		false,
		0,
		"2025-01-01T00:00:01Z",
	)
	require.NoError(t, err)
	err = db.CreateLocation(ctx, ids.deskID, fmt.Sprintf("Desk-%s", prefix), nil, false, 0, "2025-01-01T00:00:02Z")
	require.NoError(t, err)

	// Create system locations (Missing, Borrowed) with unique names
	err = db.CreateLocation(ctx, ids.missingID, fmt.Sprintf("Missing-%s", prefix), nil, true, 0, "2025-01-01T00:00:03Z")
	require.NoError(t, err)
	err = db.CreateLocation(
		ctx,
		ids.borrowedID,
		fmt.Sprintf("Borrowed-%s", prefix),
		nil,
		true,
		0,
		"2025-01-01T00:00:04Z",
	)
	require.NoError(t, err)

	// Create items in garage
	err = db.CreateItem(ctx, ids.itemID1, "10mm socket", ids.garageID, 1, "2025-01-01T00:00:05Z")
	require.NoError(t, err)
	err = db.CreateItem(ctx, ids.itemID2, "wrench", ids.garageID, 2, "2025-01-01T00:00:06Z")
	require.NoError(t, err)
	err = db.CreateItem(ctx, ids.itemID3, "hammer", ids.garageID, 3, "2025-01-01T00:00:07Z")
	require.NoError(t, err)

	// Create project used by project-association tests
	err = db.CreateProject(ctx, "test-project", "active", "2025-01-01T00:00:08Z")
	require.NoError(t, err)

	return db, ctx, ids
}

// Test: Move item - event created successfully.
func TestMoveItem_EventCreated(t *testing.T) {
	db, ctx, ids := setupMoveTest(t)
	defer db.Close()

	// Move item
	result, err := moveItem(ctx, db, ids.itemID1, ids.toolboxID, "rehome", "clear", "", "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify result fields
	assert.Equal(t, ids.itemID1, result.ItemID)
	assert.Equal(t, "10mm socket", result.DisplayName)
	assert.Positive(t, result.EventID)
	assert.Equal(t, "rehome", result.MoveType)
	assert.Equal(t, "clear", result.ProjectAction)
}

// Test: Move item with temporary flag.
func TestMoveItem_TemporaryMove_EventCreated(t *testing.T) {
	db, ctx, ids := setupMoveTest(t)
	defer db.Close()

	// Move with temporary flag
	result, err := moveItem(ctx, db, ids.itemID2, ids.toolboxID, "temporary_use", "clear", "", "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify move type in result
	assert.Equal(t, "temporary_use", result.MoveType)
}

// Test: Move item with project association.
func TestMoveItem_WithProject_EventCreated(t *testing.T) {
	db, ctx, ids := setupMoveTest(t)
	defer db.Close()

	// Move with project
	result, err := moveItem(ctx, db, ids.itemID3, ids.toolboxID, "rehome", "set", "test-project", "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify project action
	assert.Equal(t, "set", result.ProjectAction)
	assert.Equal(t, "test-project", result.ProjectID)
}

// Test: Move item and keep project association.
func TestMoveItem_KeepProject_EventCreated(t *testing.T) {
	db, ctx, ids := setupMoveTest(t)
	defer db.Close()

	// Move with keep-project
	result, err := moveItem(ctx, db, ids.itemID1, ids.toolboxID, "rehome", "keep", "", "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "keep", result.ProjectAction)
}

// Test: Move item and clear project.
func TestMoveItem_ClearProject_EventCreated(t *testing.T) {
	db, ctx, ids := setupMoveTest(t)
	defer db.Close()

	// Move with clear project (default)
	result, err := moveItem(ctx, db, ids.itemID2, ids.toolboxID, "rehome", "clear", "", "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "clear", result.ProjectAction)
	assert.Empty(t, result.ProjectID)
}

// Test: Fail when moving FROM system location (Missing).
func TestMoveItem_FromSystemLocation_Missing_Fails(t *testing.T) {
	db, ctx, ids := setupMoveTest(t)
	defer db.Close()

	// Create item in missing location to test moving from it
	itemInMissingID := uuid.New().String()
	err := db.CreateItem(ctx, itemInMissingID, "lost item", ids.missingID, 4, "2025-01-01T00:00:08Z")
	require.NoError(t, err)

	// Attempt to move from missing location
	result, err := moveItem(ctx, db, itemInMissingID, ids.toolboxID, "rehome", "clear", "", "testuser", "")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "cannot move items from system location")
}

// Test: Fail when moving FROM system location (Borrowed).
func TestMoveItem_FromSystemLocation_Borrowed_Fails(t *testing.T) {
	db, ctx, ids := setupMoveTest(t)
	defer db.Close()

	// Create item in borrowed location
	itemInBorrowedID := uuid.New().String()
	err := db.CreateItem(ctx, itemInBorrowedID, "borrowed item", ids.borrowedID, 5, "2025-01-01T00:00:09Z")
	require.NoError(t, err)

	// Attempt to move from borrowed location
	result, err := moveItem(ctx, db, itemInBorrowedID, ids.toolboxID, "rehome", "clear", "", "testuser", "")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "cannot move items from system location")
}

// Test: Fail when moving TO system location (Missing).
func TestMoveItem_ToSystemLocation_Missing_Fails(t *testing.T) {
	db, ctx, ids := setupMoveTest(t)
	defer db.Close()

	// Attempt to move to missing location
	result, err := moveItem(ctx, db, ids.itemID1, ids.missingID, "rehome", "clear", "", "testuser", "")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "cannot move items to system location")
}

// Test: Fail when moving TO system location (Borrowed).
func TestMoveItem_ToSystemLocation_Borrowed_Fails(t *testing.T) {
	db, ctx, ids := setupMoveTest(t)
	defer db.Close()

	// Attempt to move to borrowed location
	result, err := moveItem(ctx, db, ids.itemID2, ids.borrowedID, "rehome", "clear", "", "testuser", "")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "cannot move items to system location")
}

// Test: Fail when item does not exist.
func TestMoveItem_ItemNotFound_Fails(t *testing.T) {
	db, ctx, ids := setupMoveTest(t)
	defer db.Close()

	// Attempt to move non-existent item
	result, err := moveItem(ctx, db, uuid.New().String(), ids.toolboxID, "rehome", "clear", "", "testuser", "")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "item not found")
}

// Test: Fail when destination location does not exist.
func TestMoveItem_DestinationNotFound_Fails(t *testing.T) {
	db, ctx, ids := setupMoveTest(t)
	defer db.Close()

	// Attempt to move to non-existent location
	result, err := moveItem(ctx, db, ids.itemID3, uuid.New().String(), "rehome", "clear", "", "testuser", "")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "to location not found")
}

// Test: DetermineMoveType function.
func TestDetermineMoveType(t *testing.T) {
	tests := []struct {
		name string
		temp bool
		want string
	}{
		{
			name: "temp flag false returns rehome",
			temp: false,
			want: "rehome",
		},
		{
			name: "temp flag true returns temporary_use",
			temp: true,
			want: "temporary_use",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineMoveType(tt.temp)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Test: DetermineProjectAction function.
func TestDetermineProjectAction(t *testing.T) {
	tests := []struct {
		name        string
		projectID   string
		keepProject bool
		want        string
	}{
		{
			name:        "project ID set returns 'set'",
			projectID:   "test-proj",
			keepProject: false,
			want:        "set",
		},
		{
			name:        "keep project flag true returns 'keep'",
			projectID:   "",
			keepProject: true,
			want:        "keep",
		},
		{
			name:        "default returns 'clear'",
			projectID:   "",
			keepProject: false,
			want:        "clear",
		},
		{
			name:        "project ID takes precedence over keep-project",
			projectID:   "test-proj",
			keepProject: true,
			want:        "set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineProjectAction(tt.projectID, tt.keepProject)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Test: ValidateDestinationNotSystem function.
func TestValidateDestinationNotSystem(t *testing.T) {
	db, ctx, ids := setupMoveTest(t)
	defer db.Close()

	tests := []struct {
		name       string
		locationID string
		wantError  bool
	}{
		{
			name:       "normal location passes validation",
			locationID: ids.toolboxID,
			wantError:  false,
		},
		{
			name:       "system location fails validation",
			locationID: ids.missingID,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDestinationNotSystem(ctx, db, tt.locationID)
			if tt.wantError {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test: Result struct JSON marshaling.
func TestResult_JSONMarshal(t *testing.T) {
	result := &Result{
		ItemID:        uuid.New().String(),
		DisplayName:   "10mm socket",
		FromLocation:  "Garage",
		ToLocation:    "Tool Box",
		EventID:       42,
		MoveType:      "rehome",
		ProjectAction: "clear",
		ProjectID:     "",
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)

	var unmarshaled Result
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, result.ItemID, unmarshaled.ItemID)
	assert.Equal(t, result.DisplayName, unmarshaled.DisplayName)
	assert.Equal(t, result.EventID, unmarshaled.EventID)
}

// Test: Command structure and flag parsing.
func TestGetMoveCmd_Structure(t *testing.T) {
	cmd := NewDefaultMoveCmd()

	assert.NotNil(t, cmd)
	assert.Equal(t, "move", cmd.Name())
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Check required flags
	toFlag := cmd.Flags().Lookup("to")
	require.NotNil(t, toFlag)

	// Check optional flags
	assert.NotNil(t, cmd.Flags().Lookup("temp"))
	assert.NotNil(t, cmd.Flags().Lookup("project"))
	assert.NotNil(t, cmd.Flags().Lookup("keep-project"))
	assert.NotNil(t, cmd.Flags().Lookup("note"))
}

// Test: Move with note.
func TestMoveItem_WithNote_EventCreated(t *testing.T) {
	db, ctx, ids := setupMoveTest(t)
	defer db.Close()

	noteText := "organizing tools"
	result, err := moveItem(ctx, db, ids.itemID1, ids.toolboxID, "rehome", "clear", "", "testuser", noteText)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, ids.itemID1, result.ItemID)
	assert.NotEmpty(t, result.EventID)
}

// Test: Multiple sequential move operations.
func TestMoveItem_MultipleSequential(t *testing.T) {
	db, ctx, ids := setupMoveTest(t)
	defer db.Close()

	// Move first item
	result1, err := moveItem(ctx, db, ids.itemID1, ids.toolboxID, "rehome", "clear", "", "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result1)
	assert.Equal(t, ids.itemID1, result1.ItemID)
	assert.Positive(t, result1.EventID)

	// Move second item
	result2, err := moveItem(ctx, db, ids.itemID2, ids.deskID, "rehome", "clear", "", "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result2)
	assert.Equal(t, ids.itemID2, result2.ItemID)
	assert.Greater(t, result2.EventID, result1.EventID) // Sequential event IDs
}

// Test: Move different items to same destination.
func TestMoveItem_MultipleToSameDestination(t *testing.T) {
	db, ctx, ids := setupMoveTest(t)
	defer db.Close()

	// Move multiple items to toolbox
	result1, err := moveItem(ctx, db, ids.itemID1, ids.toolboxID, "rehome", "clear", "", "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result1)

	result2, err := moveItem(ctx, db, ids.itemID2, ids.toolboxID, "rehome", "clear", "", "testuser", "")
	require.NoError(t, err)
	require.NotNil(t, result2)

	// Verify both have events created
	assert.Positive(t, result1.EventID)
	assert.Positive(t, result2.EventID)
	assert.NotEqual(t, result1.EventID, result2.EventID)
}
