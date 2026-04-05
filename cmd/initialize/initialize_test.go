package initialize

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetInitializeCmd_Singleton(t *testing.T) {
	cmd1 := GetInitializeCmd()
	require.NotNil(t, cmd1)

	cmd2 := GetInitializeCmd()

	assert.Same(t, cmd1, cmd2)
}
