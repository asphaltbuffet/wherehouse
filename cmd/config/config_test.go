package config

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetConfigCmd_Returns(t *testing.T) {
	defer ResetForTesting()

	cmd := GetConfigCmd()

	require.NotNil(t, cmd)
	assert.Equal(t, "config", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestGetConfigCmd_Singleton(t *testing.T) {
	defer ResetForTesting()

	cmd1 := GetConfigCmd()
	cmd2 := GetConfigCmd()

	assert.Same(t, cmd1, cmd2)
}

func TestGetConfigCmd_HasSubcommands(t *testing.T) {
	defer ResetForTesting()

	cmd := GetConfigCmd()
	subcommands := cmd.Commands()

	require.NotEmpty(t, subcommands)
	// Should have 7 subcommands
	assert.Len(t, subcommands, 7)
}

func TestGetConfigCmd_SubcommandNames(t *testing.T) {
	defer ResetForTesting()

	cmd := GetConfigCmd()
	subcommands := cmd.Commands()

	names := make(map[string]bool)
	for _, subcmd := range subcommands {
		names[subcmd.Name()] = true
	}

	assert.True(t, names["init"])
	assert.True(t, names["get"])
	assert.True(t, names["set"])
	assert.True(t, names["unset"])
	assert.True(t, names["path"])
	assert.True(t, names["check"])
	assert.True(t, names["edit"])
}

func TestGetInitCmd_Returns(t *testing.T) {
	defer ResetForTesting()

	cmd := GetInitCmd()

	require.NotNil(t, cmd)
	assert.Equal(t, "init", cmd.Use)
}

func TestGetInitCmd_Singleton(t *testing.T) {
	defer ResetForTesting()

	cmd1 := GetInitCmd()
	cmd2 := GetInitCmd()

	assert.Same(t, cmd1, cmd2)
}

func TestGetInitCmd_HasFlags(t *testing.T) {
	defer ResetForTesting()

	cmd := GetInitCmd()

	local := cmd.Flags().Lookup("local")
	assert.NotNil(t, local)

	force := cmd.Flags().Lookup("force")
	assert.NotNil(t, force)
}

func TestGetGetCmd_Returns(t *testing.T) {
	defer ResetForTesting()

	cmd := GetGetCmd()

	require.NotNil(t, cmd)
	assert.NotEmpty(t, cmd.Use)
	assert.True(t, cmd.Use == "get" || cmd.Use == "get [key]")
}

func TestGetGetCmd_Singleton(t *testing.T) {
	defer ResetForTesting()

	cmd1 := GetGetCmd()
	cmd2 := GetGetCmd()

	assert.Same(t, cmd1, cmd2)
}

func TestGetSetCmd_Returns(t *testing.T) {
	defer ResetForTesting()

	cmd := GetSetCmd()

	require.NotNil(t, cmd)
	assert.NotEmpty(t, cmd.Use)
	assert.True(t, cmd.Use == "set" || cmd.Use == "set <key> <value>")
}

func TestGetSetCmd_Singleton(t *testing.T) {
	defer ResetForTesting()

	cmd1 := GetSetCmd()
	cmd2 := GetSetCmd()

	assert.Same(t, cmd1, cmd2)
}

func TestGetUnsetCmd_Returns(t *testing.T) {
	defer ResetForTesting()

	cmd := GetUnsetCmd()

	require.NotNil(t, cmd)
	assert.NotEmpty(t, cmd.Use)
	assert.True(t, cmd.Use == "unset" || cmd.Use == "unset <key>")
}

func TestGetUnsetCmd_Singleton(t *testing.T) {
	defer ResetForTesting()

	cmd1 := GetUnsetCmd()
	cmd2 := GetUnsetCmd()

	assert.Same(t, cmd1, cmd2)
}

func TestGetPathCmd_Returns(t *testing.T) {
	defer ResetForTesting()

	cmd := GetPathCmd()

	require.NotNil(t, cmd)
	assert.Equal(t, "path", cmd.Use)
}

func TestGetPathCmd_Singleton(t *testing.T) {
	defer ResetForTesting()

	cmd1 := GetPathCmd()
	cmd2 := GetPathCmd()

	assert.Same(t, cmd1, cmd2)
}

func TestGetCheckCmd_Returns(t *testing.T) {
	defer ResetForTesting()

	cmd := GetCheckCmd()

	require.NotNil(t, cmd)
	assert.Equal(t, "check", cmd.Use)
}

func TestGetCheckCmd_Singleton(t *testing.T) {
	defer ResetForTesting()

	cmd1 := GetCheckCmd()
	cmd2 := GetCheckCmd()

	assert.Same(t, cmd1, cmd2)
}

func TestGetEditCmd_Returns(t *testing.T) {
	defer ResetForTesting()

	cmd := GetEditCmd()

	require.NotNil(t, cmd)
	assert.Equal(t, "edit", cmd.Use)
}

func TestGetEditCmd_Singleton(t *testing.T) {
	defer ResetForTesting()

	cmd1 := GetEditCmd()
	cmd2 := GetEditCmd()

	assert.Same(t, cmd1, cmd2)
}

func TestResetForTesting_ResetsAll(t *testing.T) {
	// Initialize all commands
	GetConfigCmd()
	GetInitCmd()
	GetGetCmd()
	GetSetCmd()
	GetUnsetCmd()
	GetPathCmd()
	GetCheckCmd()
	GetEditCmd()

	// Verify they're initialized
	assert.NotNil(t, configCmd)
	assert.NotNil(t, initCmd)
	assert.NotNil(t, getCmd)
	assert.NotNil(t, setCmd)
	assert.NotNil(t, unsetCmd)
	assert.NotNil(t, pathCmd)
	assert.NotNil(t, checkCmd)
	assert.NotNil(t, editCmd)

	// Reset
	ResetForTesting()

	// Verify all are nil
	assert.Nil(t, configCmd)
	assert.Nil(t, initCmd)
	assert.Nil(t, getCmd)
	assert.Nil(t, setCmd)
	assert.Nil(t, unsetCmd)
	assert.Nil(t, pathCmd)
	assert.Nil(t, checkCmd)
	assert.Nil(t, editCmd)
}

func TestResetForTesting_AllowsReinitialization(t *testing.T) {
	defer ResetForTesting()

	// Get command
	cmd1 := GetConfigCmd()
	assert.NotNil(t, cmd1)

	// Reset
	ResetForTesting()

	// Get again - should reinitialize
	cmd2 := GetConfigCmd()
	assert.NotNil(t, cmd2)
	assert.NotSame(t, cmd1, cmd2)
}

func TestAllSubcommands_HaveRunE(t *testing.T) {
	defer ResetForTesting()

	cmds := []func() *cobra.Command{
		GetInitCmd,
		GetGetCmd,
		GetSetCmd,
		GetUnsetCmd,
		GetPathCmd,
		GetCheckCmd,
		GetEditCmd,
	}

	for _, getCmd := range cmds {
		cmd := getCmd()
		assert.NotNil(t, cmd.RunE, "Command %s should have RunE", cmd.Use)
	}
}

func TestConfigCmd_HasCorrectStructure(t *testing.T) {
	defer ResetForTesting()

	cmd := GetConfigCmd()

	assert.Equal(t, "config", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.NotNil(t, cmd.Commands())
}
