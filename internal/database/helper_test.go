package database

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/require"
)

// Test Constants - Fixed 10-character alphanumeric IDs for reproducible tests.
const (
	// TestLocationWorkshop is a root-level test location ID.
	TestLocationWorkshop = "tst0loc001"
	// TestLocationStorage is a root-level test location ID.
	TestLocationStorage = "tst0loc002"

	// TestLocationToolbox is a Workshop child location ID.
	TestLocationToolbox = "tst0loc003"
	// TestLocationWorkbench is a Workshop child location ID.
	TestLocationWorkbench = "tst0loc004"

	// TestLocationShelves is a Storage child location ID.
	TestLocationShelves = "tst0loc005"
	// TestLocationBinA is a Shelves child location ID.
	TestLocationBinA = "tst0loc006"
	// TestLocationBinB is a Shelves child location ID.
	TestLocationBinB = "tst0loc007"

	// TestItem10mmSocket is a test item ID.
	TestItem10mmSocket = "tst0itm001"
	// TestItemScrewdriverSet is a test item ID.
	TestItemScrewdriverSet = "tst0itm002"
	// TestItemHammer is a test item ID.
	TestItemHammer = "tst0itm003"
	// TestItemDrillBits is a test item ID.
	TestItemDrillBits = "tst0itm004"
	// TestItemSandpaper is a test item ID.
	TestItemSandpaper = "tst0itm005"
	// TestItemMissingWrench is a test item ID.
	TestItemMissingWrench = "tst0itm006"
	// TestItemBorrowedSaw is a test item ID.
	TestItemBorrowedSaw = "tst0itm007"

	// TestProjectDeck is a test project ID.
	TestProjectDeck = "test-project-deck"
	// TestProjectShelving is a test project ID.
	TestProjectShelving = "test-project-shelving"

	// TestActorUser is the test user ID for event attribution.
	TestActorUser = "test-user"

	// Test timing and depth constants.
	testWaitTimeoutSeconds   = 5
	testWaitPollMilliseconds = 10
	testExpectedDepthLevel2  = 2
)

// NewTestDB creates a new test database with migrations applied.
// It returns a Database instance connected to a temporary SQLite file.
// The database is automatically cleaned up when the test completes.
func NewTestDB(t *testing.T) *Database {
	t.Helper()

	tmpDir := t.TempDir()
	// Create temporary database file
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open database with migrations
	cfg := DefaultConfig()
	cfg.Path = dbPath
	cfg.AutoMigrate = true

	db, err := Open(cfg)
	require.NoError(t, err, "failed to open test database")

	// Clean up on test completion
	t.Cleanup(func() {
		if err = db.Close(); err != nil {
			t.Logf("failed to close test database: %v", err)
		}
	})

	return db
}

// NewTestDBWithSeed creates a test database with seed data already populated.
// This is useful for testing workflows that require pre-existing data.
func NewTestDBWithSeed(t *testing.T) *Database {
	t.Helper()

	db := NewTestDB(t)
	ctx := t.Context()

	// Seed test data
	err := SeedTestData(ctx, db)
	require.NoError(t, err, "failed to seed test data")

	return db
}

// SeedTestData populates the database with test data using events.
// This ensures data is created through the event system for authenticity.
//
// Created data:
//   - 8 Locations: Workshop, Storage (roots), plus Toolbox, Workbench (Workshop children),
//     Shelves, Bin A, Bin B (Storage hierarchy)
//   - 7 Items: 10mm Socket, Screwdriver Set, Hammer, Drill Bits, Sandpaper, Missing Wrench, Borrowed Saw
//   - 2 Projects: test-project-deck (active), test-project-shelving (completed).
//
//nolint:gocognit // Test data seeding is inherently sequential and complex
func SeedTestData(ctx context.Context, db *Database) error {
	// Insert all events first (without processing)

	// Create root locations via events
	if _, err := db.insertEvent(ctx, LocationCreatedEvent, TestActorUser, map[string]any{
		"location_id":  TestLocationWorkshop,
		"display_name": "Workshop",
		"parent_id":    nil,
	}, ""); err != nil {
		return err
	}

	if _, err := db.insertEvent(ctx, LocationCreatedEvent, TestActorUser, map[string]any{
		"location_id":  TestLocationStorage,
		"display_name": "Storage",
		"parent_id":    nil,
	}, ""); err != nil {
		return err
	}

	// Create Workshop children
	workshopPtr := TestLocationWorkshop
	if _, err := db.insertEvent(ctx, LocationCreatedEvent, TestActorUser, map[string]any{
		"location_id":  TestLocationToolbox,
		"display_name": "Toolbox",
		"parent_id":    workshopPtr,
	}, ""); err != nil {
		return err
	}

	if _, err := db.insertEvent(ctx, LocationCreatedEvent, TestActorUser, map[string]any{
		"location_id":  TestLocationWorkbench,
		"display_name": "Workbench",
		"parent_id":    workshopPtr,
	}, ""); err != nil {
		return err
	}

	// Create Storage hierarchy
	storagePtr := TestLocationStorage
	if _, err := db.insertEvent(ctx, LocationCreatedEvent, TestActorUser, map[string]any{
		"location_id":  TestLocationShelves,
		"display_name": "Shelves",
		"parent_id":    storagePtr,
	}, ""); err != nil {
		return err
	}

	shelvesPtr := TestLocationShelves
	if _, err := db.insertEvent(ctx, LocationCreatedEvent, TestActorUser, map[string]any{
		"location_id":  TestLocationBinA,
		"display_name": "Bin A",
		"parent_id":    shelvesPtr,
	}, ""); err != nil {
		return err
	}

	if _, err := db.insertEvent(ctx, LocationCreatedEvent, TestActorUser, map[string]any{
		"location_id":  TestLocationBinB,
		"display_name": "Bin B",
		"parent_id":    shelvesPtr,
	}, ""); err != nil {
		return err
	}

	// Create items in locations
	if _, err := db.insertEvent(ctx, ItemCreatedEvent, TestActorUser, map[string]any{
		"item_id":        TestItem10mmSocket,
		"display_name":   "10mm Socket",
		"canonical_name": CanonicalizeString("10mm Socket"),
		"location_id":    TestLocationToolbox,
	}, ""); err != nil {
		return err
	}

	if _, err := db.insertEvent(ctx, ItemCreatedEvent, TestActorUser, map[string]any{
		"item_id":        TestItemScrewdriverSet,
		"display_name":   "Screwdriver Set",
		"canonical_name": CanonicalizeString("Screwdriver Set"),
		"location_id":    TestLocationToolbox,
	}, ""); err != nil {
		return err
	}

	if _, err := db.insertEvent(ctx, ItemCreatedEvent, TestActorUser, map[string]any{
		"item_id":        TestItemHammer,
		"display_name":   "Hammer",
		"canonical_name": CanonicalizeString("Hammer"),
		"location_id":    TestLocationWorkbench,
	}, ""); err != nil {
		return err
	}

	if _, err := db.insertEvent(ctx, ItemCreatedEvent, TestActorUser, map[string]any{
		"item_id":        TestItemDrillBits,
		"display_name":   "Drill Bits",
		"canonical_name": CanonicalizeString("Drill Bits"),
		"location_id":    TestLocationBinA,
	}, ""); err != nil {
		return err
	}

	if _, err := db.insertEvent(ctx, ItemCreatedEvent, TestActorUser, map[string]any{
		"item_id":        TestItemSandpaper,
		"display_name":   "Sandpaper",
		"canonical_name": CanonicalizeString("Sandpaper"),
		"location_id":    TestLocationBinB,
	}, ""); err != nil {
		return err
	}

	// Get system locations for items that should be in Missing/Borrowed
	missingLoc, err := db.GetLocationByCanonicalName(ctx, "missing")
	if err != nil {
		return err
	}
	borrowedLoc, err := db.GetLocationByCanonicalName(ctx, "borrowed")
	if err != nil {
		return err
	}

	// Create items in system locations
	if _, err = db.insertEvent(ctx, ItemCreatedEvent, TestActorUser, map[string]any{
		"item_id":        TestItemMissingWrench,
		"display_name":   "Missing Wrench",
		"canonical_name": CanonicalizeString("Missing Wrench"),
		"location_id":    missingLoc.LocationID,
	}, ""); err != nil {
		return err
	}

	if _, err = db.insertEvent(ctx, ItemCreatedEvent, TestActorUser, map[string]any{
		"item_id":        TestItemBorrowedSaw,
		"display_name":   "Borrowed Saw",
		"canonical_name": CanonicalizeString("Borrowed Saw"),
		"location_id":    borrowedLoc.LocationID,
	}, ""); err != nil {
		return err
	}

	// Create projects via events
	if _, err = db.insertEvent(ctx, ProjectCreatedEvent, TestActorUser, map[string]any{
		"project_id": TestProjectDeck,
		"status":     "active",
	}, ""); err != nil {
		return err
	}

	if _, err = db.insertEvent(ctx, ProjectCreatedEvent, TestActorUser, map[string]any{
		"project_id": TestProjectShelving,
		"status":     "completed",
	}, ""); err != nil {
		return err
	}

	// Associate Drill Bits with test-project-deck (via item.moved event with project action)
	if _, err = db.insertEvent(ctx, ItemMovedEvent, TestActorUser, map[string]any{
		"item_id":          TestItemDrillBits,
		"from_location_id": TestLocationBinA,
		"to_location_id":   TestLocationBinA, // Same location, just setting project
		"move_type":        "rehome",
		"project_action":   "set",
		"project_id":       TestProjectDeck,
	}, ""); err != nil {
		return err
	}

	// Associate Sandpaper with test-project-deck (via item.moved event with project action)
	if _, err = db.insertEvent(ctx, ItemMovedEvent, TestActorUser, map[string]any{
		"item_id":          TestItemSandpaper,
		"from_location_id": TestLocationBinB,
		"to_location_id":   TestLocationBinB, // Same location, just setting project
		"move_type":        "rehome",
		"project_action":   "set",
		"project_id":       TestProjectDeck,
	}, ""); err != nil {
		return err
	}

	// Now process all events in order to populate projections
	// We do this outside a transaction so that computeLocationPath can query parent locations
	events, err := db.GetAllEvents(ctx)
	if err != nil {
		return err
	}

	for _, event := range events {
		if err = db.ProcessEvent(ctx, event); err != nil {
			return fmt.Errorf("failed to process event %d (%s): %w", event.EventID, event.EventType, err)
		}
	}

	return nil
}

// insertEvent creates a new event in the events table without updating projections.
// This is a low-level primitive for replay and seed scenarios where events are
// batch-inserted before being processed.
func (d *Database) insertEvent(
	ctx context.Context,
	eventType EventType,
	actorUserID string,
	payload any,
	note string,
) (int64, error) {
	// Marshal payload to JSON
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Extract entity IDs from payload for indexing
	var payloadMap map[string]any
	if unmarshalErr := json.Unmarshal(payloadJSON, &payloadMap); unmarshalErr != nil {
		return 0, fmt.Errorf("failed to unmarshal payload for ID extraction: %w", unmarshalErr)
	}

	var itemID, locationID, projectID *string
	if id, ok := payloadMap["item_id"].(string); ok && id != "" {
		itemID = &id
	}
	if id, ok := payloadMap["location_id"].(string); ok && id != "" {
		locationID = &id
	}
	if id, ok := payloadMap["project_id"].(string); ok && id != "" {
		projectID = &id
	}

	// Generate timestamp in RFC3339 format with Z
	timestamp := time.Now().UTC().Format(time.RFC3339)

	// Prepare note (NULL if empty)
	var notePtr *string
	if note != "" {
		notePtr = &note
	}

	// Insert event
	const query = `
		INSERT INTO events (
			event_type,
			timestamp_utc,
			actor_user_id,
			payload,
			note,
			item_id,
			location_id,
			project_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := d.db.ExecContext(ctx, query,
		eventType,
		timestamp,
		actorUserID,
		string(payloadJSON),
		notePtr,
		itemID,
		locationID,
		projectID,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert event: %w", err)
	}

	eventID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get event ID: %w", err)
	}

	return eventID, nil
}

// AssertTestDataIntegrity verifies that all test seed data exists and is in expected state.
// This is useful for validating that test data survived migrations or rebuilds.
func AssertTestDataIntegrity(t *testing.T, db *Database) {
	t.Helper()

	ctx := t.Context()

	// Verify all test locations exist
	locations := map[string]struct{}{
		TestLocationWorkshop:  {},
		TestLocationStorage:   {},
		TestLocationToolbox:   {},
		TestLocationWorkbench: {},
		TestLocationShelves:   {},
		TestLocationBinA:      {},
		TestLocationBinB:      {},
	}

	for locID := range locations {
		loc, err := db.GetLocation(ctx, locID)
		require.NoError(t, err, "location %s should exist", locID)
		require.NotNil(t, loc)
	}

	// Verify all test items exist
	items := map[string]string{
		TestItem10mmSocket:     TestLocationToolbox,
		TestItemScrewdriverSet: TestLocationToolbox,
		TestItemHammer:         TestLocationWorkbench,
		TestItemDrillBits:      TestLocationBinA,
		TestItemSandpaper:      TestLocationBinB,
	}

	for itemID, expectedLocID := range items {
		item, err := db.GetItem(ctx, itemID)
		require.NoError(t, err, "item %s should exist", itemID)
		require.NotNil(t, item)
		require.Equal(t, expectedLocID, item.LocationID, "item %s should be in location %s", itemID, expectedLocID)
	}

	// Verify items in system locations
	missingLoc, err := db.GetLocationByCanonicalName(ctx, "missing")
	require.NoError(t, err)
	require.NotNil(t, missingLoc)

	borrowedLoc, err := db.GetLocationByCanonicalName(ctx, "borrowed")
	require.NoError(t, err)
	require.NotNil(t, borrowedLoc)

	missingItem, err := db.GetItem(ctx, TestItemMissingWrench)
	require.NoError(t, err)
	require.NotNil(t, missingItem)
	require.Equal(t, missingLoc.LocationID, missingItem.LocationID)

	borrowedItem, err := db.GetItem(ctx, TestItemBorrowedSaw)
	require.NoError(t, err)
	require.NotNil(t, borrowedItem)
	require.Equal(t, borrowedLoc.LocationID, borrowedItem.LocationID)

	// Verify projects exist
	deckProj, err := db.GetProject(ctx, TestProjectDeck)
	require.NoError(t, err)
	require.NotNil(t, deckProj)
	require.Equal(t, "active", deckProj.Status)

	shelvingProj, err := db.GetProject(ctx, TestProjectShelving)
	require.NoError(t, err)
	require.NotNil(t, shelvingProj)
	require.Equal(t, "completed", shelvingProj.Status)

	// Verify project associations
	drillItem, err := db.GetItem(ctx, TestItemDrillBits)
	require.NoError(t, err)
	require.NotNil(t, drillItem)
	require.NotNil(t, drillItem.ProjectID, "drill bits should have project association")
	require.Equal(t, TestProjectDeck, *drillItem.ProjectID)

	sandpaperItem, err := db.GetItem(ctx, TestItemSandpaper)
	require.NoError(t, err)
	require.NotNil(t, sandpaperItem)
	require.NotNil(t, sandpaperItem.ProjectID, "sandpaper should have project association")
	require.Equal(t, TestProjectDeck, *sandpaperItem.ProjectID)

	// Verify location hierarchy
	toolboxLoc, err := db.GetLocation(ctx, TestLocationToolbox)
	require.NoError(t, err)
	require.NotNil(t, toolboxLoc)
	require.NotNil(t, toolboxLoc.ParentID)
	require.Equal(t, TestLocationWorkshop, *toolboxLoc.ParentID)
	require.Equal(t, 1, toolboxLoc.Depth)

	binALoc, err := db.GetLocation(ctx, TestLocationBinA)
	require.NoError(t, err)
	require.NotNil(t, binALoc)
	require.NotNil(t, binALoc.ParentID)
	require.Equal(t, TestLocationShelves, *binALoc.ParentID)
	require.Equal(t, testExpectedDepthLevel2, binALoc.Depth)

	// Verify full paths
	require.Equal(t, "Workshop >> Toolbox", toolboxLoc.FullPathDisplay)
	require.Equal(t, "workshop:toolbox", toolboxLoc.FullPathCanonical)

	require.Equal(t, "Storage >> Shelves >> Bin A", binALoc.FullPathDisplay)
	require.Equal(t, "storage:shelves:bin_a", binALoc.FullPathCanonical)
}

// WaitForEventID is a test helper that waits for a specific event to be created.
// Useful for testing concurrent scenarios. Times out after 5 seconds.
func (d *Database) WaitForEventID(ctx context.Context, targetEventID int64) error {
	deadline := time.Now().Add(testWaitTimeoutSeconds * time.Second)
	for {
		var count int
		err := d.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM events WHERE event_id >= ?", targetEventID).Scan(&count)
		if err != nil {
			return err
		}

		if count > 0 {
			return nil
		}

		if time.Now().After(deadline) {
			return ErrEventNotFound
		}

		time.Sleep(testWaitPollMilliseconds * time.Millisecond)
	}
}
