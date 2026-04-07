package add

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
)

var itemCmd *cobra.Command

// GetItemCmd returns the item subcommand, initializing it if necessary.
func GetItemCmd() *cobra.Command {
	if itemCmd != nil {
		return itemCmd
	}

	itemCmd = &cobra.Command{
		Use:   "item ITEM_NAME [ITEM_NAME...]",
		Short: "Add one or more items to a location",
		Long: `Add one or more items to a specified location.

Each item name becomes a separate item with a unique ID. Multiple identical
names will create separate items (useful for bulk additions like "nail" "nail" "nail").

The --in flag specifies the location where items are stored. Location can be
specified by canonical name or ID.

Examples:
  wherehouse add item "10mm Socket" --in Garage
  wherehouse add item "Phillips Screwdriver" "Flathead Screwdriver" --in Toolbox
  wherehouse add item "Nail" "Nail" "Nail" --in "Hardware Bin"`,
		Args: cobra.MinimumNArgs(1),
		RunE: runAddItem,
	}

	itemCmd.Flags().StringP("in", "i", "", "Location where items are stored (REQUIRED)")
	if err := itemCmd.MarkFlagRequired("in"); err != nil {
		panic(fmt.Sprintf("failed to mark 'in' flag as required: %v", err))
	}
	if err := itemCmd.RegisterFlagCompletionFunc(
		"in",
		func(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			return cli.LocationCompletions(cmd.Context())
		},
	); err != nil {
		panic(fmt.Sprintf("failed to register 'in' flag completion: %v", err))
	}

	return itemCmd
}

// runAddItem implements the item addition logic.
func runAddItem(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Get required --in flag
	locationInput, _ := cmd.Flags().GetString("in")

	err := cli.AddItems(ctx, args, locationInput)
	if err != nil {
		return err
	}

	return nil
}
