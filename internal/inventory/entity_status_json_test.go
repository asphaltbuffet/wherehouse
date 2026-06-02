package inventory_test

import (
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func TestEntityStatus_MarshalJSON(t *testing.T) {
	b, err := json.Marshal(inventory.EntityStatusBorrowed)
	require.NoError(t, err)
	assert.JSONEq(t, `"borrowed"`, string(b))
}

func TestEntityStatus_MarshalJSON_InvalidErrors(t *testing.T) {
	var zero inventory.EntityStatus
	_, err := json.Marshal(zero)
	assert.Error(t, err)
}

func TestEntityStatus_UnmarshalJSON(t *testing.T) {
	var es inventory.EntityStatus
	require.NoError(t, json.Unmarshal([]byte(`"missing"`), &es))
	assert.Equal(t, inventory.EntityStatusMissing, es)
}

func TestEntityStatus_UnmarshalJSON_UnknownErrors(t *testing.T) {
	var es inventory.EntityStatus
	assert.Error(t, json.Unmarshal([]byte(`"nope"`), &es))
}

func TestEntityStatus_JSONRoundtrip(t *testing.T) {
	for _, es := range []inventory.EntityStatus{
		inventory.EntityStatusOk,
		inventory.EntityStatusBorrowed,
		inventory.EntityStatusMissing,
		inventory.EntityStatusLoaned,
		inventory.EntityStatusRemoved,
	} {
		b, err := json.Marshal(es)
		require.NoError(t, err)

		var got inventory.EntityStatus
		require.NoError(t, json.Unmarshal(b, &got))
		assert.Equal(t, es, got)
	}
}
