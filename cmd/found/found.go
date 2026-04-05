package found

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

const foundLongDescription = `Record that one or more items have been found at a specific location.

The item's home location is NOT changed by default. Use --return to also
move the item back to its home location immediately.

Selector types:
  - ID:            aB3xK9mPqR
  - LOCATION:ITEM: garage:socket (both canonical names)
  - Canonical:     "10mm socket" (must match exactly 1 item)

Examples:
  wherehouse found "10mm socket" --in garage
  wherehouse found "10mm socket" --in garage --return
  wherehouse found garage:screwdriver --in shed --note "behind workbench"`

// Result represents the result of a single item found operation.
type Result struct {
	ItemID        string   `json:"item_id"`
	DisplayName   string   `json:"display_name"`
	FoundAt       string   `json:"found_at"`
	HomeLocation  string   `json:"home_location"`
	Returned      bool     `json:"returned"`
	FoundEventID  int64    `json:"found_event_id"`
	ReturnEventID *int64   `json:"return_event_id,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
}

// NewFoundCmd returns a found command that uses the provided db for all database
// operations. The caller retains no reference to db after this call; the
// returned command's RunE closes it via defer before returning.
func NewFoundCmd(db foundDB) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "found <item-selector>... --in <location>",
		Short: "Record that a lost or missing item has been found",
		Long:  foundLongDescription,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() {
				if closeErr := db.Close(); closeErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to close database: %v\n", closeErr)
				}
			}()
			return runFoundItem(cmd, args, db)
		},
	}

	registerFoundFlags(cmd)
	return cmd
}

// NewDefaultFoundCmd returns a found command that opens the database from context
// configuration at runtime. This is the production entry point registered with
// the root command.
func NewDefaultFoundCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "found <item-selector>... --in <location>",
		Short: "Record that a lost or missing item has been found",
		Long:  foundLongDescription,
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
			return runFoundItem(cmd, args, db)
		},
	}

	registerFoundFlags(cmd)
	return cmd
}

// registerFoundFlags attaches all found-specific flags to cmd.
// Called by both NewFoundCmd and NewDefaultFoundCmd to ensure identical flag sets.
func registerFoundFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("in", "i", "", "location where item was found (required)")
	_ = cmd.MarkFlagRequired("in")

	cmd.Flags().BoolP("return", "r", false, "also return item to its home location")
	cmd.Flags().StringP("note", "n", "", "optional note for event")
}

// runFoundItem is the main entry point for the found command.
func runFoundItem(cmd *cobra.Command, args []string, db foundDB) error {
	ctx := cmd.Context()

	foundLocationStr, _ := cmd.Flags().GetString("in")
	returnToHome, _ := cmd.Flags().GetBool("return")
	note, _ := cmd.Flags().GetString("note")

	actorUserID := cli.GetActorUserID(ctx)

	foundLocationID, err := cli.ResolveLocation(ctx, db, foundLocationStr)
	if err != nil {
		return fmt.Errorf("found location not found: %w", err)
	}

	if sysErr := validateNotSystemLocation(ctx, db, foundLocationID); sysErr != nil {
		return sysErr
	}

	cfg := cli.MustGetConfig(ctx)
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	var results []Result

	for _, selector := range args {
		itemID, itemErr := cli.ResolveItemSelector(ctx, db, selector, "wherehouse found")
		if itemErr != nil {
			return fmt.Errorf("failed to resolve %q: %w", selector, itemErr)
		}

		result, foundErr := foundItem(ctx, db, itemID, foundLocationID, returnToHome, actorUserID, note)
		if foundErr != nil {
			return fmt.Errorf("failed to record found for %q: %w", selector, foundErr)
		}

		results = append(results, *result)

		if !cfg.IsJSON() {
			out.Success(formatSuccessMessage(result))

			for _, w := range result.Warnings {
				out.Warning(w)
			}
		}
	}

	if cfg.IsJSON() {
		output := map[string]any{"found": results}
		if jsonErr := out.JSON(output); jsonErr != nil {
			return fmt.Errorf("failed to encode JSON output: %w", jsonErr)
		}
	}

	return nil
}

// foundItem performs a single item found operation, firing the item.found event
// and optionally a follow-up item.moved event when --return is used.
// It delegates all domain logic to cli.FoundItem.
func foundItem(
	ctx context.Context,
	db foundDB,
	itemID, foundLocationID string,
	returnToHome bool,
	actorUserID, note string,
) (*Result, error) {
	r, err := cli.FoundItem(ctx, db, itemID, foundLocationID, returnToHome, actorUserID, note)
	if err != nil {
		return nil, err
	}

	return &Result{
		ItemID:        r.ItemID,
		DisplayName:   r.DisplayName,
		FoundAt:       r.FoundAt,
		HomeLocation:  r.HomeLocation,
		Returned:      r.Returned,
		FoundEventID:  r.FoundEventID,
		ReturnEventID: r.ReturnEventID,
		Warnings:      r.Warnings,
	}, nil
}

// validateNotSystemLocation returns an error if the given location is a system location.
func validateNotSystemLocation(ctx context.Context, db foundDB, locationID string) error {
	loc, err := db.GetLocation(ctx, locationID)
	if err != nil {
		return fmt.Errorf("failed to get location: %w", err)
	}

	if loc.IsSystem {
		return fmt.Errorf(
			"cannot record item as found at system location %q\nUse a real location for --in",
			loc.DisplayName,
		)
	}

	return nil
}

// formatSuccessMessage returns a human-readable success message for a found result.
func formatSuccessMessage(r *Result) string {
	if r.Returned {
		return fmt.Sprintf("Found %q at %s, returned to %s", r.DisplayName, r.FoundAt, r.HomeLocation)
	}

	return fmt.Sprintf("Found %q at %s (home: %s)", r.DisplayName, r.FoundAt, r.HomeLocation)
}

// GetFoundCmd returns the found command using the default database.
//
// Deprecated: Use NewDefaultFoundCmd instead.
func GetFoundCmd() *cobra.Command {
	return NewDefaultFoundCmd()
}

// ensure *database.Database satisfies foundDB at compile time.
var _ foundDB = (*database.Database)(nil)
