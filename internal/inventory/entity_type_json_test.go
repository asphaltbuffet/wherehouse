package inventory_test

import (
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func TestEntityType_MarshalJSON(t *testing.T) {
	b, err := json.Marshal(inventory.EntityTypeContainer)
	require.NoError(t, err)
	assert.JSONEq(t, `"container"`, string(b))
}

func TestEntityType_MarshalJSON_InvalidErrors(t *testing.T) {
	var zero inventory.EntityType
	_, err := json.Marshal(zero)
	assert.Error(t, err)
}

func TestEntityType_UnmarshalJSON(t *testing.T) {
	var et inventory.EntityType
	require.NoError(t, json.Unmarshal([]byte(`"leaf"`), &et))
	assert.Equal(t, inventory.EntityTypeLeaf, et)
}

func TestEntityType_UnmarshalJSON_UnknownErrors(t *testing.T) {
	var et inventory.EntityType
	assert.Error(t, json.Unmarshal([]byte(`"nope"`), &et))
}

func TestEntityType_UnmarshalJSON_NumberErrors(t *testing.T) {
	var et inventory.EntityType
	assert.Error(t, json.Unmarshal([]byte(`2`), &et))
}

func TestEntityType_JSONRoundtrip(t *testing.T) {
	for _, et := range []inventory.EntityType{
		inventory.EntityTypePlace,
		inventory.EntityTypeContainer,
		inventory.EntityTypeLeaf,
	} {
		b, err := json.Marshal(et)
		require.NoError(t, err)

		var got inventory.EntityType
		require.NoError(t, json.Unmarshal(b, &got))
		assert.Equal(t, et, got)
	}
}
