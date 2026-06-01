package app_test

import (
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func TestToListItems_JSONContract(t *testing.T) {
	results := []app.EntityResult{
		{
			EntityID:        "abc1234567",
			DisplayName:     "Toolbox",
			CanonicalName:   "toolbox",
			EntityType:      inventory.EntityTypeContainer,
			FullPathDisplay: "Garage:Toolbox",
			Status:          inventory.EntityStatusOk,
			HasChildren:     true,
		},
	}

	items := app.ToListItems(results)

	b, err := json.Marshal(items)
	require.NoError(t, err)

	// Pins the current `wherehouse list --json` wire format.
	assert.JSONEq(t, `[{
		"entity_id": "abc1234567",
		"path": "Garage:Toolbox",
		"type": "container",
		"status": "ok"
	}]`, string(b))
}

func TestToListItems_Empty(t *testing.T) {
	assert.Empty(t, app.ToListItems(nil))
}

func TestToScryItems_JSONContract(t *testing.T) {
	results := []app.EntityResult{
		{
			EntityID:        "xyz9876543",
			DisplayName:     "Hammer",
			CanonicalName:   "hammer",
			EntityType:      inventory.EntityTypeLeaf,
			FullPathDisplay: "Garage:Toolbox:Hammer",
			Status:          inventory.EntityStatusOk,
		},
	}

	b, err := json.Marshal(app.ToScryItems(results))
	require.NoError(t, err)

	// Pins the current `wherehouse scry --json` wire format (Distance omitted; see #216).
	assert.JSONEq(t, `[{
		"entity_id": "xyz9876543",
		"path": "Garage:Toolbox:Hammer",
		"type": "leaf",
		"status": "ok"
	}]`, string(b))
}

func TestToHistoryItems_JSONContract(t *testing.T) {
	results := []app.HistoryResult{
		{
			EventID:      42,
			EventType:    inventory.EntityStatusChangedEvent,
			TimestampUTC: "2026-06-01T12:00:00Z",
			ActorUserID:  "grue",
			Payload:      []byte(`{"ignored":true}`),
			Note:         "should not appear",
		},
	}

	b, err := json.Marshal(app.ToHistoryItems(results))
	require.NoError(t, err)

	// Pins the current `wherehouse history --json` shape: Payload and Note are dropped.
	assert.JSONEq(t, `[{
		"event_id": 42,
		"event_type": "entity.status_changed",
		"timestamp": "2026-06-01T12:00:00Z",
		"actor_user": "grue"
	}]`, string(b))
}

func TestToAddOutput_JSONContract(t *testing.T) {
	result := app.EntityResult{
		EntityID:        "add1234567",
		DisplayName:     "Wrench",
		CanonicalName:   "wrench",
		EntityType:      inventory.EntityTypeLeaf,
		FullPathDisplay: "Garage:Toolbox:Wrench",
		Status:          inventory.EntityStatusOk,
	}

	b, err := json.Marshal(app.ToAddOutput(result))
	require.NoError(t, err)

	// Pins the current `wherehouse add --json` shape.
	assert.JSONEq(t, `{
		"entity_id": "add1234567",
		"path": "Garage:Toolbox:Wrench"
	}`, string(b))
}
