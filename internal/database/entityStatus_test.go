package database_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

func TestParseEntityStatus(t *testing.T) {
	tests := []struct {
		input   string
		want    database.EntityStatus
		wantErr bool
	}{
		{"ok", database.EntityStatusOk, false},
		{"borrowed", database.EntityStatusBorrowed, false},
		{"missing", database.EntityStatusMissing, false},
		{"loaned", database.EntityStatusLoaned, false},
		{"removed", database.EntityStatusRemoved, false},
		{"", 0, true},
		{"found", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := database.ParseEntityStatus(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEntityStatusString(t *testing.T) {
	assert.Equal(t, "ok", database.EntityStatusOk.String())
	assert.Equal(t, "borrowed", database.EntityStatusBorrowed.String())
	assert.Equal(t, "missing", database.EntityStatusMissing.String())
	assert.Equal(t, "loaned", database.EntityStatusLoaned.String())
	assert.Equal(t, "removed", database.EntityStatusRemoved.String())
}

func TestEntityStatusSQLRoundTrip(t *testing.T) {
	for _, es := range []database.EntityStatus{
		database.EntityStatusOk, database.EntityStatusBorrowed,
		database.EntityStatusMissing, database.EntityStatusLoaned, database.EntityStatusRemoved,
	} {
		val, err := es.Value()
		require.NoError(t, err)

		var scanned database.EntityStatus
		require.NoError(t, scanned.Scan(val))
		assert.Equal(t, es, scanned)
	}
}
