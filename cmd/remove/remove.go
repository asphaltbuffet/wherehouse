package remove

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

const removeLongDescription = `Remove an item or empty location from the inventory.

Items are moved to the Removed system location and hidden from all normal views.
Their full history is preserved in the event log.

Non-system locations can be removed only if they are empty (no items, no sub-locations).

Selector types for items:
  - ID: aB3xK9mPqR (exact ID)
  - LOCATION:ITEM: garage:socket (both canonical names, filters by location)
  - Canonical name: "10mm socket" (must match exactly 1 item)

Use --location (-l) to remove a location instead of an item.

Examples:
  wherehouse remove garage:socket
  wherehouse remove "10mm socket" --note "broken beyond repair"
  wherehouse remove aB3xK9mPqR
  wherehouse remove --location "Old Shelf"`

// NewRemoveCmd returns a remove command that uses the provided db for all database
// operations. The caller retains no reference to db after this call; the
// returned command's RunE closes it via defer before returning.
func NewRemoveCmd(db removeDB) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <selector>",
		Short: "Remove an item or empty location",
		Long:  removeLongDescription,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() {
				if closeErr := db.Close(); closeErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to close database: %v\n", closeErr)
				}
			}()
			return runRemoveCore(cmd, args, db)
		},
	}

	registerRemoveFlags(cmd)
	return cmd
}

// NewDefaultRemoveCmd returns a remove command that opens the database from context
// configuration at runtime. This is the production entry point registered with
// the root command.
func NewDefaultRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <selector>",
		Short: "Remove an item or empty location",
		Long:  removeLongDescription,
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
			return runRemoveCore(cmd, args, db)
		},
	}

	registerRemoveFlags(cmd)
	return cmd
}

// registerRemoveFlags attaches all remove-specific flags to cmd.
// Called by both NewRemoveCmd and NewDefaultRemoveCmd to ensure identical flag sets.
func registerRemoveFlags(cmd *cobra.Command) {
	cmd.Flags().BoolP("location", "l", false, "remove a location instead of an item")
	cmd.Flags().StringP("note", "n", "", "optional note for event")
}

// ensure *database.Database satisfies removeDB at compile time.
var _ removeDB = (*database.Database)(nil)

// runRemoveCore dispatches to item or location removal based on flags.
func runRemoveCore(cmd *cobra.Command, args []string, db removeDB) error {
	ctx := cmd.Context()
	selector := args[0]

	isLocation, _ := cmd.Flags().GetBool("location")
	note, _ := cmd.Flags().GetString("note")

	actorUserID := cli.GetActorUserID(ctx)
	cfg := cli.MustGetConfig(ctx)
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	if isLocation {
		return runRemoveLocation(ctx, db, selector, actorUserID, note, cfg, out)
	}

	return runRemoveItem(ctx, db, selector, actorUserID, note, cfg, out)
}

func runRemoveLocation(
	ctx context.Context,
	db removeDB,
	selector, actorUserID, note string,
	cfg *config.Config,
	out *cli.OutputWriter,
) error {
	locationID, err := cli.ResolveLocation(ctx, db, selector)
	if err != nil {
		return fmt.Errorf("failed to resolve location %q: %w", selector, err)
	}

	result, err := removeLocation(ctx, db, locationID, actorUserID, note)
	if err != nil {
		return fmt.Errorf("failed to remove location: %w", err)
	}

	if cfg.IsJSON() {
		if jsonErr := out.JSON(result); jsonErr != nil {
			return fmt.Errorf("failed to encode JSON output: %w", jsonErr)
		}
		return nil
	}

	out.Success(fmt.Sprintf("Removed location %q", result.DisplayName))
	return nil
}

func runRemoveItem(
	ctx context.Context,
	db removeDB,
	selector, actorUserID, note string,
	cfg *config.Config,
	out *cli.OutputWriter,
) error {
	result, err := removeItem(ctx, db, selector, actorUserID, note)
	if err != nil {
		return fmt.Errorf("failed to remove item: %w", err)
	}

	if cfg.IsJSON() {
		if jsonErr := out.JSON(result); jsonErr != nil {
			return fmt.Errorf("failed to encode JSON output: %w", jsonErr)
		}
		return nil
	}

	out.Success(fmt.Sprintf("Removed item %q (was in %s)", result.DisplayName, result.PreviousLocation))
	return nil
}
