package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPathCmd_Singleton(t *testing.T) {
	cmd1 := GetPathCmd()
	require.NotNil(t, cmd1)

	cmd2 := GetPathCmd()

	assert.Same(t, cmd1, cmd2)
}
