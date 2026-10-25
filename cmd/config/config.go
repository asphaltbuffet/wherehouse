package config

import (
	"github.com/spf13/cobra"
)

// AllowInvalidConfigAnnotation marks a command subtree that must run even
// when the configuration is invalid, so users can inspect and repair it.
const AllowInvalidConfigAnnotation = "wherehouse/allow-invalid-config"

// NewConfigCmd returns the config command, initializing it if necessary.
func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "config",
		Short:       "Manage wherehouse configuration",
		Long:        longDesc,
		Annotations: map[string]string{AllowInvalidConfigAnnotation: "true"},
	}

	// Register all subcommands using their Get* functions
	cmd.AddCommand(NewInitCmd())
	cmd.AddCommand(NewPathCmd())
	cmd.AddCommand(NewCheckCmd())

	return cmd
}
