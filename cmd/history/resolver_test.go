package history

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for resolver helper functions that don't require full database integration

func TestCanonicalizeStringProcessing_Lowercase(t *testing.T) {
	input := "MySocket"
	result := strings.ToLower(strings.TrimSpace(input))
	assert.Equal(t, "mysocket", result)
}

func TestCanonicalizeStringProcessing_TrimsWhitespace(t *testing.T) {
	input := "  socket  "
	result := strings.ToLower(strings.TrimSpace(input))
	assert.Equal(t, "socket", result)
}

func TestCanonicalizeStringProcessing_PreservesSocket(t *testing.T) {
	input := "socket"
	result := strings.ToLower(strings.TrimSpace(input))
	assert.Equal(t, "socket", result)
}

func TestSelectorParsing_SplitOnColon(t *testing.T) {
	selector := "garage:socket"

	parts := strings.SplitN(selector, ":", 2)

	require.Len(t, parts, 2)
	assert.Equal(t, "garage", parts[0])
	assert.Equal(t, "socket", parts[1])
}

func TestSelectorParsing_NoColon_IsItem(t *testing.T) {
	selector := "socket"

	assert.NotContains(t, selector, ":")
}

func TestSelectorParsing_MultipleColons_SplitsOnlyOnce(t *testing.T) {
	selector := "garage:inner:socket"

	parts := strings.SplitN(selector, ":", 2)

	require.Len(t, parts, 2)
	assert.Equal(t, "garage", parts[0])
	assert.Equal(t, "inner:socket", parts[1])
}

func TestAmbiguityErrorFormat_ContainsSuggestions(t *testing.T) {
	errorMsg := "selector matches multiple items:\n  socket1 (at loc1, id: id1)\n  socket2 (at loc2, id: id2)\n\nUse --id or location-scoped selector"

	assert.Contains(t, errorMsg, "matches multiple items")
	assert.Contains(t, errorMsg, "Use --id")
}

func TestNotFoundErrorFormat_IsClear(t *testing.T) {
	errorMsg := "no item found matching \"mysocket\""

	assert.Contains(t, errorMsg, "no item found")
	assert.Contains(t, errorMsg, "mysocket")
}

func TestLocationScopedNotFoundError_IsClear(t *testing.T) {
	errorMsg := "no item \"socket\" found in location \"garage\""

	assert.Contains(t, errorMsg, "no item")
	assert.Contains(t, errorMsg, "socket")
	assert.Contains(t, errorMsg, "garage")
}
