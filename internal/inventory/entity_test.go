package inventory_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func TestEntity_Fields(t *testing.T) {
	now := time.Now()
	e := inventory.Entity{
		EntityID:          "abc123",
		DisplayName:       "Garage",
		CanonicalName:     "garage",
		EntityType:        inventory.EntityTypePlace,
		FullPathDisplay:   "Garage",
		FullPathCanonical: "garage",
		Status:            inventory.EntityStatusOk,
		LastEventID:       1,
		UpdatedAt:         now,
	}
	assert.Equal(t, "abc123", e.EntityID)
	assert.Equal(t, "Garage", e.DisplayName)
	assert.Equal(t, "garage", e.CanonicalName)
	assert.Equal(t, inventory.EntityTypePlace, e.EntityType)
	assert.Equal(t, "Garage", e.FullPathDisplay)
	assert.Equal(t, "garage", e.FullPathCanonical)
	assert.Equal(t, inventory.EntityStatusOk, e.Status)
	assert.Equal(t, int64(1), e.LastEventID)
	assert.Equal(t, now, e.UpdatedAt)
}

func TestEvent_Fields(t *testing.T) {
	payload := json.RawMessage(`{"entity_id":"abc123"}`)
	note := "a note"
	entityID := "abc123"
	ev := inventory.Event{
		EventID:      1,
		EventType:    inventory.EntityCreatedEvent,
		TimestampUTC: "2026-05-22T00:00:00Z",
		ActorUserID:  "alice",
		Payload:      payload,
		Note:         &note,
		EntityID:     &entityID,
	}
	assert.Equal(t, int64(1), ev.EventID)
	assert.Equal(t, inventory.EntityCreatedEvent, ev.EventType)
	assert.Equal(t, "2026-05-22T00:00:00Z", ev.TimestampUTC)
	assert.Equal(t, "alice", ev.ActorUserID)
	assert.Equal(t, payload, ev.Payload)
	require.NotNil(t, ev.Note)
	assert.Equal(t, "a note", *ev.Note)
	require.NotNil(t, ev.EntityID)
	assert.Equal(t, "abc123", *ev.EntityID)
}
