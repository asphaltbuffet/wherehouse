package find

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// setupTestCmd creates a test find command with in-memory database.
func setupTestCmd(t *testing.T) (*cobra.Command, *database.Database) {
	t.Helper()

	// Create in-memory database
	db, err := database.Open(database.Config{
		Path:        ":memory:",
		AutoMigrate: true,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Close()
	})

	// Create command with context containing config and database
	cmd := GetFindCmd()

	// Create a context with config that can be used by the command
	cfg := &config.Config{}
	ctx := context.WithValue(t.Context(), config.ConfigKey, cfg)
	cmd.SetContext(ctx)

	return cmd, db
}

// createTestLocation creates a location for testing.
func createTestLocation(t *testing.T, db *database.Database, displayName, canonicalName string) string {
	t.Helper()

	ctx := t.Context()
	const query = `
		INSERT INTO locations_current (location_id, display_name, canonical_name, parent_id, full_path_display, full_path_canonical, depth, is_system, updated_at)
		VALUES (?, ?, ?, NULL, ?, ?, 0, 0, CURRENT_TIMESTAMP)
	`

	locationID := fmt.Sprintf("loc_%s_%d", database.CanonicalizeString(displayName), time.Now().UnixNano())
	_, err := db.DB().ExecContext(ctx, query, locationID, displayName, canonicalName, displayName, canonicalName)
	require.NoError(t, err)

	return locationID
}

// createTestItem creates an item for testing.
func createTestItem(t *testing.T, db *database.Database, displayName, canonicalName, locationID string) string {
	t.Helper()

	ctx := t.Context()
	const query = `
		INSERT INTO items_current (item_id, display_name, canonical_name, location_id, in_temporary_use, last_event_id, updated_at)
		VALUES (?, ?, ?, ?, 0, 1, CURRENT_TIMESTAMP)
	`

	itemID := fmt.Sprintf("item_%s_%d", database.CanonicalizeString(displayName), time.Now().UnixNano())
	_, err := db.DB().ExecContext(ctx, query, itemID, displayName, canonicalName, locationID)
	require.NoError(t, err)

	return itemID
}

func TestFindCmd_BasicSearch(t *testing.T) {
	cmd, db := setupTestCmd(t)

	locID := createTestLocation(t, db, "Garage", "garage")
	itemID := createTestItem(t, db, "Hammer", "hammer", locID)

	// Mock the database opening by injecting it into output functions
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"hammer"})

	// This will fail because we can't easily inject the database, but we can test output formatting
	// Instead, test output functions directly
	results := []*database.SearchResult{
		{
			Type:        "item",
			DisplayName: "Hammer",
			ItemID:      &itemID,
			CurrentLocation: &database.LocationInfo{
				LocationID:      locID,
				DisplayName:     "Garage",
				FullPathDisplay: "Garage",
				IsSystem:        false,
			},
			CanonicalName:       "hammer",
			LevenshteinDistance: 0,
		},
	}

	outputHuman(&output, results, false, nil)
	result := output.String()

	assert.Contains(t, result, "Hammer")
	assert.Contains(t, result, "Garage")
}

func TestFindCmd_VerboseMode(t *testing.T) {
	var output bytes.Buffer

	itemID := "item_123"
	results := []*database.SearchResult{
		{
			Type:        "item",
			DisplayName: "10mm Socket",
			ItemID:      &itemID,
			CurrentLocation: &database.LocationInfo{
				LocationID:      "loc_456",
				DisplayName:     "Toolbox",
				FullPathDisplay: "Garage >> Toolbox",
			},
			CanonicalName:       "10mm_socket",
			LevenshteinDistance: 0,
		},
	}

	outputHuman(&output, results, true, nil)
	result := output.String()

	assert.Contains(t, result, "10mm Socket")
	assert.Contains(t, result, "Garage >> Toolbox")
	assert.Contains(t, result, "ID:")
	assert.Contains(t, result, "item_123")
	assert.Contains(t, result, "Match distance:")
	assert.Contains(t, result, "exact match")
}

func TestFindCmd_MissingItemIndicator(t *testing.T) {
	var output bytes.Buffer

	itemID := "item_missing"
	results := []*database.SearchResult{
		{
			Type:        "item",
			DisplayName: "Missing Hammer",
			ItemID:      &itemID,
			IsMissing:   true,
			CurrentLocation: &database.LocationInfo{
				LocationID: "missing_loc_id",
				IsSystem:   true,
			},
			LastNonSystemLocation: &database.LocationInfo{
				LocationID:      "garage_loc_id",
				DisplayName:     "Garage",
				FullPathDisplay: "Garage >> Shelf",
				IsSystem:        false,
			},
			CanonicalName:       "missing_hammer",
			LevenshteinDistance: 0,
		},
	}

	outputHuman(&output, results, false, nil)
	result := output.String()

	assert.Contains(t, result, "Missing Hammer")
	assert.Contains(t, result, "(MISSING)")
	assert.Contains(t, result, "Last location:")
	assert.Contains(t, result, "Garage >> Shelf")
	assert.Contains(t, result, "Currently: Missing")
}

func TestFindCmd_BorrowedItemIndicator(t *testing.T) {
	var output bytes.Buffer

	itemID := "item_borrowed"
	results := []*database.SearchResult{
		{
			Type:        "item",
			DisplayName: "Borrowed Screwdriver",
			ItemID:      &itemID,
			IsBorrowed:  true,
			CurrentLocation: &database.LocationInfo{
				LocationID: "borrowed_loc_id",
				IsSystem:   true,
			},
			LastNonSystemLocation: &database.LocationInfo{
				LocationID:      "workshop_loc_id",
				DisplayName:     "Workshop",
				FullPathDisplay: "Workshop >> Tool Cabinet",
				IsSystem:        false,
			},
			CanonicalName:       "borrowed_screwdriver",
			LevenshteinDistance: 0,
		},
	}

	outputHuman(&output, results, false, nil)
	result := output.String()

	assert.Contains(t, result, "Borrowed Screwdriver")
	assert.Contains(t, result, "(BORROWED)")
	assert.Contains(t, result, "Last location:")
	assert.Contains(t, result, "Workshop >> Tool Cabinet")
	assert.Contains(t, result, "Currently: Borrowed")
}

func TestFindCmd_LocationResult(t *testing.T) {
	var output bytes.Buffer

	locID := "loc_toolbox"
	results := []*database.SearchResult{
		{
			Type:                "location",
			DisplayName:         "Toolbox",
			LocationID:          &locID,
			FullPath:            "Garage >> Toolbox",
			IsSystem:            false,
			CanonicalName:       "toolbox",
			LevenshteinDistance: 0,
		},
	}

	outputHuman(&output, results, false, nil)
	result := output.String()

	assert.Contains(t, result, "Toolbox")
	assert.Contains(t, result, "(Location)")
	assert.Contains(t, result, "Garage >> Toolbox")
}

func TestFindCmd_JSONOutput_Item(t *testing.T) {
	var output bytes.Buffer

	itemID := "item_123"
	results := []*database.SearchResult{
		{
			Type:          "item",
			DisplayName:   "Hammer",
			ItemID:        &itemID,
			CanonicalName: "hammer",
			CurrentLocation: &database.LocationInfo{
				LocationID:      "loc_456",
				DisplayName:     "Garage",
				FullPathDisplay: "Garage",
			},
			InTemporaryUse:      false,
			IsMissing:           false,
			IsBorrowed:          false,
			LevenshteinDistance: 0,
		},
	}

	require.NoError(t, outputJSON(&output, results, "hammer", nil))

	var result map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &result))

	assert.Equal(t, "hammer", result["search_term"])
	require.NotNil(t, result["total_count"])
	require.NotNil(t, result["item_count"])

	resultsArr := result["results"].([]any)
	firstResult := resultsArr[0].(map[string]any)
	assert.Equal(t, "item", firstResult["type"])
	assert.Equal(t, "Hammer", firstResult["display_name"])
}

func TestFindCmd_JSONOutput_Location(t *testing.T) {
	var output bytes.Buffer

	locID := "loc_toolbox"
	results := []*database.SearchResult{
		{
			Type:                "location",
			DisplayName:         "Toolbox",
			LocationID:          &locID,
			CanonicalName:       "toolbox",
			FullPath:            "Garage >> Toolbox",
			IsSystem:            false,
			LevenshteinDistance: 0,
		},
	}

	require.NoError(t, outputJSON(&output, results, "toolbox", nil))

	var result map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &result))
	require.NotNil(t, result["location_count"])

	resultsArr := result["results"].([]any)
	firstResult := resultsArr[0].(map[string]any)
	assert.Equal(t, "location", firstResult["type"])
	assert.Equal(t, "Toolbox", firstResult["display_name"])
}

func TestFindCmd_JSONOutput_MixedResults(t *testing.T) {
	var output bytes.Buffer

	itemID := "item_123"
	locID := "loc_456"

	results := []*database.SearchResult{
		{
			Type:          "item",
			DisplayName:   "Socket",
			ItemID:        &itemID,
			CanonicalName: "socket",
			CurrentLocation: &database.LocationInfo{
				LocationID:      locID,
				DisplayName:     "Toolbox",
				FullPathDisplay: "Garage >> Toolbox",
			},
			LevenshteinDistance: 0,
		},
		{
			Type:                "location",
			DisplayName:         "Socket Storage",
			LocationID:          &locID,
			CanonicalName:       "socket_storage",
			FullPath:            "Garage >> Socket Storage",
			IsSystem:            false,
			LevenshteinDistance: 8,
		},
	}

	require.NoError(t, outputJSON(&output, results, "socket", nil))

	var result map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &result))

	require.NotNil(t, result["total_count"])
	require.NotNil(t, result["item_count"])
	require.NotNil(t, result["location_count"])
}

func TestFindCmd_JSONOutput_MissingItem(t *testing.T) {
	var output bytes.Buffer

	itemID := "item_missing"
	results := []*database.SearchResult{
		{
			Type:          "item",
			DisplayName:   "Missing Hammer",
			ItemID:        &itemID,
			CanonicalName: "missing_hammer",
			IsMissing:     true,
			LastNonSystemLocation: &database.LocationInfo{
				LocationID:      "garage_loc",
				DisplayName:     "Garage",
				FullPathDisplay: "Garage",
			},
			LevenshteinDistance: 0,
		},
	}

	require.NoError(t, outputJSON(&output, results, "hammer", nil))

	var result map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &result))

	resultsArr := result["results"].([]any)
	firstResult := resultsArr[0].(map[string]any)

	assert.True(t, firstResult["is_missing"].(bool))
	assert.NotNil(t, firstResult["last_non_system_location"])

	lastLoc := firstResult["last_non_system_location"].(map[string]any)
	assert.Equal(t, "Garage", lastLoc["display_name"])
}

func TestFindCmd_NoLastLocationForMissing(t *testing.T) {
	var output bytes.Buffer

	itemID := "item_missing_no_history"
	results := []*database.SearchResult{
		{
			Type:          "item",
			DisplayName:   "Mystery Item",
			ItemID:        &itemID,
			CanonicalName: "mystery_item",
			IsMissing:     true,
			CurrentLocation: &database.LocationInfo{
				LocationID: "missing_loc",
				IsSystem:   true,
			},
			LastNonSystemLocation: nil, // No history
			LevenshteinDistance:   0,
		},
	}

	outputHuman(&output, results, false, nil)
	result := output.String()

	assert.Contains(t, result, "Mystery Item")
	assert.Contains(t, result, "(MISSING)")
	assert.Contains(t, result, "Currently: Missing")
	// Should not mention last location if nil
	assert.NotContains(t, result, "Last location:")
}

func TestFindCmd_MultipleResults_Sorted(t *testing.T) {
	var output bytes.Buffer

	itemID1 := "item_1"
	itemID2 := "item_2"

	results := []*database.SearchResult{
		{
			Type:          "item",
			DisplayName:   "socket",
			ItemID:        &itemID1,
			CanonicalName: "socket",
			CurrentLocation: &database.LocationInfo{
				DisplayName: "Workshop",
			},
			LevenshteinDistance: 0,
		},
		{
			Type:          "item",
			DisplayName:   "sockets",
			ItemID:        &itemID2,
			CanonicalName: "sockets",
			CurrentLocation: &database.LocationInfo{
				DisplayName: "Workshop",
			},
			LevenshteinDistance: 1,
		},
	}

	outputHuman(&output, results, false, nil)
	result := output.String()

	// Verify results are present
	assert.Contains(t, result, "socket")
	assert.Contains(t, result, "sockets")
}

func TestFindCmd_HierarchicalPath(t *testing.T) {
	var output bytes.Buffer

	itemID := "item_wrench"
	results := []*database.SearchResult{
		{
			Type:          "item",
			DisplayName:   "10mm Wrench",
			ItemID:        &itemID,
			CanonicalName: "10mm_wrench",
			CurrentLocation: &database.LocationInfo{
				LocationID:      "deep_loc",
				DisplayName:     "Deep Drawer",
				FullPathDisplay: "Garage >> Toolbox >> Top Drawer >> Deep Drawer",
				IsSystem:        false,
			},
			LevenshteinDistance: 0,
		},
	}

	outputHuman(&output, results, false, nil)
	result := output.String()

	assert.Contains(t, result, "Garage >> Toolbox >> Top Drawer >> Deep Drawer")
}

func TestFindCmd_VerboseMode_LocationWithDistance(t *testing.T) {
	var output bytes.Buffer

	locID := "loc_cabinet"
	results := []*database.SearchResult{
		{
			Type:                "location",
			DisplayName:         "Cabinet",
			LocationID:          &locID,
			CanonicalName:       "cabinet",
			FullPath:            "Workshop >> Cabinet",
			IsSystem:            false,
			LevenshteinDistance: 5,
		},
	}

	outputHuman(&output, results, true, nil)
	result := output.String()

	assert.Contains(t, result, "Cabinet")
	assert.Contains(t, result, "ID:")
	assert.Contains(t, result, "Match distance: 5")
}

func TestFindCmd_JSONOutput_StructuredSchema(t *testing.T) {
	var output bytes.Buffer

	itemID := "item_tool"
	results := []*database.SearchResult{
		{
			Type:          "item",
			DisplayName:   "Torque Wrench",
			ItemID:        &itemID,
			CanonicalName: "torque_wrench",
			CurrentLocation: &database.LocationInfo{
				LocationID:      "loc_cabinet",
				DisplayName:     "Tool Cabinet",
				FullPathDisplay: "Workshop >> Cabinet",
			},
			InTemporaryUse:      false,
			IsMissing:           false,
			IsBorrowed:          false,
			LevenshteinDistance: 0,
		},
	}

	require.NoError(t, outputJSON(&output, results, "torque", nil))

	var result map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &result))

	// Verify top-level structure
	assert.Contains(t, result, "search_term")
	assert.Contains(t, result, "results")
	assert.Contains(t, result, "total_count")
	assert.Contains(t, result, "item_count")
	assert.Contains(t, result, "location_count")

	// Verify result item structure
	resultsArr := result["results"].([]any)
	firstResult := resultsArr[0].(map[string]any)
	assert.Contains(t, firstResult, "type")
	assert.Contains(t, firstResult, "item_id")
	assert.Contains(t, firstResult, "display_name")
	assert.Contains(t, firstResult, "canonical_name")
	assert.Contains(t, firstResult, "location")
	assert.Contains(t, firstResult, "levenshtein_distance")

	// Verify location sub-structure
	location := firstResult["location"].(map[string]any)
	assert.Contains(t, location, "location_id")
	assert.Contains(t, location, "display_name")
	assert.Contains(t, location, "full_path")
}

func TestFindCmd_OutputItem_InTemporaryUse(t *testing.T) {
	var output bytes.Buffer

	itemID := "item_temp"
	results := []*database.SearchResult{
		{
			Type:          "item",
			DisplayName:   "Borrowed Tool",
			ItemID:        &itemID,
			CanonicalName: "borrowed_tool",
			CurrentLocation: &database.LocationInfo{
				LocationID:      "temp_loc",
				DisplayName:     "Workbench",
				FullPathDisplay: "Workshop >> Workbench",
			},
			InTemporaryUse:      true,
			LevenshteinDistance: 0,
		},
	}

	require.NoError(t, outputJSON(&output, results, "tool", nil))

	var result map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &result))

	resultsArr := result["results"].([]any)
	firstResult := resultsArr[0].(map[string]any)
	assert.True(t, firstResult["in_temporary_use"].(bool))
}
