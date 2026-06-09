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
	t.Run("no tags", func(t *testing.T) {
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

		b, err := json.Marshal(app.ToListItems(results))
		require.NoError(t, err)

		assert.JSONEq(t, `[{
			"entity_id": "abc1234567",
			"path": "Garage:Toolbox",
			"type": "container",
			"status": "ok",
			"tags": []
		}]`, string(b))
	})

	t.Run("with tags", func(t *testing.T) {
		results := []app.EntityResult{
			{
				EntityID:        "def9876543",
				DisplayName:     "Drill",
				CanonicalName:   "drill",
				EntityType:      inventory.EntityTypeLeaf,
				FullPathDisplay: "Garage:Drill",
				Status:          inventory.EntityStatusMissing,
				Tags:            []string{"dewalt", "20v"},
			},
		}

		b, err := json.Marshal(app.ToListItems(results))
		require.NoError(t, err)

		assert.JSONEq(t, `[{
			"entity_id": "def9876543",
			"path": "Garage:Drill",
			"type": "leaf",
			"status": "missing",
			"tags": ["dewalt", "20v"]
		}]`, string(b))
	})
}

func TestToListItems_Empty(t *testing.T) {
	assert.Empty(t, app.ToListItems(nil))
}

func TestToScryItems_JSONContract(t *testing.T) {
	t.Run("list-all: distance is null", func(t *testing.T) {
		results := []app.FindResult{
			{
				Entity: app.EntityResult{
					EntityID:        "xyz9876543",
					DisplayName:     "Hammer",
					CanonicalName:   "hammer",
					EntityType:      inventory.EntityTypeLeaf,
					FullPathDisplay: "Garage:Toolbox:Hammer",
					Status:          inventory.EntityStatusOk,
				},
				Distance: 0,
			},
		}

		b, err := json.Marshal(app.ToScryItems(results, false))
		require.NoError(t, err)

		assert.JSONEq(t, `[{
			"entity_id": "xyz9876543",
			"path": "Garage:Toolbox:Hammer",
			"type": "leaf",
			"status": "ok",
			"distance": null
		}]`, string(b))
	})

	t.Run("search: distance is integer (0 for exact match)", func(t *testing.T) {
		results := []app.FindResult{
			{
				Entity: app.EntityResult{
					EntityID:        "xyz9876543",
					DisplayName:     "Hammer",
					CanonicalName:   "hammer",
					EntityType:      inventory.EntityTypeLeaf,
					FullPathDisplay: "Garage:Toolbox:Hammer",
					Status:          inventory.EntityStatusOk,
				},
				Distance: 0,
			},
		}

		b, err := json.Marshal(app.ToScryItems(results, true))
		require.NoError(t, err)

		assert.JSONEq(t, `[{
			"entity_id": "xyz9876543",
			"path": "Garage:Toolbox:Hammer",
			"type": "leaf",
			"status": "ok",
			"distance": 0
		}]`, string(b))
	})

	t.Run("search: non-zero distance", func(t *testing.T) {
		results := []app.FindResult{
			{
				Entity: app.EntityResult{
					EntityID:        "abc1234567",
					DisplayName:     "Hamster",
					CanonicalName:   "hamster",
					EntityType:      inventory.EntityTypeLeaf,
					FullPathDisplay: "Garage:Toolbox:Hamster",
					Status:          inventory.EntityStatusOk,
				},
				Distance: 3,
			},
		}

		b, err := json.Marshal(app.ToScryItems(results, true))
		require.NoError(t, err)

		assert.JSONEq(t, `[{
			"entity_id": "abc1234567",
			"path": "Garage:Toolbox:Hamster",
			"type": "leaf",
			"status": "ok",
			"distance": 3
		}]`, string(b))
	})
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

func TestToMoveOutput_JSONContract(t *testing.T) {
	result := app.EntityResult{
		EntityID:        "mov1234567",
		DisplayName:     "Wrench",
		CanonicalName:   "wrench",
		EntityType:      inventory.EntityTypeLeaf,
		FullPathDisplay: "Shed:Wrench",
		Status:          inventory.EntityStatusOk,
	}

	b, err := json.Marshal(app.ToMoveOutput(result))
	require.NoError(t, err)

	// New `move --json` shape (ADR 0014): current location only, no old_path.
	assert.JSONEq(t, `{
		"entity_id": "mov1234567",
		"display_name": "Wrench",
		"path": "Shed:Wrench"
	}`, string(b))
	assert.NotContains(t, string(b), "old_path")
	assert.NotContains(t, string(b), "new_path")
}

func TestToStatusOutput_WithNote(t *testing.T) {
	b, err := json.Marshal(app.ToStatusOutput(app.EntityResult{
		FullPathDisplay: "Garage:Bike",
		Status:          inventory.EntityStatusLoaned,
		StatusContext:   "to Alex",
	}))
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"path": "Garage:Bike",
		"status": "loaned",
		"status_context": "to Alex"
	}`, string(b))
}

func TestToStatusOutput_NoNoteOmitsContext(t *testing.T) {
	b, err := json.Marshal(app.ToStatusOutput(app.EntityResult{
		FullPathDisplay: "Garage:Bike",
		Status:          inventory.EntityStatusMissing,
	}))
	require.NoError(t, err)

	// status_context is omitted entirely when there is no note.
	assert.JSONEq(t, `{
		"path": "Garage:Bike",
		"status": "missing"
	}`, string(b))
	assert.NotContains(t, string(b), "status_context")
}
