package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBasicWorkflow tests a simple create-read workflow without seeding.
func TestBasicWorkflow(t *testing.T) {
	db := NewTestDB(t)
	ctx := t.Context()

	t.Run("create and retrieve location", func(t *testing.T) {
		// Create a location directly in projections
		locID := "test-loc-1"
		require.NoError(t, db.CreateLocation(ctx, locID, "Test Location", nil, false, 1, "2026-02-21T10:00:00Z"))

		// Retrieve it
		loc, err := db.GetLocation(ctx, locID)
		require.NoError(t, err)
		require.NotNil(t, loc)
		assert.Equal(t, "Test Location", loc.DisplayName)
		assert.Equal(t, "test_location", loc.CanonicalName)
		assert.Nil(t, loc.ParentID)
		assert.Zero(t, loc.Depth)
	})
}

// TestLocationHierarchy tests the location hierarchy workflow.
func TestLocationHierarchy(t *testing.T) {
	db := NewTestDB(t)
	ctx := t.Context()

	t.Run("root locations have no parent", func(t *testing.T) {
		// Create a root location
		locID := "root-loc"
		err := db.CreateLocation(ctx, locID, "Root", nil, false, 1, "2026-02-21T10:00:00Z")
		require.NoError(t, err)

		root, err := db.GetLocation(ctx, locID)
		require.NoError(t, err)
		require.NotNil(t, root)
		assert.Nil(t, root.ParentID)
		assert.Equal(t, 0, root.Depth)
		assert.Equal(t, "Root", root.FullPathDisplay)
		assert.Equal(t, "root", root.FullPathCanonical)
	})

	t.Run("child locations have correct parent", func(t *testing.T) {
		// Create parent first
		parentID := "parent-loc"
		err := db.CreateLocation(ctx, parentID, "Parent", nil, false, 1, "2026-02-21T10:00:00Z")
		require.NoError(t, err)

		// Create child
		childID := "child-loc"
		err = db.CreateLocation(ctx, childID, "Child", &parentID, false, 2, "2026-02-21T10:00:00Z")
		require.NoError(t, err)

		child, err := db.GetLocation(ctx, childID)
		require.NoError(t, err)
		require.NotNil(t, child)
		require.NotNil(t, child.ParentID)
		assert.Equal(t, parentID, *child.ParentID)
		assert.Equal(t, 1, child.Depth)
		assert.Equal(t, "Parent >> Child", child.FullPathDisplay)
		assert.Equal(t, "parent:child", child.FullPathCanonical)
	})

	t.Run("child location retrieval", func(t *testing.T) {
		// Create parent
		parentID := "parent-loc-2"
		err := db.CreateLocation(ctx, parentID, "Parent 2", nil, false, 3, "2026-02-21T10:00:00Z")
		require.NoError(t, err)

		// Create children
		child1ID := "child-1"
		err = db.CreateLocation(ctx, child1ID, "Child 1", &parentID, false, 4, "2026-02-21T10:00:00Z")
		require.NoError(t, err)

		child2ID := "child-2"
		err = db.CreateLocation(ctx, child2ID, "Child 2", &parentID, false, 5, "2026-02-21T10:00:00Z")
		require.NoError(t, err)

		children, err := db.GetLocationChildren(ctx, parentID)
		require.NoError(t, err)
		require.Len(t, children, 2, "Parent should have 2 children")

		childIDs := make(map[string]bool)
		for _, child := range children {
			childIDs[child.LocationID] = true
		}
		assert.True(t, childIDs[child1ID])
		assert.True(t, childIDs[child2ID])
	})

	t.Run("system locations from migration are marked", func(t *testing.T) {
		// After migration, system locations should exist
		missing, err := db.GetLocationByCanonicalName(ctx, "missing")
		require.NoError(t, err)
		require.NotNil(t, missing)
		assert.True(t, missing.IsSystem)

		borrowed, err := db.GetLocationByCanonicalName(ctx, "borrowed")
		require.NoError(t, err)
		require.NotNil(t, borrowed)
		assert.True(t, borrowed.IsSystem)
	})
}

// TestCreateLocationMoveItemWorkflow tests a complete workflow.
func TestCreateLocationMoveItemWorkflow(t *testing.T) {
	db := NewTestDB(t)
	ctx := t.Context()

	t.Run("create location → create item → move item", func(t *testing.T) {
		// Step 1: Create a new location via event
		locPayload := map[string]any{
			"location_id":  "new-loc-1",
			"display_name": "New Location",
			"parent_id":    nil,
		}
		_, err := db.insertEvent(ctx, LocationCreatedEvent, "test-user", locPayload, "")
		require.NoError(t, err)

		// Apply event to projection
		events, err := db.GetEventsByType(ctx, LocationCreatedEvent)
		require.NoError(t, err)
		require.NotEmpty(t, events)
		require.NoError(t, db.ProcessEvent(ctx, events[len(events)-1]))

		// Verify location created
		loc, err := db.GetLocation(ctx, "new-loc-1")
		require.NoError(t, err)
		require.NotNil(t, loc)
		assert.Equal(t, "New Location", loc.DisplayName)

		// Step 2: Create item in the new location
		itemPayload := map[string]any{
			"item_id":        "new-item-1",
			"display_name":   "Test Item",
			"canonical_name": "test_item",
			"location_id":    "new-loc-1",
		}
		_, err = db.insertEvent(ctx, ItemCreatedEvent, "test-user", itemPayload, "")
		require.NoError(t, err)

		// Apply event to projection
		events, err = db.GetEventsByType(ctx, ItemCreatedEvent)
		require.NoError(t, err)
		require.NotEmpty(t, events)
		require.NoError(t, db.ProcessEvent(ctx, events[len(events)-1]))

		// Verify item created
		item, err := db.GetItem(ctx, "new-item-1")
		require.NoError(t, err)
		require.NotNil(t, item)
		assert.Equal(t, "new-loc-1", item.LocationID)

		// Step 3: Move item via event
		movePayload := map[string]any{
			"item_id":          "new-item-1",
			"from_location_id": "new-loc-1",
			"to_location_id":   "new-loc-1", // Same location
			"move_type":        "rehome",
			"project_action":   "clear",
		}
		_, err = db.insertEvent(ctx, ItemMovedEvent, "test-user", movePayload, "")
		require.NoError(t, err)

		// Apply event to projection
		events, err = db.GetEventsByType(ctx, ItemMovedEvent)
		require.NoError(t, err)
		require.NotEmpty(t, events)
		require.NoError(t, db.ProcessEvent(ctx, events[len(events)-1]))

		// Verify item still in location (no actual move)
		item, err = db.GetItem(ctx, "new-item-1")
		require.NoError(t, err)
		require.NotNil(t, item)
		assert.Equal(t, "new-loc-1", item.LocationID)
	})
}

// TestProjectAssociationWorkflow tests project association workflow.
func TestProjectAssociationWorkflow(t *testing.T) {
	db := NewTestDB(t)
	ctx := t.Context()

	t.Run("create projects with different statuses", func(t *testing.T) {
		// Create active project
		require.NoError(t, db.CreateProject(ctx, "proj-active", "active", "2026-02-21T10:00:00Z"))

		proj, err := db.GetProject(ctx, "proj-active")
		require.NoError(t, err)
		require.NotNil(t, proj)
		assert.Equal(t, "active", proj.Status)

		// Create completed project
		require.NoError(t, db.CreateProject(ctx, "proj-complete", "completed", "2026-02-21T10:00:00Z"))

		proj, err = db.GetProject(ctx, "proj-complete")
		require.NoError(t, err)
		require.NotNil(t, proj)
		assert.Equal(t, "completed", proj.Status)
	})

	t.Run("items can have project association", func(t *testing.T) {
		// Create location
		locID := "proj-test-loc"
		require.NoError(t, db.CreateLocation(ctx, locID, "Proj Test", nil, false, 1, "2026-02-21T10:00:00Z"))

		// Create item
		itemID := "proj-test-item"
		require.NoError(t, db.CreateItem(ctx, itemID, "Test Item", locID, 2, "2026-02-21T10:00:00Z"))

		// Create project
		projID := "proj-test"
		require.NoError(t, db.CreateProject(ctx, projID, "active", "2026-02-21T10:00:00Z"))

		// Associate item with project
		updates := map[string]any{
			"project_id": projID,
		}
		require.NoError(t, db.UpdateItem(ctx, itemID, updates, 3, "2026-02-21T10:00:00Z"))

		// Verify association
		item, err := db.GetItem(ctx, itemID)
		require.NoError(t, err)
		require.NotNil(t, item.ProjectID)
		assert.Equal(t, projID, *item.ProjectID)
	})
}

// TestEventLog tests event logging and retrieval.
func TestEventLog(t *testing.T) {
	db := NewTestDB(t)
	ctx := t.Context()

	t.Run("events are recorded in order", func(t *testing.T) {
		// Create location event
		eventID1, err := db.insertEvent(ctx, LocationCreatedEvent, "test-user", map[string]any{
			"location_id":  "event-loc-1",
			"display_name": "Event Location 1",
		}, "first location")
		require.NoError(t, err)

		// Create item event
		eventID2, err := db.insertEvent(ctx, ItemCreatedEvent, "test-user", map[string]any{
			"item_id":      "event-item-1",
			"display_name": "Event Item 1",
		}, "first item")
		require.NoError(t, err)

		// Verify events are in order
		assert.Greater(t, eventID2, eventID1)

		// Retrieve events
		events, err := db.GetAllEvents(ctx)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(events), 2)

		// Verify events are ordered by ID
		for i := 1; i < len(events); i++ {
			assert.Greater(t, events[i].EventID, events[i-1].EventID)
		}
	})

	t.Run("get events by type", func(t *testing.T) {
		// Insert location events
		_, err := db.insertEvent(ctx, LocationCreatedEvent, "test-user", map[string]any{
			"location_id":  "loc-type-1",
			"display_name": "Loc Type 1",
		}, "")
		require.NoError(t, err)

		_, err = db.insertEvent(ctx, LocationCreatedEvent, "test-user", map[string]any{
			"location_id":  "loc-type-2",
			"display_name": "Loc Type 2",
		}, "")
		require.NoError(t, err)

		// Insert item event
		_, err = db.insertEvent(ctx, ItemCreatedEvent, "test-user", map[string]any{
			"item_id":      "item-type-1",
			"display_name": "Item Type 1",
		}, "")
		require.NoError(t, err)

		// Get location events
		locEvents, err := db.GetEventsByType(ctx, LocationCreatedEvent)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(locEvents), 2)

		// Verify all are location events
		for _, evt := range locEvents {
			assert.Equal(t, LocationCreatedEvent, evt.EventType)
		}
	})
}

// TestValidationErrorScenarios tests error handling for validation failures.
func TestValidationErrorScenarios(t *testing.T) {
	db := NewTestDB(t)
	ctx := t.Context()

	t.Run("cannot create item with non-existent location", func(t *testing.T) {
		// Try to create item in non-existent location directly
		assert.Error(
			t,
			db.CreateItem(ctx, "bad-item-1", "Bad Item", "non-existent-location", 999, "2026-02-21T10:00:00Z"),
			"should fail due to foreign key constraint",
		)
	})

	t.Run("deleted item returns not found", func(t *testing.T) {
		// Create location
		locID := "delete-test-loc"
		require.NoError(t, db.CreateLocation(ctx, locID, "Delete Test", nil, false, 1, "2026-02-21T10:00:00Z"))

		// Create item
		itemID := "delete-test-item"
		require.NoError(t, db.CreateItem(ctx, itemID, "Delete Test Item", locID, 2, "2026-02-21T10:00:00Z"))

		// Verify item exists
		item, err := db.GetItem(ctx, itemID)
		require.NoError(t, err)
		require.NotNil(t, item)

		// Delete item
		require.NoError(t, db.DeleteItem(ctx, itemID))

		// Verify item no longer exists
		_, err = db.GetItem(ctx, itemID)
		assert.Error(t, err, "deleted item should not be found")
	})
}

// TestEventOrdering tests that events are processed in order.
func TestEventOrdering(t *testing.T) {
	db := NewTestDB(t)
	ctx := t.Context()

	t.Run("events processed in event_id order", func(t *testing.T) {
		// Create multiple locations
		for i := 1; i <= 3; i++ {
			locID := "ordered-loc-" + string(rune('0'+i))
			payload := map[string]any{
				"location_id":  locID,
				"display_name": "Location " + string(rune('0'+i)),
				"parent_id":    nil,
			}
			_, err := db.insertEvent(ctx, LocationCreatedEvent, "test-user", payload, "")
			require.NoError(t, err)
		}

		// Get all events
		events, err := db.GetAllEvents(ctx)
		require.NoError(t, err)

		// Verify events are in ascending order by event_id
		for i := 1; i < len(events); i++ {
			assert.Greater(t, events[i].EventID, events[i-1].EventID,
				"events should be ordered by ascending event_id")
		}
	})
}

// TestItemsByLocation tests querying items by location.
func TestItemsByLocation(t *testing.T) {
	db := NewTestDB(t)
	ctx := t.Context()

	t.Run("retrieve items in location", func(t *testing.T) {
		// Create location
		locID := "items-loc"
		require.NoError(t, db.CreateLocation(ctx, locID, "Items Location", nil, false, 1, "2026-02-21T10:00:00Z"))

		// Create multiple items in location
		item1ID := "item-1"
		require.NoError(t, db.CreateItem(ctx, item1ID, "Item 1", locID, 2, "2026-02-21T10:00:00Z"))

		item2ID := "item-2"
		require.NoError(t, db.CreateItem(ctx, item2ID, "Item 2", locID, 3, "2026-02-21T10:00:00Z"))

		// Retrieve items by location
		items, err := db.GetItemsByLocation(ctx, locID)
		require.NoError(t, err)
		require.Len(t, items, 2)

		itemIDs := make(map[string]bool)
		for _, item := range items {
			itemIDs[item.ItemID] = true
		}

		assert.True(t, itemIDs[item1ID])
		assert.True(t, itemIDs[item2ID])
	})

	t.Run("system locations contain items", func(t *testing.T) {
		missing, err := db.GetLocationByCanonicalName(ctx, "missing")
		require.NoError(t, err)

		// Create item in missing location
		itemID := "missing-item"
		require.NoError(t, db.CreateItem(ctx, itemID, "Missing Item", missing.LocationID, 999, "2026-02-21T10:00:00Z"))

		// Retrieve items
		items, err := db.GetItemsByLocation(ctx, missing.LocationID)
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, itemID, items[0].ItemID)
	})
}

// TestCanonicalNameNormalization tests canonical name computation.
func TestCanonicalNameNormalization(t *testing.T) {
	db := NewTestDB(t)
	ctx := t.Context()

	t.Run("items have canonical names normalized on creation", func(t *testing.T) {
		// Create location
		locID := "canonical-loc"
		require.NoError(t, db.CreateLocation(ctx, locID, "Canonical Location", nil, false, 1, "2026-02-21T10:00:00Z"))

		// Create items with various display names
		testCases := []struct {
			itemID    string
			display   string
			canonical string
		}{
			{"item-1", "10mm Socket", "10mm_socket"},
			{"item-2", "Screwdriver Set", "screwdriver_set"},
			{"item-3", "Hammer", "hammer"},
		}

		for i, tc := range testCases {
			require.NoError(t, db.CreateItem(ctx, tc.itemID, tc.display, locID, int64(i+2), "2026-02-21T10:00:00Z"))

			item, err := db.GetItem(ctx, tc.itemID)
			require.NoError(t, err)
			require.NotNil(t, item)
			assert.Equal(t, tc.display, item.DisplayName)
			assert.Equal(t, tc.canonical, item.CanonicalName)
		}
	})

	t.Run("CanonicalizeString handles edge cases", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"Hello World", "hello_world"},
			{"  spaces  ", "spaces"},
			{"multiple   spaces", "multiple_spaces"},
			{"dash-separated", "dash_separated"},
			{"under_score", "under_score"},
			{"UPPERCASE", "uppercase"},
			{"Mixed-Case_With Spaces", "mixed_case_with_spaces"},
		}

		for _, tt := range tests {
			result := CanonicalizeString(tt.input)
			assert.Equal(t, tt.expected, result, "CanonicalizeString(%q)", tt.input)
		}
	})
}

// TestMigrationTracking tests that migrations are properly tracked.
func TestMigrationTracking(t *testing.T) {
	db := NewTestDB(t)

	t.Run("migration version is tracked", func(t *testing.T) {
		version, dirty, err := db.GetMigrationVersion()
		require.NoError(t, err)
		assert.Equal(t, uint(3), version, "should be at version 3 after all migrations")
		assert.False(t, dirty, "should not be dirty")
	})

	t.Run("system locations exist after migration", func(t *testing.T) {
		ctx := t.Context()
		missing, err := db.GetLocationByCanonicalName(ctx, "missing")
		require.NoError(t, err)
		require.NotNil(t, missing)
		assert.True(t, missing.IsSystem)
	})
}
