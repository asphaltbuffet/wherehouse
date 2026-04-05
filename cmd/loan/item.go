package loan

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/database"
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
func runLoanItem(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Parse flags
	loanedTo, _ := cmd.Flags().GetString("to")
	note, _ := cmd.Flags().GetString("note")

	// Validate loanedTo is not empty (trim whitespace)
	loanedTo = strings.TrimSpace(loanedTo)
	if loanedTo == "" {
		return errors.New("--to flag cannot be empty")
	}

	// Open database
	db, err := openDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Get actor user ID
	actorUserID := cli.GetActorUserID(ctx)

	// Set up output writer
	cfg := cli.MustGetConfig(cmd.Context())
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	// PHASE 1: Resolve and validate ALL selectors (fail-fast)
	type resolvedItem struct {
		selector string
		itemID   string
	}
	var resolved []resolvedItem

	for _, selector := range args {
		// Resolve item selector
		itemID, itemErr := resolveItemSelector(ctx, db, selector)
		if itemErr != nil {
			return fmt.Errorf("failed to resolve %q: %w", selector, itemErr)
		}

		// Validate item can be loaned (basic validation)
		if validateErr := validateItemForLoan(ctx, db, itemID, loanedTo); validateErr != nil {
			return fmt.Errorf("failed to validate %q: %w", selector, validateErr)
		}

		resolved = append(resolved, resolvedItem{selector: selector, itemID: itemID})
	}

	// PHASE 2: Create events for all validated items
	var results []Result

	for _, r := range resolved {
		// Perform loan (creates event)
		result, loanErr := loanItem(ctx, db, r.itemID, loanedTo, actorUserID, note)
		if loanErr != nil {
			return fmt.Errorf("failed to loan %q: %w", r.selector, loanErr)
		}

		results = append(results, *result)

		// Print success message (unless quiet or JSON mode)
		if !cfg.IsJSON() {
			if result.WasReLoaned {
				out.Success(fmt.Sprintf("Loaned item %q to %s (previously loaned to %s)",
					result.DisplayName, result.LoanedTo, result.PreviousLoanedTo))
			} else {
				out.Success(fmt.Sprintf("Loaned item %q to %s",
					result.DisplayName, result.LoanedTo))
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

// validateItemForLoan validates an item can be loaned without creating events.
// This is used in Phase 1 of batch processing.
func validateItemForLoan(ctx context.Context, db *database.Database, itemID, loanedTo string) error {
	// Get current item state
	item, err := db.GetItem(ctx, itemID)
	if err != nil {
		return fmt.Errorf("item not found: %w", err)
	}

	// Validate using database layer validation (defense-in-depth)
	// This checks: item exists, from_location matches projection, loaned_to is non-empty
	if validateErr := db.ValidateItemLoaned(ctx, itemID, item.LocationID, loanedTo); validateErr != nil {
		return fmt.Errorf("validation failed: %w", validateErr)
	}

	return nil
}

// loanItem performs a single item loan operation.
func loanItem(
	ctx context.Context,
	db *database.Database,
	itemID, loanedTo, actorUserID, note string,
) (*Result, error) {
	// Get current item state
	item, err := db.GetItem(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("item not found: %w", err)
	}

	// Get current location
	fromLocation, err := db.GetLocation(ctx, item.LocationID)
	if err != nil {
		return nil, fmt.Errorf("from location not found: %w", err)
	}

	// Check if already loaned (for informative output)
	var previousLoanedTo string
	wasReLoaned := false
	if fromLocation.IsSystem && fromLocation.CanonicalName == "loaned" {
		wasReLoaned = true
		// Try to get previous loaned_to from event log
		if loanedInfo, loanErr := db.GetItemLoanedInfo(ctx, itemID); loanErr == nil {
			previousLoanedTo = loanedInfo.LoanedTo
		}
	}

	// Validate using database layer validation (defense-in-depth)
	// This checks: item exists, from_location matches projection, loaned_to is non-empty
	if validateErr := db.ValidateItemLoaned(ctx, itemID, item.LocationID, loanedTo); validateErr != nil {
		return nil, fmt.Errorf("projection validation failed: %w", validateErr)
	}

	// Build event payload
	payload := map[string]any{
		"item_id":          itemID,
		"from_location_id": item.LocationID,
		"loaned_to":        loanedTo,
	}

	// Insert event and update projection atomically
	eventID, err := db.AppendEvent(ctx, "item.loaned", actorUserID, payload, note)
	if err != nil {
		return nil, fmt.Errorf("failed to create loaned event: %w", err)
	}

	// Build result
	result := &Result{
		ItemID:           itemID,
		DisplayName:      item.DisplayName,
		LoanedTo:         loanedTo,
		EventID:          eventID,
		WasReLoaned:      wasReLoaned,
		PreviousLocation: fromLocation.DisplayName,
	}
	if wasReLoaned {
		result.PreviousLoanedTo = previousLoanedTo
	}

	return result, nil
}
