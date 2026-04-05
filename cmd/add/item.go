package add

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

var itemCmd *cobra.Command

// GetItemCmd returns the item subcommand, initializing it if necessary.
func GetItemCmd() *cobra.Command {
	if itemCmd != nil {
		return itemCmd
	}

	itemCmd = &cobra.Command{
		Use:   "item ITEM_NAME [ITEM_NAME...]",
		Short: "Add one or more items to a location",
		Long: `Add one or more items to a specified location.

Each item name becomes a separate item with a unique UUID. Multiple identical
names will create separate items (useful for bulk additions like "nail" "nail" "nail").

The --in flag specifies the location where items are stored. Location can be
specified by canonical name or UUID.

Examples:
  wherehouse add item "10mm Socket" --in Garage
  wherehouse add item "Phillips Screwdriver" "Flathead Screwdriver" --in Toolbox
  wherehouse add item "Nail" "Nail" "Nail" --in "Hardware Bin"`,
		Args: cobra.MinimumNArgs(1),
		RunE: runAddItem,
	}

	itemCmd.Flags().StringP("in", "i", "", "Location where items are stored (REQUIRED)")
	if err := itemCmd.MarkFlagRequired("in"); err != nil {
		panic(fmt.Sprintf("failed to mark 'in' flag as required: %v", err))
	}

	return itemCmd
}

// runAddItem implements the item addition logic.
func runAddItem(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Get required --in flag
	locationInput, _ := cmd.Flags().GetString("in")

	// Get database connection
	db, err := openDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Resolve location to UUID
	locationID, err := resolveLocation(ctx, db, locationInput)
	if err != nil {
		return fmt.Errorf("failed to resolve location %q: %w", locationInput, err)
	}

	// Validate location exists
	if validateErr := db.ValidateLocationExists(ctx, locationID); validateErr != nil {
		return fmt.Errorf("location not found: %w", validateErr)
	}

	// Get actor user ID
	actorUserID := cli.GetActorUserID(ctx)

	// Set up output writer
	jsonMode, _ := cmd.Flags().GetBool("json")
	quietMode := cli.IsQuietMode(cmd)
	out := cli.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), jsonMode, quietMode)

	// Process each item (FAIL-FAST: exit on first error)
	for _, itemName := range args {
		// Validate no colon in name (reserved for selector syntax)
		if validateErr := database.ValidateNoColonInName(itemName); validateErr != nil {
			return validateErr // FAIL-FAST: exit on first error
		}

		// Generate UUID v7
		itemUUID, uuidErr := uuid.NewV7()
		if uuidErr != nil {
			return fmt.Errorf("failed to generate UUID: %w", uuidErr)
		}
		itemID := itemUUID.String()

		// Build event payload
		payload := map[string]any{
			"item_id":        itemID,
			"display_name":   itemName,
			"canonical_name": database.CanonicalizeString(itemName),
			"location_id":    locationID,
		}

		// Insert event and update projection atomically
		_, insertErr := db.AppendEvent(ctx, "item.created", actorUserID, payload, "")
		if insertErr != nil {
			return fmt.Errorf("failed to create item %q: %w", itemName, insertErr)
		}

		// Output success (respects quiet mode and JSON mode)
		out.Success(fmt.Sprintf("Added item %q (id: %s) to location %s",
			itemName, itemID, locationID))
	}

	return nil
}
