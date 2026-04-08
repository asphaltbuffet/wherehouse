package config

import (
	"github.com/spf13/cobra"
)

var configCmd *cobra.Command

// GetConfigCmd returns the config command, initializing it if necessary.
// This is the parent command for all config subcommands (init, get, set, etc.).
func GetConfigCmd() *cobra.Command {
	if configCmd != nil {
		return configCmd
	}

	configCmd = &cobra.Command{
		Use:   "config",
		Short: "Manage wherehouse configuration",
		Long:  longDesc,
	}

	// Register all subcommands using their Get* functions
	configCmd.AddCommand(GetInitCmd())
	configCmd.AddCommand(GetPathCmd())
	configCmd.AddCommand(GetCheckCmd())

	return configCmd
}

// ResetForTesting resets all command variables to nil.
// This allows tests to reinitialize commands with fresh state.
func ResetForTesting() {
	configCmd = nil
	initCmd = nil
	pathCmd = nil
	checkCmd = nil
}
