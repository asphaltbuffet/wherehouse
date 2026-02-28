package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetGetCmd_Singleton(t *testing.T) {
	cmd1 := GetGetCmd()
	require.NotNil(t, cmd1)

	cmd2 := GetGetCmd()

	assert.Same(t, cmd1, cmd2)
}
