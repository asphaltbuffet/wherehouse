package add

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetItemCmd_Returns tests that GetItemCmd returns a non-nil command.
func TestGetItemCmd_Returns(t *testing.T) {
	cmd := GetItemCmd()
	require.NotNil(t, cmd)
	assert.Contains(t, cmd.Use, "item")
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
}

// TestGetItemCmd_Singleton tests that GetItemCmd returns the same instance.
func TestGetItemCmd_Singleton(t *testing.T) {
	cmd1 := GetItemCmd()
	cmd2 := GetItemCmd()
	assert.Same(t, cmd1, cmd2)
}
