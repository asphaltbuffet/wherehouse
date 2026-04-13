package history

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetHistoryCmd_ReturnsNonNil(t *testing.T) {
	cmd := NewHistoryCmd()
	require.NotNil(t, cmd)
	assert.Equal(t, "history <item-selector>", cmd.Use)
}
