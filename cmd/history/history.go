package history

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

const historyLongDescription = `Display event history for a specific item (newest first by default).

Item selector can be:
  - Canonical name: "10mm_socket"
  - Location-scoped: "garage:10mm_socket"
  - ID: --id <id>

Examples:
  wherehouse history 10mm_socket                                    # Show full history
  wherehouse history toolbox:screwdriver -n 10                      # Last 10 events
  wherehouse history --id abc-123-def --since "2 weeks ago"         # Filter by date
  wherehouse history socket --since 2026-01-15 --oldest-first       # Oldest first`

// NewHistoryCmd returns a history command that opens the database from context
// configuration at runtime.
func NewHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history <item-selector>",
		Short: "Show event history for an item",
		Long:  historyLongDescription,
		Args:  cobra.MaximumNArgs(1), // 0 args if using --id
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
			return runHistoryCore(cmd, args, db)
		},
	}

	cmd.Flags().StringP("id", "i", "", "Item ID (alternative to name selector)")
	cmd.Flags().IntP("limit", "n", 0, "Maximum number of events (0 = unlimited)")
	cmd.Flags().String("since", "", "Show events since date/relative-time (e.g. '2 weeks ago')")
	cmd.Flags().Bool("oldest-first", false, "Show oldest events first (default: newest first)")

	return cmd
}

// ensure *database.Database satisfies historyDB at compile time.
var _ historyDB = (*database.Database)(nil)

// runHistoryCore executes the history command.
func runHistoryCore(cmd *cobra.Command, args []string, db historyDB) error {
	ctx := cmd.Context()

	// Parse flags
	itemID, _ := cmd.Flags().GetString("id")
	limit, _ := cmd.Flags().GetInt("limit")
	sinceStr, _ := cmd.Flags().GetString("since")
	oldestFirst, _ := cmd.Flags().GetBool("oldest-first")

	// Validate selector
	if itemID == "" && len(args) == 0 {
		return errors.New("item selector or --id required")
	}
	if itemID != "" && len(args) > 0 {
		return errors.New("cannot specify both selector argument and --id flag")
	}

	var err error

	// Resolve item selector to ID
	if itemID == "" {
		itemID, err = cli.ResolveItemSelector(ctx, db, args[0], "wherehouse history")
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
	events, err := db.GetEventsByEntity(ctx, &itemID, nil)
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
	cfg := cli.MustGetConfig(ctx)
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	return formatOutput(ctx, out, db, filtered, cfg.IsJSON())
}
