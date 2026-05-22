package store_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := store.Open(store.Config{Path: path, AutoMigrate: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

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
