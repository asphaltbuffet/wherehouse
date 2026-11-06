package config

import (
	"github.com/spf13/cobra"
)

// AllowInvalidConfigAnnotation marks a command subtree that must run even
// when the configuration is invalid, so users can inspect and repair it.
const AllowInvalidConfigAnnotation = "wherehouse/allow-invalid-config"

// AllowMissingConfigAnnotation marks a command that expects the --config file
// not to exist yet (it creates it), so a missing file loads defaults silently.
const AllowMissingConfigAnnotation = "wherehouse/allow-missing-config"

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
