package initialize

import "github.com/spf13/cobra"

// NewInitializeCmd returns the initialize command group. The parent command shows help only;
// action is delegated to subcommands (e.g., `initialize database`).
func NewInitializeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "initialize",
		Aliases: []string{"init"},
		Short:   "Initialize wherehouse resources",
		Long: `Initialize wherehouse resources for first-time setup.

Examples:
  wherehouse initialize database           # Create the database
  wherehouse initialize database --force   # Reinitialize (backs up existing database)`,
		// No RunE: displays help when called without a subcommand.
	}

	cmd.AddCommand(NewInitializeDatabaseCmd())

	return cmd
}
