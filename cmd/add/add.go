package add

import (
	"github.com/spf13/cobra"
)

const addLongDescription = `Add new items and locations.

Examples:
  wherehouse add location <name> --in <location>  # Add a new location
  wherehouse add item <name> --in <location>       # Add a new item`

// NewAddCmd returns an add command with the given subcommands registered.
// This is the canonical constructor; both subcommands (item, location) are
// always registered since they delegate database access to internal/cli.
func NewAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add items and locations",
		Long:  addLongDescription,
	}

	// Register subcommands
	cmd.AddCommand(NewAddItemCmd())
	cmd.AddCommand(NewAddLocationCmd())

	return cmd
}
