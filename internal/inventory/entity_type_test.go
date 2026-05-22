package inventory_test

import (
	"testing"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntityType_String(t *testing.T) {
	assert.Equal(t, "place", inventory.EntityTypePlace.String())
	assert.Equal(t, "container", inventory.EntityTypeContainer.String())
	assert.Equal(t, "leaf", inventory.EntityTypeLeaf.String())
}

func TestParseEntityType(t *testing.T) {
	got, err := inventory.ParseEntityType("place")
	require.NoError(t, err)
	assert.Equal(t, inventory.EntityTypePlace, got)

	got, err = inventory.ParseEntityType("container")
	require.NoError(t, err)
	assert.Equal(t, inventory.EntityTypeContainer, got)

	_, err = inventory.ParseEntityType("unknown")
	assert.Error(t, err)
}

func TestEntityType_SQLRoundtrip(t *testing.T) {
	et := inventory.EntityTypeContainer
	v, err := et.Value()
	require.NoError(t, err)
	assert.Equal(t, "container", v)

	var scanned inventory.EntityType
	require.NoError(t, scanned.Scan("container"))
	assert.Equal(t, inventory.EntityTypeContainer, scanned)
}
