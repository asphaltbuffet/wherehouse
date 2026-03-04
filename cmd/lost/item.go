package lost

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
)

// Result represents the result of marking an item as lost.
type Result struct {
	ItemID           string `json:"item_id"`
	DisplayName      string `json:"display_name"`
	PreviousLocation string `json:"previous_location"`
	EventID          int64  `json:"event_id"`
}

// runLostItem is the main entry point for the lost command.
func runLostItem(cmd *cobra.Command, args []string, db lostDB) error {
	ctx := cmd.Context()
	selector := args[0]

	// Parse flags
	note, _ := cmd.Flags().GetString("note")

	// Get actor user ID and set up output writer
	actorUserID := cli.GetActorUserID(ctx)
	cfg := cli.MustGetConfig(cmd.Context())
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	result, err := markItemLost(ctx, db, selector, actorUserID, note)
	if err != nil {
		return fmt.Errorf("failed to mark item as lost: %w", err)
	}

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

// resolveItemSelector converts a selector (name or location:name) to item ID.
// Returns error if selector is ambiguous or not found.
func resolveItemSelector(ctx context.Context, db lostDB, selector string) (string, error) {
	return cli.ResolveItemSelector(ctx, db, selector, "wherehouse lost")
}

// markItemLost marks an item as lost by delegating to cli.LostItem.
// The itemIDOrSelector may be an item ID, LOCATION:ITEM selector, or canonical name.
func markItemLost(
	ctx context.Context,
	db lostDB,
	itemIDOrSelector, actorUserID, note string,
) (*Result, error) {
	opts := cli.LostItemOptions{
		Note: note,
	}

	lostResult, err := cli.LostItem(ctx, db, itemIDOrSelector, actorUserID, opts)
	if err != nil {
		return nil, err
	}

	return &Result{
		ItemID:           lostResult.ItemID,
		DisplayName:      lostResult.DisplayName,
		PreviousLocation: lostResult.PreviousLocation,
		EventID:          lostResult.EventID,
	}, nil
}
