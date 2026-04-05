package add

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAddCmd(t *testing.T) {
	cmd1 := GetAddCmd()
	require.NotNil(t, cmd1)

	cmd2 := GetAddCmd()

	assert.Same(t, cmd1, cmd2)
}
