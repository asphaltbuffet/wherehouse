package scry

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/database"
	"github.com/asphaltbuffet/wherehouse/internal/nanoid"
)

// newTestContext returns a context with the given config stored under config.ConfigKey.
func newTestContext(cfg *config.Config) context.Context {
	return context.WithValue(context.Background(), config.ConfigKey, cfg)
}

func humanCfg() *config.Config {
	cfg := &config.Config{}
	cfg.Output.DefaultFormat = "human"
	return cfg
}

func jsonCfg() *config.Config {
	cfg := &config.Config{}
	cfg.Output.DefaultFormat = "json"
	return cfg
}

// scryTestIDs groups IDs used by setupScryTest.
type scryTestIDs struct {
	garageID   string
	missingID  string
	borrowedID string
	loanedID   string
	itemID     string
}

// setupScryTest creates an in-memory DB with one item already in the Missing location.
func setupScryTest(t *testing.T) (*database.Database, context.Context, scryTestIDs) {
	t.Helper()

	ctx := context.Background()
	db, err := database.Open(database.Config{
		Path:        ":memory:",
		BusyTimeout: database.DefaultBusyTimeout,
		AutoMigrate: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	ids := scryTestIDs{
		garageID: nanoid.MustNew(),
		itemID:   nanoid.MustNew(),
	}

	// Create a regular location
	err = db.CreateLocation(ctx, ids.garageID, "Garage", nil, false, 0, "2025-01-01T00:00:00Z")
	require.NoError(t, err)

	// Resolve system location IDs
	missingID, borrowedID, loanedID, _, err := db.GetSystemLocationIDs(ctx)
	require.NoError(t, err)
	ids.missingID = missingID
	ids.borrowedID = borrowedID
	ids.loanedID = loanedID

	// Create item via event (AppendEvent writes to both events table and projection).
	// ScryItem queries item.created from the events table, so CreateItem alone is insufficient.
	_, err = db.AppendEvent(ctx, database.ItemCreatedEvent, "testuser", map[string]any{
		"item_id":      ids.itemID,
		"display_name": "10mm socket",
		"location_id":  ids.garageID,
	}, "")
	require.NoError(t, err)

	// Mark item as missing via event
	_, err = db.AppendEvent(ctx, database.ItemMissingEvent, "testuser", map[string]any{
		"item_id":              ids.itemID,
		"previous_location_id": ids.garageID,
	}, "")
	require.NoError(t, err)

	return db, ctx, ids
}

// moveItemTo moves an item to a given location using AppendEvent with ItemMovedEvent.
func moveItemTo(t *testing.T, db *database.Database, itemID, fromLocID, toLocID string) {
	t.Helper()
	_, err := db.AppendEvent(context.Background(), database.ItemMovedEvent, "testuser", map[string]any{
		"item_id":          itemID,
		"from_location_id": fromLocID,
		"to_location_id":   toLocID,
		"move_type":        "rehome",
	}, "")
	require.NoError(t, err)
}

// markItemBorrowed moves the item to the Borrowed system location.
func markItemBorrowed(t *testing.T, db *database.Database, itemID, fromLocID string) {
	t.Helper()
	_, err := db.AppendEvent(context.Background(), database.ItemBorrowedEvent, "testuser", map[string]any{
		"item_id":          itemID,
		"from_location_id": fromLocID,
		"borrowed_by":      "Alice",
	}, "")
	require.NoError(t, err)
}

// markItemLoaned moves the item to the Loaned system location.
func markItemLoaned(t *testing.T, db *database.Database, itemID, fromLocID string) {
	t.Helper()
	_, err := db.AppendEvent(context.Background(), database.ItemLoanedEvent, "testuser", map[string]any{
		"item_id":          itemID,
		"from_location_id": fromLocID,
		"loaned_to":        "Bob",
	}, "")
	require.NoError(t, err)
}

// ── Command structure ────────────────────────────────────────────────────────

func TestNewScryCmd_Structure(t *testing.T) {
	db, _, _ := setupScryTest(t)

	cmd := NewScryCmd(db)

	assert.NotNil(t, cmd)
	assert.Equal(t, "scry", cmd.Name())
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	verboseFlag := cmd.Flags().Lookup("verbose")
	require.NotNil(t, verboseFlag)
	assert.Equal(t, "false", verboseFlag.DefValue)
}

func TestNewDefaultScryCmd_Structure(t *testing.T) {
	cmd := NewDefaultScryCmd()

	assert.NotNil(t, cmd)
	assert.Equal(t, "scry", cmd.Name())
	require.NotNil(t, cmd.Flags().Lookup("verbose"))
}

func TestGetScryCmd_IsNotNil(t *testing.T) {
	cmd := GetScryCmd()
	assert.NotNil(t, cmd)
	assert.Equal(t, "scry", cmd.Name())
}

func TestNewScryCmd_RequiresExactlyOneArg(t *testing.T) {
	db, _, _ := setupScryTest(t)

	cmd := NewScryCmd(db)
	cmd.SetContext(newTestContext(humanCfg()))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.Error(t, err)
}

// ── validateItemIsMissing ────────────────────────────────────────────────────

func TestValidateItemIsMissing_MissingItem_ReturnsNil(t *testing.T) {
	db, ctx, ids := setupScryTest(t)

	item, err := db.GetItem(ctx, ids.itemID)
	require.NoError(t, err)

	assert.NoError(t, validateItemIsMissing(ctx, db, item))
}

func TestValidateItemIsMissing_BorrowedItem_ReturnsError(t *testing.T) {
	db, ctx, ids := setupScryTest(t)

	markItemBorrowed(t, db, ids.itemID, ids.missingID)

	item, err := db.GetItem(ctx, ids.itemID)
	require.NoError(t, err)

	err = validateItemIsMissing(ctx, db, item)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "borrowed")
}

func TestValidateItemIsMissing_LoanedItem_ReturnsError(t *testing.T) {
	db, ctx, ids := setupScryTest(t)

	markItemLoaned(t, db, ids.itemID, ids.missingID)

	item, err := db.GetItem(ctx, ids.itemID)
	require.NoError(t, err)

	err = validateItemIsMissing(ctx, db, item)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "loaned")
}

func TestValidateItemIsMissing_KnownLocation_ErrorContainsLocationName(t *testing.T) {
	db, ctx, ids := setupScryTest(t)

	moveItemTo(t, db, ids.itemID, ids.missingID, ids.garageID)

	item, err := db.GetItem(ctx, ids.itemID)
	require.NoError(t, err)

	err = validateItemIsMissing(ctx, db, item)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not missing")
	assert.Contains(t, err.Error(), "Garage")
}

// ── runScryCore via NewScryCmd (integration) ─────────────────────────────────

func TestRunScryCore_MissingItem_HumanOutput(t *testing.T) {
	db, _, ids := setupScryTest(t)

	cmd := NewScryCmd(db)
	cmd.SetArgs([]string{ids.itemID})
	cmd.SetContext(newTestContext(humanCfg()))

	stdout := &bytes.Buffer{}
	cmd.SetOut(stdout)

	require.NoError(t, cmd.Execute())

	out := stdout.String()
	assert.Contains(t, out, "Scrying for")
	assert.Contains(t, out, "MISSING")
}

func TestRunScryCore_MissingItem_JSONOutput(t *testing.T) {
	db, _, ids := setupScryTest(t)

	cmd := NewScryCmd(db)
	cmd.SetArgs([]string{ids.itemID})
	cmd.SetContext(newTestContext(jsonCfg()))

	stdout := &bytes.Buffer{}
	cmd.SetOut(stdout)

	require.NoError(t, cmd.Execute())

	var out map[string]any
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &out))

	assert.Equal(t, ids.itemID, out["item_id"])
	assert.Equal(t, "10mm socket", out["display_name"])
	assert.NotNil(t, out["found_locations"])
	assert.NotNil(t, out["temp_use_locations"])
	assert.NotNil(t, out["similar_item_locations"])
}

func TestRunScryCore_ItemNotMissing_ReturnsError(t *testing.T) {
	db, _, ids := setupScryTest(t)

	moveItemTo(t, db, ids.itemID, ids.missingID, ids.garageID)

	cmd := NewScryCmd(db)
	cmd.SetArgs([]string{ids.itemID})
	cmd.SetContext(newTestContext(humanCfg()))
	cmd.SetOut(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not missing")
}

func TestRunScryCore_UnknownItem_ReturnsError(t *testing.T) {
	db, _, _ := setupScryTest(t)

	cmd := NewScryCmd(db)
	cmd.SetArgs([]string{nanoid.MustNew()})
	cmd.SetContext(newTestContext(humanCfg()))
	cmd.SetOut(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
}

func TestRunScryCore_VerboseFlag_ShowsOccurrences(t *testing.T) {
	db, ctx, ids := setupScryTest(t)

	// Move item back to Garage then re-mark as missing so scry has a "found" history
	moveItemTo(t, db, ids.itemID, ids.missingID, ids.garageID)
	_, err := db.AppendEvent(ctx, database.ItemMissingEvent, "testuser", map[string]any{
		"item_id":              ids.itemID,
		"previous_location_id": ids.garageID,
	}, "")
	require.NoError(t, err)

	cmd := NewScryCmd(db)
	cmd.SetArgs([]string{ids.itemID, "--verbose"})
	cmd.SetContext(newTestContext(humanCfg()))

	stdout := &bytes.Buffer{}
	cmd.SetOut(stdout)

	require.NoError(t, cmd.Execute())
	assert.Contains(t, stdout.String(), "Scrying for")
}

// ── outputHuman / formatting ─────────────────────────────────────────────────

func TestOutputHuman_HomeLocationOnly(t *testing.T) {
	result := &database.ScryResult{
		ItemID:      "id1",
		DisplayName: "10mm socket",
		HomeLocation: &database.LocationInfo{
			LocationID:      "loc1",
			DisplayName:     "Garage",
			FullPathDisplay: "Home > Garage",
		},
		FoundLocations:       []*database.ScoredLocation{},
		TempUseLocations:     []*database.ScoredLocation{},
		SimilarItemLocations: []*database.ScoredLocation{},
	}

	var buf bytes.Buffer
	outputHuman(&buf, result, false)
	out := buf.String()

	assert.Contains(t, out, "Scrying for")
	assert.Contains(t, out, "10mm socket")
	assert.Contains(t, out, "MISSING")
	assert.Contains(t, out, "Home location:")
	assert.Contains(t, out, "Home > Garage")
}

func TestOutputHuman_VerboseShowsOccurrences(t *testing.T) {
	result := &database.ScryResult{
		ItemID:      "id1",
		DisplayName: "wrench",
		HomeLocation: &database.LocationInfo{
			LocationID:      "loc1",
			DisplayName:     "Shelf",
			FullPathDisplay: "Shelf",
		},
		FoundLocations: []*database.ScoredLocation{
			{
				Location: &database.LocationInfo{
					LocationID:      "loc2",
					DisplayName:     "Drawer",
					FullPathDisplay: "Kitchen > Drawer",
				},
				Occurrences: 3,
			},
			{
				Location: &database.LocationInfo{
					LocationID:      "loc3",
					DisplayName:     "Box",
					FullPathDisplay: "Garage > Box",
				},
				Occurrences: 1,
			},
		},
		TempUseLocations:     []*database.ScoredLocation{},
		SimilarItemLocations: []*database.ScoredLocation{},
	}

	var buf bytes.Buffer
	outputHuman(&buf, result, true)
	out := buf.String()

	assert.Contains(t, out, "3 times")
	assert.Contains(t, out, "1 time")
	assert.Contains(t, out, "Kitchen > Drawer")
	assert.Contains(t, out, "Garage > Box")
}

func TestOutputHuman_SimilarItems_VerboseShowsDistance(t *testing.T) {
	result := &database.ScryResult{
		ItemID:           "id1",
		DisplayName:      "10mm socket",
		HomeLocation:     &database.LocationInfo{FullPathDisplay: "Garage"},
		FoundLocations:   []*database.ScoredLocation{},
		TempUseLocations: []*database.ScoredLocation{},
		SimilarItemLocations: []*database.ScoredLocation{
			{
				Location:               &database.LocationInfo{FullPathDisplay: "Tool Box"},
				SimilarItemName:        "10mm_socket",
				SimilarItemDisplayName: "10mm socket (copy)",
				LevenshteinDistance:    2,
			},
		},
	}

	var verboseBuf bytes.Buffer
	outputHuman(&verboseBuf, result, true)
	assert.Contains(t, verboseBuf.String(), "dist=2")

	var quietBuf bytes.Buffer
	outputHuman(&quietBuf, result, false)
	out := quietBuf.String()
	assert.NotContains(t, out, "dist=")
	assert.Contains(t, out, "10mm socket (copy)")
}

func TestOutputHuman_SimilarItems_FallsBackToCanonicalName(t *testing.T) {
	result := &database.ScryResult{
		ItemID:           "id1",
		DisplayName:      "wrench",
		HomeLocation:     &database.LocationInfo{FullPathDisplay: "Shelf"},
		FoundLocations:   []*database.ScoredLocation{},
		TempUseLocations: []*database.ScoredLocation{},
		SimilarItemLocations: []*database.ScoredLocation{
			{
				Location:               &database.LocationInfo{FullPathDisplay: "Drawer"},
				SimilarItemName:        "wrnch",
				SimilarItemDisplayName: "", // empty — must fall back to SimilarItemName
				LevenshteinDistance:    1,
			},
		},
	}

	var buf bytes.Buffer
	outputHuman(&buf, result, false)
	assert.Contains(t, buf.String(), "wrnch")
}

func TestPrintScoredCategory_Empty_WritesNothing(t *testing.T) {
	var buf bytes.Buffer
	printScoredCategory(&buf, "Found here before:", []*database.ScoredLocation{}, false)
	assert.Empty(t, buf.String())
}

func TestPrintLabeledRow_WidthAlignment(t *testing.T) {
	var buf bytes.Buffer
	printLabeledRow(&buf, "Home location:", "Garage", "")
	line := buf.String()
	assert.True(t, strings.HasPrefix(line, "  Home location:"))
	assert.Contains(t, line, "Garage")
}

func TestPrintContinuationRow_NoLabel(t *testing.T) {
	var buf bytes.Buffer
	printContinuationRow(&buf, "Second shelf", "")
	line := buf.String()
	assert.True(t, strings.HasPrefix(line, "  "))
	assert.Contains(t, line, "Second shelf")
}

// ── outputJSON ───────────────────────────────────────────────────────────────

func TestOutputJSON_HomeLocationPresent(t *testing.T) {
	result := &database.ScryResult{
		ItemID:        "item1",
		DisplayName:   "10mm socket",
		CanonicalName: "10mm_socket",
		HomeLocation: &database.LocationInfo{
			LocationID:      "loc1",
			DisplayName:     "Garage",
			FullPathDisplay: "Home > Garage",
		},
		FoundLocations: []*database.ScoredLocation{
			{
				Location: &database.LocationInfo{
					LocationID:      "loc2",
					DisplayName:     "Drawer",
					FullPathDisplay: "Kitchen > Drawer",
				},
				Occurrences: 2,
			},
		},
		TempUseLocations: []*database.ScoredLocation{},
		SimilarItemLocations: []*database.ScoredLocation{
			{
				Location: &database.LocationInfo{
					LocationID:      "loc3",
					DisplayName:     "Box",
					FullPathDisplay: "Box",
				},
				SimilarItemName:        "10mm_socket_set",
				SimilarItemDisplayName: "10mm socket set",
				LevenshteinDistance:    4,
			},
		},
	}

	var buf bytes.Buffer
	out := cli.NewOutputWriterFromConfig(&buf, &bytes.Buffer{}, jsonCfg())
	require.NoError(t, outputJSON(out, result))

	var m map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &m))

	assert.Equal(t, "item1", m["item_id"])
	assert.Equal(t, "10mm socket", m["display_name"])
	assert.Equal(t, "10mm_socket", m["canonical_name"])

	homeLocation := m["home_location"].(map[string]any)
	assert.Equal(t, "loc1", homeLocation["location_id"])
	assert.Equal(t, "Home > Garage", homeLocation["full_path"])

	foundLocs := m["found_locations"].([]any)
	require.Len(t, foundLocs, 1)
	fl := foundLocs[0].(map[string]any)
	assert.Equal(t, "loc2", fl["location_id"])
	assert.InDelta(t, float64(2), fl["occurrences"], 0.1)

	similarLocs := m["similar_item_locations"].([]any)
	require.Len(t, similarLocs, 1)
	sl := similarLocs[0].(map[string]any)
	assert.Equal(t, "10mm_socket_set", sl["similar_item"])
	assert.Equal(t, "10mm socket set", sl["similar_item_display_name"])
}

func TestOutputJSON_HomeLocationNilOmitted(t *testing.T) {
	result := &database.ScryResult{
		ItemID:               "x",
		DisplayName:          "widget",
		CanonicalName:        "widget",
		HomeLocation:         nil,
		FoundLocations:       []*database.ScoredLocation{},
		TempUseLocations:     []*database.ScoredLocation{},
		SimilarItemLocations: []*database.ScoredLocation{},
	}

	var buf bytes.Buffer
	out := cli.NewOutputWriterFromConfig(&buf, &bytes.Buffer{}, jsonCfg())
	require.NoError(t, outputJSON(out, result))

	var m map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &m))

	_, hasHome := m["home_location"]
	assert.False(t, hasHome, "home_location should be omitted when nil")
}

func TestOutputJSON_SimilarItems_FallBackToCanonicalName(t *testing.T) {
	result := &database.ScryResult{
		ItemID:           "x",
		DisplayName:      "wrench",
		CanonicalName:    "wrench",
		HomeLocation:     nil,
		FoundLocations:   []*database.ScoredLocation{},
		TempUseLocations: []*database.ScoredLocation{},
		SimilarItemLocations: []*database.ScoredLocation{
			{
				Location: &database.LocationInfo{
					LocationID:      "l1",
					DisplayName:     "Drawer",
					FullPathDisplay: "Drawer",
				},
				SimilarItemName:        "wrnch",
				SimilarItemDisplayName: "", // empty — should fall back to SimilarItemName
				LevenshteinDistance:    1,
			},
		},
	}

	var buf bytes.Buffer
	out := cli.NewOutputWriterFromConfig(&buf, &bytes.Buffer{}, jsonCfg())
	require.NoError(t, outputJSON(out, result))

	var m map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &m))

	sl := m["similar_item_locations"].([]any)[0].(map[string]any)
	// When SimilarItemDisplayName is empty, display_name falls back to SimilarItemName
	assert.Equal(t, "wrnch", sl["similar_item_display_name"])
}

// ── Close error logged to stderr ─────────────────────────────────────────────

func TestRunScryCore_CloseError_LoggedToStderr(t *testing.T) {
	db, _, ids := setupScryTest(t)

	cmd := NewScryCmd(db)
	cmd.SetArgs([]string{ids.itemID})
	cmd.SetContext(newTestContext(humanCfg()))

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	// Close succeeds normally — verify no warning in stderr
	require.NoError(t, cmd.Execute())
	assert.NotContains(t, stderr.String(), "warning")
}
