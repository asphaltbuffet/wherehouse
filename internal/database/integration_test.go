package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBasicWorkflow tests schema presence and basic event log operations.
func TestBasicWorkflow(t *testing.T) {
	db := NewTestDB(t)
	ctx := t.Context()

	t.Run("entities_current table exists", func(t *testing.T) {
		var name string
		require.NoError(t, db.db.QueryRowContext(ctx,
			"SELECT name FROM sqlite_master WHERE type='table' AND name='entities_current'",
		).Scan(&name))
		assert.Equal(t, "entities_current", name)
	})

	t.Run("events table has entity_id column", func(t *testing.T) {
		// Insert an event with entity_id and verify it is stored
		_, err := db.insertEvent(ctx, EntityCreatedEvent, TestActorUser, map[string]any{
			"entity_id":    "basic-entity-1",
			"display_name": "Basic Entity",
		}, "")
		require.NoError(t, err)

		events, err := db.GetAllEvents(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, events)
	})
}

// TestValidationErrorScenarios tests error handling for validation failures.
func TestValidationErrorScenarios(t *testing.T) {
	db := NewTestDB(t)
	ctx := t.Context()

	t.Run("unknown entity event returns not found", func(t *testing.T) {
		_, err := db.GetEventByID(ctx, 999999)
		assert.ErrorIs(t, err, ErrEventNotFound)
	})
}

// TestCanonicalNameNormalization tests canonical name computation.
func TestCanonicalNameNormalization(t *testing.T) {
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
		assert.EqualValues(t, 6, version, "should be at version 6 after all migrations")
		assert.False(t, dirty, "should not be dirty")
	})
}

// TestEventOrdering tests that events are inserted in order.
func TestEventOrdering(t *testing.T) {
	db := NewTestDB(t)
	ctx := t.Context()

	t.Run("events inserted in event_id order", func(t *testing.T) {
		// Insert events directly without processing
		for i := 1; i <= 3; i++ {
			_, err := db.insertEvent(ctx, EntityCreatedEvent, "test-user", map[string]any{
				"entity_id": "test-entity-" + string(rune('0'+i)),
			}, "")
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

// TestEventLog tests event logging and retrieval.
func TestEventLog(t *testing.T) {
	db := NewTestDB(t)
	ctx := t.Context()

	t.Run("events are recorded in order", func(t *testing.T) {
		// Insert events directly
		eventID1, err := db.insertEvent(ctx, EntityCreatedEvent, "test-user", map[string]any{
			"entity_id": "event-entity-1",
		}, "first entity")
		require.NoError(t, err)

		eventID2, err := db.insertEvent(ctx, EntityCreatedEvent, "test-user", map[string]any{
			"entity_id": "event-entity-2",
		}, "second entity")
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
		// Insert entity events
		_, err := db.insertEvent(ctx, EntityCreatedEvent, "test-user", map[string]any{
			"entity_id": "entity-type-1",
		}, "")
		require.NoError(t, err)

		_, err = db.insertEvent(ctx, EntityRenamedEvent, "test-user", map[string]any{
			"entity_id": "entity-type-2",
		}, "")
		require.NoError(t, err)

		// Get created events
		createdEvents, err := db.GetEventsByType(ctx, EntityCreatedEvent)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(createdEvents), 1)

		// Verify all are created events
		for _, evt := range createdEvents {
			assert.Equal(t, EntityCreatedEvent, evt.EventType)
		}
	})
}
