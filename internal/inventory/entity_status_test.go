package inventory_test

import (
	"testing"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntityStatus_String(t *testing.T) {
	assert.Equal(t, "ok", inventory.EntityStatusOk.String())
	assert.Equal(t, "borrowed", inventory.EntityStatusBorrowed.String())
	assert.Equal(t, "missing", inventory.EntityStatusMissing.String())
	assert.Equal(t, "loaned", inventory.EntityStatusLoaned.String())
	assert.Equal(t, "removed", inventory.EntityStatusRemoved.String())
}

func TestParseEntityStatus(t *testing.T) {
	got, err := inventory.ParseEntityStatus("ok")
	require.NoError(t, err)
	assert.Equal(t, inventory.EntityStatusOk, got)

	_, err = inventory.ParseEntityStatus("unknown")
	assert.Error(t, err)
}

func TestEntityStatus_SQLRoundtrip(t *testing.T) {
	es := inventory.EntityStatusMissing
	v, err := es.Value()
	require.NoError(t, err)
	assert.Equal(t, "missing", v)

	var scanned inventory.EntityStatus
	require.NoError(t, scanned.Scan("missing"))
	assert.Equal(t, inventory.EntityStatusMissing, scanned)
}
