package lost

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

const lostLongDescription = `Mark an item as lost or missing by moving it to the Missing system location.

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
  wherehouse lost aB3xK9mPqR`

// NewLostCmd returns a lost command that uses the provided db for all database
// operations. The caller retains no reference to db after this call; the
// returned command's RunE closes it via defer before returning.
func NewLostCmd(db lostDB) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lost <item-selector>",
		Short: "Mark an item as lost/missing",
		Long:  lostLongDescription,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() {
				if closeErr := db.Close(); closeErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to close database: %v\n", closeErr)
				}
			}()
			return runLostItem(cmd, args, db)
		},
	}

	registerLostFlags(cmd)
	return cmd
}

// NewDefaultLostCmd returns a lost command that opens the database from context
// configuration at runtime. This is the production entry point registered with
// the root command.
func NewDefaultLostCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lost <item-selector>",
		Short: "Mark an item as lost/missing",
		Long:  lostLongDescription,
		Args:  cobra.ExactArgs(1),
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
			return runLostItem(cmd, args, db)
		},
	}

	registerLostFlags(cmd)
	return cmd
}

// registerLostFlags attaches all lost-specific flags to cmd.
// Called by both NewLostCmd and NewDefaultLostCmd to ensure identical flag sets.
func registerLostFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("note", "n", "", "optional note explaining circumstances")
}

// GetLostCmd returns the lost command using the default database.
//
// Deprecated: Use NewDefaultLostCmd instead.
func GetLostCmd() *cobra.Command {
	return NewDefaultLostCmd()
}

// ensure *database.Database satisfies lostDB at compile time.
var _ lostDB = (*database.Database)(nil)
