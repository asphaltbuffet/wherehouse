package lost

import (
	"github.com/spf13/cobra"
)

var lostCmd *cobra.Command

// GetLostCmd returns the lost command, initializing it if necessary.
func GetLostCmd() *cobra.Command {
	if lostCmd != nil {
		return lostCmd
	}

	lostCmd = &cobra.Command{
		Use:   "lost <item-selector>",
		Short: "Mark an item as lost/missing",
		Long: `Mark an item as lost or missing by moving it to the Missing system location.

The item's home location is preserved so it can be returned when found.

Selector types:
  - ID: aB3xK9mPqR (exact ID)
  - LOCATION:ITEM: garage:socket (both canonical names, filters by location)
  - Canonical name: "10mm socket" (must match exactly 1 item)

Validation rules:
  - Item must NOT already be in Missing location (returns error)
  - Borrowed items CAN be marked as missing (borrowed → missing is valid)

Examples:
  wherehouse lost garage:socket
  wherehouse lost "10mm socket" --note "checked toolbox"
  wherehouse lost aB3xK9mPqR`,
		Args: cobra.ExactArgs(1),
		RunE: runLostItem,
	}

	// Event metadata flag
	lostCmd.Flags().StringP("note", "n", "", "optional note explaining circumstances")

	return lostCmd
}
