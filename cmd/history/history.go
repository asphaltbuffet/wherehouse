package history

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

var historyCmd *cobra.Command

// GetHistoryCmd returns the history command, initializing it if necessary.
func GetHistoryCmd() *cobra.Command {
	if historyCmd != nil {
		return historyCmd
	}

	historyCmd = &cobra.Command{
		Use:   "history <item-selector>",
		Short: "Show event history for an item",
		Long: `Display event history for a specific item (newest first by default).

Item selector can be:
  - Canonical name: "10mm_socket"
  - Location-scoped: "garage:10mm_socket"
  - UUID: --id <uuid>

Examples:
  wherehouse history 10mm_socket
  wherehouse history toolbox:screwdriver -n 10
  wherehouse history --id abc-123-def --since "2 weeks ago"
  wherehouse history socket --since 2026-01-15 --oldest-first`,
		Args: cobra.MaximumNArgs(1), // 0 args if using --id
		RunE: runHistory,
	}

	historyCmd.Flags().StringP("id", "i", "", "Item UUID (alternative to name selector)")
	historyCmd.Flags().IntP("limit", "n", 0, "Maximum number of events (0 = unlimited)")
	historyCmd.Flags().String("since", "", "Show events since date/relative-time (e.g. '2 weeks ago')")
	historyCmd.Flags().Bool("oldest-first", false, "Show oldest events first (default: newest first)")

	return historyCmd
}

// runHistory executes the history command.
func runHistory(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Parse flags
	itemID, _ := cmd.Flags().GetString("id")
	limit, _ := cmd.Flags().GetInt("limit")
	sinceStr, _ := cmd.Flags().GetString("since")
	oldestFirst, _ := cmd.Flags().GetBool("oldest-first")
	jsonMode, _ := cmd.Flags().GetBool("json")

	// Validate selector
	if itemID == "" && len(args) == 0 {
		return errors.New("item selector or --id required")
	}
	if itemID != "" && len(args) > 0 {
		return errors.New("cannot specify both selector argument and --id flag")
	}

	// Open database
	db, err := openDatabase(ctx)
	if err != nil {
		return err
	}
	defer db.Close()

	// Resolve item selector to UUID
	if itemID == "" {
		itemID, err = resolveItemSelector(ctx, db, args[0])
		if err != nil {
			return err
		}
	}

	// Validate item exists
	_, err = db.GetItem(ctx, itemID)
	if err != nil {
		if errors.Is(err, database.ErrItemNotFound) {
			return errors.New("item not found - check spelling or use --id flag")
		}
		return fmt.Errorf("failed to retrieve item: %w", err)
	}

	// Retrieve all events for item
	events, err := db.GetEventsByEntity(ctx, &itemID, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to retrieve events: %w", err)
	}

	if len(events) == 0 {
		return errors.New("no history found for item")
	}

	// Apply filters (newest-first by default, unless --oldest-first)
	filtered, err := filterEvents(events, limit, sinceStr, !oldestFirst)
	if err != nil {
		return fmt.Errorf("filter error: %w", err)
	}

	// Format and output
	return formatOutput(ctx, cmd, db, filtered, jsonMode)
}

// openDatabase opens the database connection using config settings.
func openDatabase(ctx context.Context) (*database.Database, error) {
	return cli.OpenDatabase(ctx)
}
