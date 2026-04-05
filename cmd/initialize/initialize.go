package initialize

import "github.com/spf13/cobra"

var initializeCmd *cobra.Command

// GetInitializeCmd returns the initialize command group. The parent command shows help only;
// action is delegated to subcommands (e.g., `initialize database`).
func GetInitializeCmd() *cobra.Command {
	if initializeCmd != nil {
		return initializeCmd
	}

	initializeCmd = &cobra.Command{
		Use:     "initialize",
		Aliases: []string{"init"},
		Short:   "Initialize wherehouse resources",
		Long: `Initialize wherehouse resources for first-time setup.

Examples:
  wherehouse initialize database           # Create the database
  wherehouse initialize database --force   # Reinitialize (backs up existing database)`,
		// No RunE: displays help when called without a subcommand.
	}

	initializeCmd.AddCommand(GetDatabaseCmd())

	return initializeCmd
}
