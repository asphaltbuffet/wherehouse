package inventory_test

import (
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func TestEventType_MarshalJSON(t *testing.T) {
	b, err := json.Marshal(inventory.EntityStatusChangedEvent)
	require.NoError(t, err)
	assert.JSONEq(t, `"entity.status_changed"`, string(b))
}

func TestEventType_MarshalJSON_InvalidErrors(t *testing.T) {
	var zero inventory.EventType
	_, err := json.Marshal(zero)
	assert.Error(t, err)
}

func TestEventType_UnmarshalJSON(t *testing.T) {
	var et inventory.EventType
	require.NoError(t, json.Unmarshal([]byte(`"entity.created"`), &et))
	assert.Equal(t, inventory.EntityCreatedEvent, et)
}

func TestEventType_UnmarshalJSON_UnknownErrors(t *testing.T) {
	var et inventory.EventType
	assert.Error(t, json.Unmarshal([]byte(`"nope"`), &et))
}

func TestEventType_JSONRoundtrip(t *testing.T) {
	for _, et := range []inventory.EventType{
		inventory.EntityCreatedEvent,
		inventory.EntityRenamedEvent,
		inventory.EntityReparentedEvent,
		inventory.EntityPathChangedEvent,
		inventory.EntityStatusChangedEvent,
		inventory.EntityRemovedEvent,
	} {
		b, err := json.Marshal(et)
		require.NoError(t, err)

		var got inventory.EventType
		require.NoError(t, json.Unmarshal(b, &got))
		assert.Equal(t, et, got)
	}
}
