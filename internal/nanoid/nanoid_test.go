package nanoid_test

import (
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/nanoid"
)

func TestIDLength_IsCorrectValue(t *testing.T) {
	assert.Equal(t, 10, nanoid.IDLength, "IDLength should be 10")
}

func TestNew_ReturnsIDOfCorrectLength(t *testing.T) {
	id, err := nanoid.New()
	require.NoError(t, err, "New() should not return an error")
	assert.Len(t, id, nanoid.IDLength, "New() should return an ID of IDLength characters")
}

func TestNew_ReturnsAlphanumericOnly(t *testing.T) {
	for range 100 {
		id, err := nanoid.New()
		require.NoError(t, err, "New() should not return an error")
		for _, c := range id {
			assert.True(t, unicode.IsLetter(c) || unicode.IsDigit(c),
				"New() returned non-alphanumeric char %q in ID %q", c, id)
		}
	}
}

func TestNew_NoDuplicates(t *testing.T) {
	seen := make(map[string]struct{}, 1000)
	for range 1000 {
		id, err := nanoid.New()
		require.NoError(t, err, "New() should not return an error")
		_, exists := seen[id]
		assert.False(t, exists, "New() produced duplicate ID %q", id)
		seen[id] = struct{}{}
	}
}

func TestMustNew_ReturnsIDOfCorrectLength(t *testing.T) {
	id := nanoid.MustNew()
	assert.Len(t, id, nanoid.IDLength, "MustNew() should return an ID of IDLength characters")
}

func TestMustNew_ReturnsAlphanumericOnly(t *testing.T) {
	for range 100 {
		id := nanoid.MustNew()
		for _, c := range id {
			assert.True(t, unicode.IsLetter(c) || unicode.IsDigit(c),
				"MustNew() returned non-alphanumeric char %q in ID %q", c, id)
		}
	}
}
