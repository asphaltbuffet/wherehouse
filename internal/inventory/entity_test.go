package inventory_test

import (
	"encoding/json"
	"testing"
	"time"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntity_Fields(t *testing.T) {
	e := inventory.Entity{
		EntityID: "abc123", DisplayName: "Garage", CanonicalName: "garage",
		EntityType: inventory.EntityTypePlace, FullPathDisplay: "Garage",
		FullPathCanonical: "garage", Status: inventory.EntityStatusOk, LastEventID: 1,
		UpdatedAt: time.Now(),
	}
	assert.Equal(t, "abc123", e.EntityID)
	assert.Equal(t, inventory.EntityTypePlace, e.EntityType)
	assert.Equal(t, inventory.EntityStatusOk, e.Status)
}

func TestEvent_Fields(t *testing.T) {
	payload := json.RawMessage(`{"entity_id":"abc123"}`)
	note := "a note"
	entityID := "abc123"
	ev := inventory.Event{
		EventID: 1, EventType: inventory.EntityCreatedEvent,
		TimestampUTC: "2026-05-22T00:00:00Z", ActorUserID: "alice",
		Payload: payload, Note: &note, EntityID: &entityID,
	}
	require.NotNil(t, ev.Note)
	assert.Equal(t, "a note", *ev.Note)
	assert.Equal(t, inventory.EntityCreatedEvent, ev.EventType)
}
