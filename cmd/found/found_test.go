package found

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFoundCmd_ReturnsNonNil(t *testing.T) {
	cmd := GetFoundCmd()
	require.NotNil(t, cmd)
	assert.Equal(t, "found <item-selector>... --in <location>", cmd.Use)
}

func TestNewDefaultFoundCmd_ReturnsNonNil(t *testing.T) {
	cmd := NewDefaultFoundCmd()
	require.NotNil(t, cmd)
	assert.Equal(t, "found <item-selector>... --in <location>", cmd.Use)
}
