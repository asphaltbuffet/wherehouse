package find

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFindCmd_ReturnsNonNil(t *testing.T) {
	cmd := GetFindCmd()
	require.NotNil(t, cmd)
	assert.Equal(t, "find <name>", cmd.Use)
}

func TestNewDefaultFindCmd_ReturnsNonNil(t *testing.T) {
	cmd := NewDefaultFindCmd()
	require.NotNil(t, cmd)
	assert.Equal(t, "find <name>", cmd.Use)
}
