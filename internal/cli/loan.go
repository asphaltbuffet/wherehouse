package cli

import (
	"context"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// LoanItemResult holds the outcome of a single item loan operation.
type LoanItemResult struct {
	ItemID           string
	DisplayName      string
	LoanedTo         string
	EventID          int64
	WasReLoaned      bool
	PreviousLoanedTo string
	PreviousLocation string
}

// LoanItemOptions configures optional behavior for LoanItem.
type LoanItemOptions struct {
	// Borrower is the name of the person/entity borrowing the item. Required.
	Borrower string
	// Note is an optional free-text note attached to the event.
	Note string
}

// loanDB is the database interface required by LoanItem.
// *database.Database satisfies this interface.
type loanDB interface {
	LocationItemQuerier
	GetItemLoanedInfo(ctx context.Context, itemID string) (*database.LoanedInfo, error)
	ValidateItemLoaned(ctx context.Context, itemID, fromLocationID, loanedTo string) error
	AppendEvent(
		ctx context.Context,
		eventType database.EventType,
		actorUserID string,
		payload any,
		note string,
	) (int64, error)
}

// LoanItem records an item as loaned to a borrower.
// Validates item state before creating the ItemLoaned event.
// The itemSelector may be an ID, a LOCATION:ITEM selector, or a canonical name.
// actorUserID is the user performing the action.
func LoanItem(
	ctx context.Context,
	db loanDB,
	itemSelector string,
	actorUserID string,
	opts LoanItemOptions,
) (*LoanItemResult, error) {
	// Resolve item selector to an item ID
	itemID, err := ResolveItemSelector(ctx, db, itemSelector, "wherehouse loan")
	if err != nil {
		return nil, fmt.Errorf("failed to resolve %q: %w", itemSelector, err)
	}

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

	// Validate using database layer validation (defense-in-depth).
	// Checks: item exists, from_location matches projection, loaned_to is non-empty.
	if validateErr := db.ValidateItemLoaned(ctx, itemID, item.LocationID, opts.Borrower); validateErr != nil {
		return nil, fmt.Errorf("projection validation failed: %w", validateErr)
	}

	// Build event payload
	payload := map[string]any{
		"item_id":          itemID,
		"from_location_id": item.LocationID,
		"loaned_to":        opts.Borrower,
	}

	// Insert event and update projection atomically
	eventID, err := db.AppendEvent(ctx, database.ItemLoanedEvent, actorUserID, payload, opts.Note)
	if err != nil {
		return nil, fmt.Errorf("failed to create loaned event: %w", err)
	}

	// Build result
	result := &LoanItemResult{
		ItemID:           itemID,
		DisplayName:      item.DisplayName,
		LoanedTo:         opts.Borrower,
		EventID:          eventID,
		WasReLoaned:      wasReLoaned,
		PreviousLocation: fromLocation.DisplayName,
	}
	if wasReLoaned {
		result.PreviousLoanedTo = previousLoanedTo
	}

	return result, nil
}
