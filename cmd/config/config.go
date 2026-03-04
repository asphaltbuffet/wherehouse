// Package config implements configuration management commands for wherehouse.
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
		Long: `Manage wherehouse configuration files and settings.

Wherehouse supports both global and local configuration files:
  Global: ~/.config/wherehouse/wherehouse.toml
  Local:  ./wherehouse.toml

Local configuration overrides global configuration.

Examples:
  wherehouse config init                             # Create global config file
  wherehouse config init --local                     # Create local config file
  wherehouse config get                              # Show all configuration values
  wherehouse config get database.path                # Show specific configuration value
  wherehouse config set database.path /custom/path   # Set configuration value
  wherehouse config check                            # Validate configuration`,
	}

	// Register all subcommands using their Get* functions
	configCmd.AddCommand(GetInitCmd())
	configCmd.AddCommand(GetGetCmd())
	configCmd.AddCommand(GetSetCmd())
	configCmd.AddCommand(GetPathCmd())
	configCmd.AddCommand(GetCheckCmd())
	configCmd.AddCommand(GetEditCmd())

	return configCmd
}

// ResetForTesting resets all command variables to nil.
// This allows tests to reinitialize commands with fresh state.
func ResetForTesting() {
	configCmd = nil
	initCmd = nil
	getCmd = nil
	setCmd = nil
	pathCmd = nil
	checkCmd = nil
	editCmd = nil
}
