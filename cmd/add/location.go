package add

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/database"
	"github.com/asphaltbuffet/wherehouse/internal/nanoid"
)

var locationCmd *cobra.Command

// GetLocationCmd returns the location subcommand, initializing it if necessary.
func GetLocationCmd() *cobra.Command {
	if locationCmd != nil {
		return locationCmd
	}

	locationCmd = &cobra.Command{
		Use:   "location LOCATION_NAME [LOCATION_NAME...]",
		Short: "Add one or more locations",
		Long: `Add one or more locations to the hierarchy.

If --in is specified, locations are created as children of that parent.
Otherwise, locations are created at the root level.

Each location receives a unique ID and is validated for name uniqueness.

Examples:
  wherehouse add location Garage            # Create root location
  wherehouse add location Shelf --in Garage # Create child location
  wherehouse add location "Shelf A" "Shelf B" --in Garage # Multiple locations`,
		Args: cobra.MinimumNArgs(1), // Require at least one location name
		RunE: runAddLocation,
	}

	locationCmd.Flags().StringP("in", "i", "", "Parent location name or ID (optional, omit for root)")

	return locationCmd
}

// runAddLocation implements the add location command logic.
func runAddLocation(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// 1. Get optional --in flag
	parentInput, _ := cmd.Flags().GetString("in")

	// 2. Get database connection
	db, err := openDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// 3. Resolve parent location (if provided)
	var parentID *string
	if parentInput != "" {
		resolved, resolveErr := resolveLocation(ctx, db, parentInput)
		if resolveErr != nil {
			return fmt.Errorf("failed to resolve parent location %q: %w", parentInput, resolveErr)
		}

		// Validate parent exists
		if validateErr := db.ValidateLocationExists(ctx, resolved); validateErr != nil {
			return fmt.Errorf("parent location not found: %w", validateErr)
		}

		parentID = &resolved
	}

	// 4. Get actor user ID
	actorUserID := cli.GetActorUserID(ctx)

	// 5. Set up output writer
	cfg := cli.MustGetConfig(ctx)
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	// 6. Process each location (FAIL-FAST)
	for _, locationName := range args {
		// Validate no colon in name
		if validateErr := database.ValidateNoColonInName(locationName); validateErr != nil {
			return validateErr // FAIL-FAST: exit on first error
		}

		// Canonicalize name
		canonicalName := database.CanonicalizeString(locationName)

		// Check uniqueness (CRITICAL: must do before event)
		if uniqueErr := db.ValidateUniqueLocationName(ctx, canonicalName, nil); uniqueErr != nil {
			return fmt.Errorf("location %q already exists: %w", locationName, uniqueErr)
		}

		// Generate ID
		locationID, idErr := nanoid.New()
		if idErr != nil {
			return fmt.Errorf("failed to generate ID for location %q: %w", locationName, idErr)
		}

		// Build event payload
		payload := map[string]any{
			"location_id":    locationID,
			"display_name":   locationName,
			"canonical_name": canonicalName,
			"parent_id":      parentID,
			"is_system":      false,
		}

		// Insert event and update projection atomically
		_, insertErr := db.AppendEvent(ctx, database.LocationCreatedEvent, actorUserID, payload, "")
		if insertErr != nil {
			return fmt.Errorf("failed to create location %q: %w", locationName, insertErr)
		}

		// Get full path for output
		loc, getErr := db.GetLocation(ctx, locationID)
		if getErr != nil {
			// Location was successfully created but we can't fetch it for display
			// Show a simpler success message instead of failing
			out.Success(fmt.Sprintf("Added location %q (id: %s)", locationName, locationID))
		} else {
			// Output success with full path (respects quiet mode and JSON mode)
			out.Success(fmt.Sprintf("Added location %q (path: %s)", locationName, loc.FullPathDisplay))
		}
	}

	return nil
}
