package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigSet_ReturnsCommand(t *testing.T) {
	defer ResetForTesting()

	cmd := GetSetCmd()

	require.NotNil(t, cmd)
	assert.NotEmpty(t, cmd.Use)
}

func TestConfigSet_LocalFlag(t *testing.T) {
	defer ResetForTesting()

	cmd := GetSetCmd()
	local := cmd.Flags().Lookup("local")

	assert.NotNil(t, local)
}

func TestConfigSet_NoGlobalFlag(t *testing.T) {
	defer ResetForTesting()

	cmd := GetSetCmd()

	// Set command only has --local flag, not --global
	// By default it modifies global config
	local := cmd.Flags().Lookup("local")
	assert.NotNil(t, local)

	global := cmd.Flags().Lookup("global")
	assert.Nil(t, global)
}

func TestConfigSet_HasRunE(t *testing.T) {
	defer ResetForTesting()

	cmd := GetSetCmd()

	assert.NotNil(t, cmd.RunE)
}
