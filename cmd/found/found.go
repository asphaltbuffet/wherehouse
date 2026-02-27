package found

import (
	"github.com/spf13/cobra"
)

var foundCmd *cobra.Command

// GetFoundCmd returns the found command, initializing it if necessary.
func GetFoundCmd() *cobra.Command {
	if foundCmd != nil {
		return foundCmd
	}

	foundCmd = &cobra.Command{
		Use:   "found <item-selector>... --in <location>",
		Short: "Record that a lost or missing item has been found",
		Long: `Record that one or more items have been found at a specific location.

The item's home location is NOT changed by default. Use --return to also
move the item back to its home location immediately.

Selector types:
  - UUID:          550e8400-e29b-41d4-a716-446655440001
  - LOCATION:ITEM: garage:socket (both canonical names)
  - Canonical:     "10mm socket" (must match exactly 1 item)

Examples:
  wherehouse found "10mm socket" --in garage
  wherehouse found "10mm socket" --in garage --return
  wherehouse found garage:screwdriver --in shed --note "behind workbench"`,
		Args: cobra.MinimumNArgs(1),
		RunE: runFoundItem,
	}

	foundCmd.Flags().StringP("in", "i", "", "location where item was found (required)")
	_ = foundCmd.MarkFlagRequired("in")

	foundCmd.Flags().BoolP("return", "r", false, "also return item to its home location")
	foundCmd.Flags().StringP("note", "n", "", "optional note for event")

	return foundCmd
}
