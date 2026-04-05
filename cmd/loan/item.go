package loan

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
)

// Result represents the result of a single item loan operation.
type Result struct {
	ItemID           string `json:"item_id"`
	DisplayName      string `json:"display_name"`
	LoanedTo         string `json:"loaned_to"`
	EventID          int64  `json:"event_id"`
	WasReLoaned      bool   `json:"was_re_loaned"`
	PreviousLoanedTo string `json:"previous_loaned_to,omitempty"`
	PreviousLocation string `json:"previous_location"`
}

// runLoanItem is the main entry point for the loan command.
func runLoanItem(cmd *cobra.Command, args []string, db loanDB) error {
	ctx := cmd.Context()

	// Parse flags
	loanedTo, _ := cmd.Flags().GetString("to")
	note, _ := cmd.Flags().GetString("note")

	// Validate loanedTo is not empty (trim whitespace)
	loanedTo = strings.TrimSpace(loanedTo)
	if loanedTo == "" {
		return errors.New("--to flag cannot be empty")
	}

	// Get actor user ID and set up output writer
	actorUserID := cli.GetActorUserID(ctx)
	cfg := cli.MustGetConfig(cmd.Context())
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	opts := cli.LoanItemOptions{
		Borrower: loanedTo,
		Note:     note,
	}

	var results []Result

	for _, selector := range args {
		loanResult, loanErr := cli.LoanItem(ctx, db, selector, actorUserID, opts)
		if loanErr != nil {
			return fmt.Errorf("failed to loan %q: %w", selector, loanErr)
		}

		results = append(results, Result{
			ItemID:           loanResult.ItemID,
			DisplayName:      loanResult.DisplayName,
			LoanedTo:         loanResult.LoanedTo,
			EventID:          loanResult.EventID,
			WasReLoaned:      loanResult.WasReLoaned,
			PreviousLoanedTo: loanResult.PreviousLoanedTo,
			PreviousLocation: loanResult.PreviousLocation,
		})

		// Print success message (unless quiet or JSON mode)
		if !cfg.IsJSON() {
			if loanResult.WasReLoaned {
				out.Success(fmt.Sprintf("Loaned item %q to %s (previously loaned to %s)",
					loanResult.DisplayName, loanResult.LoanedTo, loanResult.PreviousLoanedTo))
			} else {
				out.Success(fmt.Sprintf("Loaned item %q to %s",
					loanResult.DisplayName, loanResult.LoanedTo))
			}
		}
	}

	// Output JSON if requested
	if cfg.IsJSON() {
		output := map[string]any{
			"success":      true,
			"items_loaned": results,
		}
		if jsonErr := out.JSON(output); jsonErr != nil {
			return fmt.Errorf("failed to encode JSON output: %w", jsonErr)
		}
	}

	return nil
}
