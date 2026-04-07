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
		{ItemCreatedEvent, "item.created"},
		{ItemMovedEvent, "item.moved"},
		{ItemMissingEvent, "item.missing"},
		{ItemBorrowedEvent, "item.borrowed"},
		{ItemLoanedEvent, "item.loaned"},
		{ItemFoundEvent, "item.found"},
		{ItemRemovedEvent, "item.removed"},
		{LocationCreatedEvent, "location.created"},
		{LocationRenamedEvent, "location.renamed"},
		{LocationMovedEvent, "location.reparented"},
		{LocationRemovedEvent, "location.removed"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.et.String(), "EventType(%d).String()", int(tt.et))
	}
}

// TestParseEventType tests that ParseEventType correctly round-trips each constant.
func TestParseEventType(t *testing.T) {
	allTypes := []EventType{
		ItemCreatedEvent,
		ItemMovedEvent,
		ItemMissingEvent,
		ItemBorrowedEvent,
		ItemLoanedEvent,
		ItemFoundEvent,
		ItemRemovedEvent,
		LocationCreatedEvent,
		LocationRenamedEvent,
		LocationMovedEvent,
		LocationRemovedEvent,
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
		"ItemCreatedEvent",
		"item_created",
		"ITEM.CREATED",
	}

	for _, s := range unknowns {
		t.Run(s, func(t *testing.T) {
			_, err := ParseEventType(s)
			assert.Error(t, err, "ParseEventType(%q) should return an error", s)
		})
	}
}
