package lost

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// Result represents the result of marking an item as lost.
type Result struct {
	ItemID           string `json:"item_id"`
	DisplayName      string `json:"display_name"`
	PreviousLocation string `json:"previous_location"`
	EventID          int64  `json:"event_id"`
}

// runLostItem is the main entry point for the lost command.
func runLostItem(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	selector := args[0]

	// Parse flags
	note, _ := cmd.Flags().GetString("note")

	// Open database
	db, err := openDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Get actor user ID
	actorUserID := cli.GetActorUserID(ctx)

	// Resolve item selector
	itemID, err := resolveItemSelector(ctx, db, selector)
	if err != nil {
		return fmt.Errorf("failed to resolve %q: %w", selector, err)
	}

	// Mark item as lost
	result, err := markItemLost(ctx, db, itemID, actorUserID, note)
	if err != nil {
		return fmt.Errorf("failed to mark item as lost: %w", err)
	}

	// Set up output writer
	cfg := cli.MustGetConfig(cmd.Context())
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	// Output result
	if cfg.IsJSON() {
		if jsonErr := out.JSON(result); jsonErr != nil {
			return fmt.Errorf("failed to encode JSON output: %w", jsonErr)
		}
		return nil
	}

	// Human-readable output
	out.Success(fmt.Sprintf("Marked item %q as missing (was in %s)",
		result.DisplayName, result.PreviousLocation))

	return nil
}

// markItemLost performs the core logic of marking an item as lost.
// Creates an item.missing event and updates the projection.
func markItemLost(
	ctx context.Context,
	db *database.Database,
	itemID, actorUserID, note string,
) (*Result, error) {
	// Get current item state
	item, err := db.GetItem(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("item not found: %w", err)
	}

	// Get current location
	location, err := db.GetLocation(ctx, item.LocationID)
	if err != nil {
		return nil, fmt.Errorf("location not found: %w", err)
	}

	// ERROR if already missing (prevents duplicate events)
	if location.IsSystem && location.CanonicalName == "missing" {
		return nil, fmt.Errorf("item %q is already marked as missing", item.DisplayName)
	}

	// Borrowed items CAN be marked as missing (no special handling needed)

	// Validate from_location matches projection (CRITICAL for event-sourcing)
	if validateErr := db.ValidateFromLocation(ctx, itemID, item.LocationID); validateErr != nil {
		return nil, fmt.Errorf("projection validation failed: %w", validateErr)
	}

	// Build event payload
	payload := map[string]any{
		"item_id":              itemID,
		"previous_location_id": item.LocationID,
	}

	// Insert event and update projection atomically
	eventID, err := db.AppendEvent(ctx, "item.missing", actorUserID, payload, note)
	if err != nil {
		return nil, fmt.Errorf("failed to create marked_missing event: %w", err)
	}

	// Build result
	result := &Result{
		ItemID:           itemID,
		DisplayName:      item.DisplayName,
		PreviousLocation: location.DisplayName,
		EventID:          eventID,
	}

	return result, nil
}
