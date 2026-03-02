package loan

import (
	"github.com/spf13/cobra"
)

var loanCmd *cobra.Command

// GetLoanCmd returns the loan command, initializing it if necessary.
func GetLoanCmd() *cobra.Command {
	if loanCmd != nil {
		return loanCmd
	}

	loanCmd = &cobra.Command{
		Use:   "loan <item-selector>... --to <name>",
		Short: "Mark items as loaned to someone",
		Long: `Mark one or more items as loaned to someone by moving them to the Loaned system location.

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
  wherehouse loan aB3xK9mPqR --to Bob`,
		Args: cobra.MinimumNArgs(1),
		RunE: runLoanItem,
	}

	// Required flags
	loanCmd.Flags().String("to", "", "person receiving the loan (required, free text)")
	_ = loanCmd.MarkFlagRequired("to")

	// Event metadata flag
	loanCmd.Flags().StringP("note", "n", "", "optional note for context")

	return loanCmd
}
