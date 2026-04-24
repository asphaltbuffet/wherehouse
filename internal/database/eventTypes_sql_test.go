package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEventTypeValuer tests that EventType implements [driver.Valuer].
func TestEventTypeValuer(t *testing.T) {
	tests := []struct {
		et       EventType
		expected string
	}{
		{EntityCreatedEvent, "entity.created"},
		{EntityReparentedEvent, "entity.reparented"},
		{EntityRemovedEvent, "entity.removed"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			v, err := tt.et.Value()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, v)
		})
	}
}

// TestEventTypeScanner tests that EventType implements [sql.Scanner].
func TestEventTypeScanner(t *testing.T) {
	t.Run("scan valid string", func(t *testing.T) {
		var et EventType
		require.NoError(t, et.Scan("entity.created"))
		assert.Equal(t, EntityCreatedEvent, et)
	})

	t.Run("scan entity.reparented", func(t *testing.T) {
		var et EventType
		require.NoError(t, et.Scan("entity.reparented"))
		assert.Equal(t, EntityReparentedEvent, et)
	})

	t.Run("scan unknown string returns error", func(t *testing.T) {
		var et EventType
		assert.Error(t, et.Scan("bogus"))
	})

	t.Run("scan non-string returns error", func(t *testing.T) {
		var et EventType
		assert.Error(t, et.Scan(42))
	})

	t.Run("scan nil returns error", func(t *testing.T) {
		var et EventType
		assert.Error(t, et.Scan(nil))
	})
}
