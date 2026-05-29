package store_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

func TestAppendRawEvent(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	payload := json.RawMessage(`{"entity_id":"abc","display_name":"Garage","entity_type":"place"}`)
	entityID := "abc"

	eventID, err := s.AppendRawEvent(ctx, inventory.EntityCreatedEvent, "alice", payload, nil, &entityID)
	require.NoError(t, err)
	assert.Positive(t, eventID)
}

func TestGetEventByID(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	payload := json.RawMessage(`{"entity_id":"abc","display_name":"Garage","entity_type":"place"}`)
	entityID := "abc"
	eventID, err := s.AppendRawEvent(ctx, inventory.EntityCreatedEvent, "alice", payload, nil, &entityID)
	require.NoError(t, err)

	ev, err := s.GetEventByID(ctx, eventID)
	require.NoError(t, err)
	assert.Equal(t, inventory.EntityCreatedEvent, ev.EventType)
	assert.Equal(t, "alice", ev.ActorUserID)
}

func TestGetEventsByEntity(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	payload := json.RawMessage(`{"entity_id":"abc","display_name":"Garage","entity_type":"place"}`)
	entityID := "abc"
	_, err := s.AppendRawEvent(ctx, inventory.EntityCreatedEvent, "alice", payload, nil, &entityID)
	require.NoError(t, err)

	events, err := s.GetEventsByEntity(ctx, entityID)
	require.NoError(t, err)
	assert.Len(t, events, 1)
}

func TestHasEvents_EmptyDatabase(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	has, err := s.HasEvents(ctx)
	require.NoError(t, err)
	assert.False(t, has)
}

func TestHasEvents_AfterInsert(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	payload := json.RawMessage(`{"entity_id":"x1","display_name":"Box","entity_type":"place"}`)
	entityID := "x1"
	_, err := s.AppendRawEvent(ctx, inventory.EntityCreatedEvent, "alice", payload, nil, &entityID)
	require.NoError(t, err)

	has, err := s.HasEvents(ctx)
	require.NoError(t, err)
	assert.True(t, has)
}

func TestClearAllData_RemovesEventsAndEntities(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	// Seed an event and an entity projection
	payload := json.RawMessage(`{"entity_id":"c1","display_name":"Shelf","entity_type":"place"}`)
	entityID := "c1"
	_, err := s.AppendRawEvent(ctx, inventory.EntityCreatedEvent, "alice", payload, nil, &entityID)
	require.NoError(t, err)

	err = s.ClearAllData(ctx)
	require.NoError(t, err)

	has, err := s.HasEvents(ctx)
	require.NoError(t, err)
	assert.False(t, has, "events table should be empty after ClearAllData")

	_, err = s.GetEntity(ctx, "c1")
	assert.ErrorIs(t, err, store.ErrNotFound, "entities_current should be empty after ClearAllData")
}

func TestClearAllData_PreservesSchemaMetadata(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	err := s.ClearAllData(ctx)
	require.NoError(t, err)

	// schema_metadata is seeded by the migration with "created_at" and "app_version"
	val, err := s.GetMetadata(ctx, "app_version")
	require.NoError(t, err)
	assert.NotEmpty(t, val, "schema_metadata should survive ClearAllData")
}

func TestGetAllEventsRaw(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	entityID := "e1"
	payload := json.RawMessage(`{"entity_id":"e1","display_name":"Garage","entity_type":"place"}`)

	_, err := s.AppendRawEvent(ctx, inventory.EntityCreatedEvent, "alice", payload, nil, &entityID)
	require.NoError(t, err)

	// Insert an event with an unknown type directly — AppendRawEvent enforces typed EventType.
	_, err = s.DB().ExecContext(ctx,
		`INSERT INTO events (event_type, timestamp_utc, actor_user_id, payload, note, entity_id)
		 VALUES (?, ?, ?, ?, NULL, ?)`,
		"entity.unknown_future", "2026-01-01T00:00:00Z", "alice", `{}`, &entityID,
	)
	require.NoError(t, err)

	events, err := s.GetAllEventsRaw(ctx)
	require.NoError(t, err)
	require.Len(t, events, 2)

	assert.Equal(t, "entity.created", events[0].EventType)
	assert.Equal(t, "entity.unknown_future", events[1].EventType)
	assert.NotNil(t, events[0].EntityID)
	assert.Equal(t, "e1", *events[0].EntityID)
}
