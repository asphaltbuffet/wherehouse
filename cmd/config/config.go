package config

import (
	"github.com/spf13/cobra"
)

// NewConfigCmd returns the config command, initializing it if necessary.
func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage wherehouse configuration",
		Long:  longDesc,
	}

	// Register all subcommands using their Get* functions
	cmd.AddCommand(NewInitCmd())
	cmd.AddCommand(NewPathCmd())
	cmd.AddCommand(NewCheckCmd())

	return cmd
}
