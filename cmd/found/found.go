package found

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

var foundCmd *cobra.Command

// GetFoundCmd returns the found command, initializing it if necessary.
func GetFoundCmd() *cobra.Command {
	if foundCmd != nil {
		return foundCmd
	}

	foundCmd = &cobra.Command{
		Use:   "found <item-selector>... --in <location>",
		Short: "Record that a lost or missing item has been found",
		Long: `Record that one or more items have been found at a specific location.

The item's home location is NOT changed by default. Use --return to also
move the item back to its home location immediately.

Selector types:
  - UUID:          550e8400-e29b-41d4-a716-446655440001
  - LOCATION:ITEM: garage:socket (both canonical names)
  - Canonical:     "10mm socket" (must match exactly 1 item)

Examples:
  wherehouse found "10mm socket" --in garage
  wherehouse found "10mm socket" --in garage --return
  wherehouse found garage:screwdriver --in shed --note "behind workbench"`,
		Args: cobra.MinimumNArgs(1),
		RunE: runFoundItem,
	}

	foundCmd.Flags().StringP("in", "i", "", "location where item was found (required)")
	_ = foundCmd.MarkFlagRequired("in")

	foundCmd.Flags().BoolP("return", "r", false, "also return item to its home location")
	foundCmd.Flags().StringP("note", "n", "", "optional note for event")

	return foundCmd
}

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

// runFoundItem is the main entry point for the found command.
func runFoundItem(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	foundLocationStr, _ := cmd.Flags().GetString("in")
	returnToHome, _ := cmd.Flags().GetBool("return")
	note, _ := cmd.Flags().GetString("note")

	db, err := cli.OpenDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

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
func foundItem(
	ctx context.Context,
	db *database.Database,
	itemID, foundLocationID string,
	returnToHome bool,
	actorUserID, note string,
) (*Result, error) {
	// Get current item state
	item, err := db.GetItem(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("item not found: %w", err)
	}

	// Get current item location for warning checks
	currentLoc, err := db.GetLocation(ctx, item.LocationID)
	if err != nil {
		return nil, fmt.Errorf("current location not found: %w", err)
	}

	// Collect non-fatal warnings about the item's current state
	var warnings []string

	switch {
	case currentLoc.IsSystem && currentLoc.CanonicalName == "missing":
		// Normal case: item is at Missing - no warning needed
	case currentLoc.IsSystem:
		// Item is at a non-Missing system location (e.g. Borrowed)
		warnings = append(warnings, fmt.Sprintf(
			"item is currently at system location %q (not Missing)", currentLoc.DisplayName))
	default:
		// Item is at a normal (non-system) location
		warnings = append(warnings, fmt.Sprintf(
			"item is not currently missing (currently at %q)", currentLoc.DisplayName))
	}

	// Determine home location for the item.found event payload.
	// If TempOriginLocationID is NULL, use foundLocationID as a safe fallback
	// so the event handler always receives a valid home_location_id.
	homeLocationID := foundLocationID
	if item.TempOriginLocationID != nil {
		homeLocationID = *item.TempOriginLocationID
	}

	// Get location display names for the result
	foundLoc, err := db.GetLocation(ctx, foundLocationID)
	if err != nil {
		return nil, fmt.Errorf("found location details not found: %w", err)
	}

	homeLoc, err := db.GetLocation(ctx, homeLocationID)
	if err != nil {
		return nil, fmt.Errorf("home location details not found: %w", err)
	}

	// Fire item.found event
	foundPayload := map[string]any{
		"item_id":           itemID,
		"found_location_id": foundLocationID,
		"home_location_id":  homeLocationID,
	}

	foundEventID, err := db.AppendEvent(ctx, "item.found", actorUserID, foundPayload, note)
	if err != nil {
		return nil, fmt.Errorf("failed to create found event: %w", err)
	}

	result := &Result{
		ItemID:       itemID,
		DisplayName:  item.DisplayName,
		FoundAt:      foundLoc.DisplayName,
		HomeLocation: homeLoc.DisplayName,
		Returned:     false,
		FoundEventID: foundEventID,
		Warnings:     warnings,
	}

	// Handle --return flag
	if returnToHome {
		switch {
		case item.TempOriginLocationID == nil:
			// Home is unknown - skip move, add warning
			result.Warnings = append(result.Warnings,
				"home location unknown - could not return item (use move command to return manually)")

		case foundLocationID == homeLocationID:
			// Already at home - skip move, add note
			result.Warnings = append(result.Warnings,
				"already at home location - return skipped")

		default:
			// Validate from_location matches projection (CRITICAL for event-sourcing)
			if validateErr := db.ValidateFromLocation(ctx, itemID, foundLocationID); validateErr != nil {
				return nil, fmt.Errorf("projection validation failed: %w", validateErr)
			}

			// Fire item.moved rehome event to return to home
			movePayload := map[string]any{
				"item_id":          itemID,
				"from_location_id": foundLocationID,
				"to_location_id":   homeLocationID,
				"move_type":        "rehome",
				"project_action":   "clear",
			}

			returnEventID, moveErr := db.AppendEvent(ctx, "item.moved", actorUserID, movePayload, note)
			if moveErr != nil {
				return nil, fmt.Errorf("failed to create return event: %w", moveErr)
			}

			result.Returned = true
			result.ReturnEventID = &returnEventID
		}
	}

	return result, nil
}

// validateNotSystemLocation returns an error if the given location is a system location.
func validateNotSystemLocation(ctx context.Context, db *database.Database, locationID string) error {
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
