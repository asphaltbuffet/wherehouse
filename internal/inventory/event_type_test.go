package inventory_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func TestEventType_String(t *testing.T) {
	assert.Equal(t, "entity.created", inventory.EntityCreatedEvent.String())
	assert.Equal(t, "entity.renamed", inventory.EntityRenamedEvent.String())
	assert.Equal(t, "entity.reparented", inventory.EntityReparentedEvent.String())
	assert.Equal(t, "entity.path_changed", inventory.EntityPathChangedEvent.String())
	assert.Equal(t, "entity.status_changed", inventory.EntityStatusChangedEvent.String())
	assert.Equal(t, "entity.removed", inventory.EntityRemovedEvent.String())
}

func TestParseEventType(t *testing.T) {
	got, err := inventory.ParseEventType("entity.created")
	require.NoError(t, err)
	assert.Equal(t, inventory.EntityCreatedEvent, got)
	_, err = inventory.ParseEventType("unknown")
	assert.Error(t, err)
}

func TestEventType_SQLRoundtrip(t *testing.T) {
	et := inventory.EntityRenamedEvent
	v, err := et.Value()
	require.NoError(t, err)
	assert.Equal(t, "entity.renamed", v)
	var scanned inventory.EventType
	require.NoError(t, scanned.Scan("entity.renamed"))
	assert.Equal(t, inventory.EntityRenamedEvent, scanned)
}
