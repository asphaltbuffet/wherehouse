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
