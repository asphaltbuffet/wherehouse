package add

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetItemCmd(t *testing.T) {
	cmd1 := GetItemCmd()
	require.NotNil(t, cmd1)

	cmd2 := GetItemCmd()

	assert.Same(t, cmd1, cmd2)
}
