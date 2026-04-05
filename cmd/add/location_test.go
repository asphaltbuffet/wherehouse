package add

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetLocationCmd(t *testing.T) {
	cmd1 := GetLocationCmd()
	require.NotNil(t, cmd1)

	cmd2 := GetLocationCmd()

	assert.Same(t, cmd1, cmd2)
}
