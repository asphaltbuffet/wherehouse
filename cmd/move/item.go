package move

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// Result represents the result of a single item move operation.
type Result struct {
	ItemID        string `json:"item_id"`
	DisplayName   string `json:"display_name"`
	FromLocation  string `json:"from_location"`
	ToLocation    string `json:"to_location"`
	EventID       int64  `json:"event_id"`
	MoveType      string `json:"move_type"`
	ProjectAction string `json:"project_action,omitempty"`
	ProjectID     string `json:"project_id,omitempty"`
}

// runMoveItemCore contains the main business logic for the move command.
// The db connection lifecycle (Close) is owned by the RunE closures in move.go.
func runMoveItemCore(cmd *cobra.Command, args []string, db moveDB) error {
	ctx := cmd.Context()

	// Parse flags
	toLocation, _ := cmd.Flags().GetString("to")
	temp, _ := cmd.Flags().GetBool("temp")
	projectID, _ := cmd.Flags().GetString("project")
	keepProject, _ := cmd.Flags().GetBool("keep-project")
	note, _ := cmd.Flags().GetString("note")

	// Get actor user ID
	actorUserID := cli.GetActorUserID(ctx)

	// Resolve destination location once (shared across all moves)
	toLocationID, err := resolveLocation(ctx, db, toLocation)
	if err != nil {
		return fmt.Errorf("destination location not found: %w", err)
	}

	// Validate destination is not a system location
	if sysErr := validateDestinationNotSystem(ctx, db, toLocationID); sysErr != nil {
		return sysErr
	}

	// Validate project if specified
	if projectID != "" {
		activeStatus := "active"
		if projErr := db.ValidateProjectExists(ctx, projectID, &activeStatus); projErr != nil {
			return fmt.Errorf("project validation failed: %w", projErr)
		}
	}

	// Determine move type and project action
	moveType := determineMoveType(temp)
	projectAction := determineProjectAction(projectID, keepProject)

	// Set up output writer
	cfg := cli.MustGetConfig(ctx)
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	// Process each item selector in order (fail-fast)
	var results []Result

	for _, selector := range args {
		// Resolve item selector
		itemID, itemErr := resolveItemSelector(ctx, db, selector)
		if itemErr != nil {
			return fmt.Errorf("failed to resolve %q: %w", selector, itemErr)
		}

		// Perform move
		result, moveErr := moveItem(
			ctx, db, itemID, toLocationID,
			moveType, projectAction, projectID, actorUserID, note,
		)
		if moveErr != nil {
			return fmt.Errorf("failed to move %q: %w", selector, moveErr)
		}

		results = append(results, *result)

		// Print success message (unless quiet or JSON mode)
		if !cfg.IsJSON() {
			out.Success(fmt.Sprintf("Moved item %q from %s to %s",
				result.DisplayName, result.FromLocation, result.ToLocation))
		}
	}

	// Output JSON if requested
	if cfg.IsJSON() {
		output := map[string]any{
			"moved": results,
		}
		if jsonErr := out.JSON(output); jsonErr != nil {
			return fmt.Errorf("failed to encode JSON output: %w", jsonErr)
		}
	}

	return nil
}

// moveItem performs a single item move operation.
func moveItem(
	ctx context.Context,
	db moveDB,
	itemID, toLocationID, moveType, projectAction, projectID, actorUserID, note string,
) (*Result, error) {
	// Get current item state
	item, err := db.GetItem(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("item not found: %w", err)
	}

	// Get from location
	fromLocation, err := db.GetLocation(ctx, item.LocationID)
	if err != nil {
		return nil, fmt.Errorf("from location not found: %w", err)
	}

	// Validate: Cannot move FROM system location
	if fromLocation.IsSystem {
		return nil, fmt.Errorf(
			"cannot move items from system location %q\nUse dedicated command for this operation",
			fromLocation.DisplayName,
		)
	}

	// Get to location
	toLocation, err := db.GetLocation(ctx, toLocationID)
	if err != nil {
		return nil, fmt.Errorf("to location not found: %w", err)
	}

	// Validate: Cannot move TO system location (redundant with pre-check, but defensive)
	if toLocation.IsSystem {
		return nil, fmt.Errorf(
			"cannot move items to system location %q\nUse dedicated command for this operation",
			toLocation.DisplayName,
		)
	}

	// Validate from_location matches projection (CRITICAL for event-sourcing)
	if validateErr := db.ValidateFromLocation(ctx, itemID, item.LocationID); validateErr != nil {
		return nil, fmt.Errorf("projection validation failed: %w", validateErr)
	}

	// Build event payload
	payload := map[string]any{
		"item_id":          itemID,
		"from_location_id": item.LocationID,
		"to_location_id":   toLocationID,
		"move_type":        moveType,
	}

	// Add project fields if applicable
	if projectAction != "" {
		payload["project_action"] = projectAction
	}
	if projectID != "" {
		payload["project_id"] = projectID
	}

	// Insert event and update projection atomically
	eventID, err := db.AppendEvent(ctx, database.ItemMovedEvent, actorUserID, payload, note)
	if err != nil {
		return nil, fmt.Errorf("failed to create move event: %w", err)
	}

	// Build result
	result := &Result{
		ItemID:        itemID,
		DisplayName:   item.DisplayName,
		FromLocation:  fromLocation.DisplayName,
		ToLocation:    toLocation.DisplayName,
		EventID:       eventID,
		MoveType:      moveType,
		ProjectAction: projectAction,
	}
	if projectID != "" {
		result.ProjectID = projectID
	}

	return result, nil
}

// validateDestinationNotSystem checks that destination is not a system location.
func validateDestinationNotSystem(ctx context.Context, db moveDB, locationID string) error {
	loc, err := db.GetLocation(ctx, locationID)
	if err != nil {
		return fmt.Errorf("failed to get destination location: %w", err)
	}

	if loc.IsSystem {
		return fmt.Errorf(
			"cannot move items to system location %q\nUse dedicated command for this operation",
			loc.DisplayName,
		)
	}

	return nil
}

// determineMoveType determines the move type based on flags.
func determineMoveType(temp bool) string {
	if temp {
		return "temporary_use"
	}
	return "rehome"
}

// determineProjectAction determines the project action based on flags.
func determineProjectAction(projectID string, keepProject bool) string {
	if projectID != "" {
		return "set"
	}
	if keepProject {
		return "keep"
	}
	return "clear" // Default
}
