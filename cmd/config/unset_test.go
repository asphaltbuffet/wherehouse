package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigUnset_ReturnsCommand(t *testing.T) {
	defer ResetForTesting()

	cmd := GetUnsetCmd()

	require.NotNil(t, cmd)
	assert.NotEmpty(t, cmd.Use)
}

func TestConfigUnset_LocalFlag(t *testing.T) {
	defer ResetForTesting()

	cmd := GetUnsetCmd()
	local := cmd.Flags().Lookup("local")

	assert.NotNil(t, local)
}

func TestConfigUnset_GlobalFlag(t *testing.T) {
	defer ResetForTesting()

	cmd := GetUnsetCmd()
	global := cmd.Flags().Lookup("global")

	assert.NotNil(t, global)
}

func TestConfigUnset_HasRunE(t *testing.T) {
	defer ResetForTesting()

	cmd := GetUnsetCmd()

	assert.NotNil(t, cmd.RunE)
}
