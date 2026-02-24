package add

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetLocationCmd_Returns tests that GetLocationCmd returns a non-nil command.
func TestGetLocationCmd_Returns(t *testing.T) {
	cmd := GetLocationCmd()
	require.NotNil(t, cmd)
	assert.Contains(t, cmd.Use, "location")
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
}

// TestGetLocationCmd_Singleton tests that GetLocationCmd returns the same instance.
func TestGetLocationCmd_Singleton(t *testing.T) {
	cmd1 := GetLocationCmd()
	cmd2 := GetLocationCmd()
	assert.Same(t, cmd1, cmd2)
}
