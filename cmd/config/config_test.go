package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetConfigCmd_Singleton(t *testing.T) {
	cmd1 := GetConfigCmd()
	require.NotNil(t, cmd1)

	cmd2 := GetConfigCmd()

	assert.Same(t, cmd1, cmd2)
}
