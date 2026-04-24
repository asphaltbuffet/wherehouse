package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEventTypeString tests that EventType constants have correct string representations.
func TestEventTypeString(t *testing.T) {
	tests := []struct {
		et       EventType
		expected string
	}{
		{EntityCreatedEvent, "entity.created"},
		{EntityRenamedEvent, "entity.renamed"},
		{EntityReparentedEvent, "entity.reparented"},
		{EntityPathChangedEvent, "entity.path_changed"},
		{EntityStatusChangedEvent, "entity.status_changed"},
		{EntityRemovedEvent, "entity.removed"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.et.String(), "EventType(%d).String()", int(tt.et))
	}
}

// TestParseEventType tests that ParseEventType correctly round-trips each constant.
func TestParseEventType(t *testing.T) {
	allTypes := []EventType{
		EntityCreatedEvent,
		EntityRenamedEvent,
		EntityReparentedEvent,
		EntityPathChangedEvent,
		EntityStatusChangedEvent,
		EntityRemovedEvent,
	}

	for _, et := range allTypes {
		t.Run(et.String(), func(t *testing.T) {
			parsed, err := ParseEventType(et.String())
			require.NoError(t, err)
			assert.Equal(t, et, parsed, "ParseEventType(%q) should round-trip correctly", et.String())
		})
	}
}

// TestParseEventTypeUnknown tests that ParseEventType returns an error for unknown strings.
func TestParseEventTypeUnknown(t *testing.T) {
	unknowns := []string{
		"",
		"unknown",
		"EntityCreatedEvent",
		"entity_created",
		"ENTITY.CREATED",
		"item.created",
		"location.created",
	}

	for _, s := range unknowns {
		t.Run(s, func(t *testing.T) {
			_, err := ParseEventType(s)
			assert.Error(t, err, "ParseEventType(%q) should return an error", s)
		})
	}
}
