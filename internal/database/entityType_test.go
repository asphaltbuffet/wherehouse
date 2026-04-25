package database_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

func TestParseEntityType(t *testing.T) {
	tests := []struct {
		input   string
		want    database.EntityType
		wantErr bool
	}{
		{"place", database.EntityTypePlace, false},
		{"container", database.EntityTypeContainer, false},
		{"leaf", database.EntityTypeLeaf, false},
		{"", 0, true},
		{"item", 0, true},
		{"location", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := database.ParseEntityType(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEntityTypeString(t *testing.T) {
	assert.Equal(t, "place", database.EntityTypePlace.String())
	assert.Equal(t, "container", database.EntityTypeContainer.String())
	assert.Equal(t, "leaf", database.EntityTypeLeaf.String())
}

func TestEntityTypeSQLRoundTrip(t *testing.T) {
	for _, et := range []database.EntityType{database.EntityTypePlace, database.EntityTypeContainer, database.EntityTypeLeaf} {
		val, err := et.Value()
		require.NoError(t, err)

		var scanned database.EntityType
		require.NoError(t, scanned.Scan(val))
		assert.Equal(t, et, scanned)
	}
}
