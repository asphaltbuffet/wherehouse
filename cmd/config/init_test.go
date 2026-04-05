package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetInitCmd_Singleton(t *testing.T) {
	cmd1 := GetInitCmd()
	require.NotNil(t, cmd1)

	cmd2 := GetInitCmd()

	assert.Same(t, cmd1, cmd2)
}
