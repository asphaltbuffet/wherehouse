package move

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
)

const moveLongDescription = `Move one or more items to a different location.

Selector types:
  - ID: aB3xK9mPqR (exact ID)
  - LOCATION:ITEM: garage:socket (both canonical names, filters by location)
  - Canonical name: "10mm socket" (must match exactly 1 item)

System location restrictions:
  - Cannot move FROM system locations (Missing, Borrowed)
  - Cannot move TO system locations (Missing, Borrowed)
  - Use dedicated commands (found, return, borrow) for these operations

Move types:
  - Default: Permanent move (rehome)
  - --temp: Temporary move (preserves origin for return)

Examples:
  wherehouse move garage:socket --to toolbox
  wherehouse move aB3xK9mPqR --to desk
  wherehouse move "10mm socket" --to garage --temp
  wherehouse move wrench screwdriver --to toolbox`

// NewMoveCmd returns a move command that opens the database from context
// configuration at runtime.
func NewMoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "move <item-selector>...",
		Short: "Move items to a different location",
		Long:  moveLongDescription,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := cli.OpenDatabase(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to open database: %w", err)
			}
			defer func() {
				if closeErr := db.Close(); closeErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to close database: %v\n", closeErr)
				}
			}()
			return runMoveItemCore(cmd, args, db)
		},
	}

	// Required flags
	cmd.Flags().StringP("to", "t", "", "destination location (required)")
	_ = cmd.MarkFlagRequired("to")
	_ = cmd.RegisterFlagCompletionFunc(
		"to",
		func(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			return cli.LocationCompletions(cmd.Context())
		},
	)

	// Move type flags
	cmd.Flags().Bool("temp", false, "temporary move (preserve origin for return)")

	// Event metadata
	cmd.Flags().StringP("note", "n", "", "optional note for event")

	return cmd
}
