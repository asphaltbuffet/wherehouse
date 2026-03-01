package move

import (
	"fmt"

	"github.com/spf13/cobra"
)

const moveLongDescription = `Move one or more items to a different location.

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
  wherehouse move 550e8400-e29b-41d4-a716-446655440001 --to shed --project dinner-prep --note "need for project"`

// NewMoveCmd returns a move command that uses the provided db for all database
// operations. The caller retains no reference to db after this call; the
// returned command's RunE closes it via defer before returning.
func NewMoveCmd(db moveDB) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "move <item-selector>... --to <location>",
		Short: "Move items to a different location",
		Long:  moveLongDescription,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() {
				if closeErr := db.Close(); closeErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to close database: %v\n", closeErr)
				}
			}()
			return runMoveItemCore(cmd, args, db)
		},
	}

	registerMoveFlags(cmd)
	return cmd
}

// NewDefaultMoveCmd returns a move command that opens the database from context
// configuration at runtime. This is the production entry point registered with
// the root command.
func NewDefaultMoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "move <item-selector>... --to <location>",
		Short: "Move items to a different location",
		Long:  moveLongDescription,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDatabase(cmd.Context())
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

	registerMoveFlags(cmd)
	return cmd
}

// registerMoveFlags attaches all move-specific flags to cmd.
// Called by both NewMoveCmd and NewDefaultMoveCmd to ensure identical flag sets.
func registerMoveFlags(cmd *cobra.Command) {
	// Required flags
	cmd.Flags().StringP("to", "t", "", "destination location (required)")
	_ = cmd.MarkFlagRequired("to")

	// Move type flags
	cmd.Flags().Bool("temp", false, "temporary move (preserve origin for return)")

	// Project association flags
	cmd.Flags().String("project", "", "associate with project")
	cmd.Flags().Bool("keep-project", false, "preserve current project association")
	cmd.Flags().Bool("clear-project", false, "clear project association (default behavior)")
	cmd.MarkFlagsMutuallyExclusive("project", "keep-project")
	cmd.MarkFlagsMutuallyExclusive("project", "clear-project")
	cmd.MarkFlagsMutuallyExclusive("keep-project", "clear-project")

	// Event metadata
	cmd.Flags().StringP("note", "n", "", "optional note for event")
}
