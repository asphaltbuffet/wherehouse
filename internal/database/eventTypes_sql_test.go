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
		{ItemCreatedEvent, "item.created"},
		{LocationMovedEvent, "location.reparented"},
		{ProjectReopenedEvent, "project.reopened"},
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
		require.NoError(t, et.Scan("item.created"))
		assert.Equal(t, ItemCreatedEvent, et)
	})

	t.Run("scan location.reparented", func(t *testing.T) {
		var et EventType
		require.NoError(t, et.Scan("location.reparented"))
		assert.Equal(t, LocationMovedEvent, et)
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
