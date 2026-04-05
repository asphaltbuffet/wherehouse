package add

import (
	"github.com/spf13/cobra"
)

var addCmd *cobra.Command

// GetAddCmd returns the add command, initializing it if necessary.
// This is the parent command for all add subcommands (item, location, etc.).
func GetAddCmd() *cobra.Command {
	if addCmd != nil {
		return addCmd
	}

	addCmd = &cobra.Command{
		Use:   "add",
		Short: "Add items and locations",
		Long: `Add new items and locations.

Examples:
  wherehouse add location <name> --in <location> Add a new location
  wherehouse add item <name> --in <location>              Add a new item`,
	}

	// Register subcommands
	addCmd.AddCommand(GetItemCmd())
	addCmd.AddCommand(GetLocationCmd())

	return addCmd
}
