package move

import (
	"github.com/spf13/cobra"
)

var moveCmd *cobra.Command

// GetMoveCmd returns the move command, initializing it if necessary.
func GetMoveCmd() *cobra.Command {
	if moveCmd != nil {
		return moveCmd
	}

	moveCmd = &cobra.Command{
		Use:   "move <item-selector>... --to <location>",
		Short: "Move items to a different location",
		Long: `Move one or more items to a different location.

Selector types:
  - UUID: 550e8400-e29b-41d4-a716-446655440001 (exact ID)
  - LOCATION:ITEM: garage:socket (both canonical names, filters by location)
  - Canonical name: "10mm socket" (must match exactly 1 item)

System location restrictions:
  - Cannot move FROM system locations (Missing, Borrowed)
  - Cannot move TO system locations (Missing, Borrowed)
  - Use dedicated commands (found, return, borrow) for these operations

Move types:
  - Default: Permanent move (rehome)
  - --temp: Temporary move (preserves origin for return)

Project association:
  - Default: Clear project association (--clear-project is implicit)
  - --project <id>: Associate with project
  - --keep-project: Preserve current project association

Examples:
  wherehouse move garage:socket --to toolbox
  wherehouse move 550e8400-e29b-41d4-a716-446655440001 --to desk
  wherehouse move "10mm socket" --to garage --temp
  wherehouse move wrench screwdriver --to toolbox --keep-project
  wherehouse move 550e8400-e29b-41d4-a716-446655440001 --to shed --project dinner-prep --note "need for project"`,
		Args: cobra.MinimumNArgs(1),
		RunE: runMoveItem,
	}

	// Required flags
	moveCmd.Flags().StringP("to", "t", "", "destination location (required)")
	_ = moveCmd.MarkFlagRequired("to")

	// Move type flags
	moveCmd.Flags().Bool("temp", false, "temporary move (preserve origin for return)")

	// Project association flags
	moveCmd.Flags().String("project", "", "associate with project")
	moveCmd.Flags().Bool("keep-project", false, "preserve current project association")
	moveCmd.Flags().Bool("clear-project", false, "clear project association (default behavior)")
	moveCmd.MarkFlagsMutuallyExclusive("project", "keep-project")
	moveCmd.MarkFlagsMutuallyExclusive("project", "clear-project")
	moveCmd.MarkFlagsMutuallyExclusive("keep-project", "clear-project")

	// Event metadata
	moveCmd.Flags().StringP("note", "n", "", "optional note for event")

	return moveCmd
}
