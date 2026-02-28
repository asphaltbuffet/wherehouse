package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSetCmd_Singleton(t *testing.T) {
	cmd1 := GetSetCmd()
	require.NotNil(t, cmd1)

	cmd2 := GetSetCmd()

	assert.Same(t, cmd1, cmd2)
}
