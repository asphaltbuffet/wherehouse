package add

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAddCmd_ReturnsNonNil(t *testing.T) {
	cmd := GetAddCmd()
	require.NotNil(t, cmd)
	assert.Equal(t, "add", cmd.Use)
}

func TestNewDefaultAddCmd_ReturnsNonNil(t *testing.T) {
	cmd := NewDefaultAddCmd()
	require.NotNil(t, cmd)
	assert.Equal(t, "add", cmd.Use)
}
