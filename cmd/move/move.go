package move

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// NewDefaultMoveCmd returns a move command that opens the database from context configuration at runtime.
func NewDefaultMoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "move <entity-id>",
		Short: "Move an entity to a new parent",
		Long: `Move an entity to a new parent entity.

Place-type entities cannot be moved. Only containers and leaf entities are movable.

Examples:
  wherehouse move <entity-id> --to <dest-id>`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := cli.OpenDatabase(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to open database: %w", err)
			}
			defer db.Close()
			return runMove(cmd, args, db)
		},
	}
	cmd.Flags().StringP("to", "t", "", "Destination entity ID (REQUIRED)")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}

// NewMoveCmd returns a move command using the provided database. Intended for testing.
func NewMoveCmd(db moveDB) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "move <entity-id>",
		Short: "Move an entity to a new parent",
		Long: `Move an entity to a new parent entity.

Place-type entities cannot be moved. Only containers and leaf entities are movable.

Examples:
  wherehouse move <entity-id> --to <dest-id>`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMove(cmd, args, db)
		},
	}
	cmd.Flags().StringP("to", "t", "", "Destination entity ID (REQUIRED)")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}

type moveResult struct {
	EntityID    string `json:"entity_id"`
	DisplayName string `json:"display_name"`
	OldPath     string `json:"old_path"`
	NewPath     string `json:"new_path"`
}

func runMove(cmd *cobra.Command, args []string, db moveDB) error {
	ctx := cmd.Context()
	entityID := args[0]
	toID, _ := cmd.Flags().GetString("to")

	entity, err := db.GetEntity(ctx, entityID)
	if err != nil {
		return fmt.Errorf("entity %q not found: %w", entityID, err)
	}

	if entity.EntityType == database.EntityTypePlace {
		return fmt.Errorf("place entities cannot be moved: %q is a place", entity.DisplayName)
	}

	dest, err := db.GetEntity(ctx, toID)
	if err != nil {
		return fmt.Errorf("destination %q not found: %w", toID, err)
	}

	oldPath := entity.FullPathDisplay

	payload := map[string]any{
		"entity_id": entityID,
		"parent_id": dest.EntityID,
	}

	actorUserID := cli.GetActorUserID(ctx)
	if _, err = db.AppendEvent(ctx, database.EntityReparentedEvent, actorUserID, payload, ""); err != nil {
		return fmt.Errorf("failed to move entity: %w", err)
	}

	updated, err := db.GetEntity(ctx, entityID)
	if err != nil {
		return fmt.Errorf("failed to retrieve updated entity: %w", err)
	}

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	if cfg.IsJSON() {
		return out.JSON(moveResult{
			EntityID:    entityID,
			DisplayName: entity.DisplayName,
			OldPath:     oldPath,
			NewPath:     updated.FullPathDisplay,
		})
	}

	out.Success(fmt.Sprintf("Moved %q: %s → %s", entity.DisplayName, oldPath, updated.FullPathDisplay))
	return nil
}
