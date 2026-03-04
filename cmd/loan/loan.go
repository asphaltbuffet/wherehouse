package loan

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

const loanLongDescription = `Mark one or more items as loaned to someone by moving them to the Loaned system location.

The item's home location is preserved and the recipient's name is recorded in the event log.
Items can be loaned from ANY location, including Missing, Borrowed, and even Loaned (re-loaning).

Selector types:
  - ID: aB3xK9mPqR (exact ID)
  - LOCATION:ITEM: garage:socket (both canonical names, filters by location)
  - Canonical name: "10mm socket" (must match exactly 1 item)

Re-loaning:
  - Items already in Loaned location CAN be loaned again (to a different person)
  - The new recipient's name replaces the old one
  - Previous loan information is preserved in event history

Validation rules:
  - --to flag is REQUIRED and cannot be empty
  - Items can be loaned from any location (including system locations)

Examples:
  wherehouse loan garage:socket --to "Bob Smith"
  wherehouse loan "10mm socket" --to alice@example.com
  wherehouse loan wrench screwdriver --to "Friend's name" --note "for weekend project"
  wherehouse loan aB3xK9mPqR --to Bob`

// NewLoanCmd returns a loan command that uses the provided db for all database
// operations. The caller retains no reference to db after this call; the
// returned command's RunE closes it via defer before returning.
func NewLoanCmd(db loanDB) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "loan <item-selector>... --to <name>",
		Short: "Mark items as loaned to someone",
		Long:  loanLongDescription,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() {
				if closeErr := db.Close(); closeErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to close database: %v\n", closeErr)
				}
			}()
			return runLoanItem(cmd, args, db)
		},
	}

	registerLoanFlags(cmd)
	return cmd
}

// NewDefaultLoanCmd returns a loan command that opens the database from context
// configuration at runtime. This is the production entry point registered with
// the root command.
func NewDefaultLoanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "loan <item-selector>... --to <name>",
		Short: "Mark items as loaned to someone",
		Long:  loanLongDescription,
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
			return runLoanItem(cmd, args, db)
		},
	}

	registerLoanFlags(cmd)
	return cmd
}

// registerLoanFlags attaches all loan-specific flags to cmd.
// Called by both NewLoanCmd and NewDefaultLoanCmd to ensure identical flag sets.
func registerLoanFlags(cmd *cobra.Command) {
	cmd.Flags().String("to", "", "person receiving the loan (required, free text)")
	_ = cmd.MarkFlagRequired("to")

	cmd.Flags().StringP("note", "n", "", "optional note for context")
}

// GetLoanCmd returns the loan command using the default database.
//
// Deprecated: Use NewDefaultLoanCmd instead.
func GetLoanCmd() *cobra.Command {
	return NewDefaultLoanCmd()
}

// ensure *database.Database satisfies loanDB at compile time.
var _ loanDB = (*database.Database)(nil)
