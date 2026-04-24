package remove

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// NewDefaultRemoveCmd returns a remove command that opens the database from context configuration at runtime.
func NewDefaultRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <entity-id>",
		Short: "Remove an entity from the inventory",
		Long: `Remove an entity from the inventory.

Removed entities are hidden from all normal views.
Their full history is preserved in the event log.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := cli.OpenDatabase(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to open database: %w", err)
			}
			defer db.Close()
			return runRemove(cmd, args, db)
		},
	}
	cmd.Flags().StringP("note", "n", "", "Optional note for event")
	return cmd
}

// NewRemoveCmd returns a remove command using the provided database. Intended for testing.
func NewRemoveCmd(db removeDB) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <entity-id>",
		Short: "Remove an entity from the inventory",
		Long: `Remove an entity from the inventory.

Removed entities are hidden from all normal views.
Their full history is preserved in the event log.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRemove(cmd, args, db)
		},
	}
	cmd.Flags().StringP("note", "n", "", "Optional note for event")
	return cmd
}

func runRemove(cmd *cobra.Command, args []string, db removeDB) error {
	ctx := cmd.Context()
	entityID := args[0]
	noteFlag, _ := cmd.Flags().GetString("note")

	entity, err := db.GetEntity(ctx, entityID)
	if err != nil {
		return fmt.Errorf("entity %q not found: %w", entityID, err)
	}

	payload := map[string]any{
		"entity_id": entityID,
	}

	actorUserID := cli.GetActorUserID(ctx)
	if _, err = db.AppendEvent(ctx, database.EntityRemovedEvent, actorUserID, payload, noteFlag); err != nil {
		return fmt.Errorf("failed to remove entity: %w", err)
	}

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	if cfg.IsJSON() {
		return out.JSON(map[string]string{
			"entity_id":    entityID,
			"display_name": entity.DisplayName,
		})
	}

	out.Success(fmt.Sprintf("Removed %q", entity.DisplayName))
	return nil
}
