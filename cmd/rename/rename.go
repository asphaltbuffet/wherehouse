package rename

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// NewDefaultRenameCmd returns a rename command that opens the database from context configuration at runtime.
func NewDefaultRenameCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rename <entity-id>",
		Short: "Rename an entity",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := cli.OpenDatabase(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to open database: %w", err)
			}
			defer db.Close()
			return runRename(cmd, args, db)
		},
	}
	cmd.Flags().StringP("to", "t", "", "New display name (REQUIRED)")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}

// NewRenameCmd returns a rename command using the provided database. Intended for testing.
func NewRenameCmd(db renameDB) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rename <entity-id>",
		Short: "Rename an entity",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRename(cmd, args, db)
		},
	}
	cmd.Flags().StringP("to", "t", "", "New display name (REQUIRED)")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}

type renameResult struct {
	EntityID string `json:"entity_id"`
	OldName  string `json:"old_name"`
	NewName  string `json:"new_name"`
	NewPath  string `json:"new_path"`
}

func runRename(cmd *cobra.Command, args []string, db renameDB) error {
	ctx := cmd.Context()
	entityID := args[0]
	toFlag, _ := cmd.Flags().GetString("to")

	entity, err := db.GetEntity(ctx, entityID)
	if err != nil {
		return fmt.Errorf("entity %q not found: %w", entityID, err)
	}

	oldName := entity.DisplayName

	payload := map[string]any{
		"entity_id":    entityID,
		"display_name": toFlag,
	}

	actorUserID := cli.GetActorUserID(ctx)
	if _, err = db.AppendEvent(ctx, database.EntityRenamedEvent, actorUserID, payload, ""); err != nil {
		return fmt.Errorf("failed to rename entity: %w", err)
	}

	updated, err := db.GetEntity(ctx, entityID)
	if err != nil {
		return fmt.Errorf("failed to retrieve renamed entity: %w", err)
	}

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	if cfg.IsJSON() {
		return out.JSON(renameResult{
			EntityID: entityID,
			OldName:  oldName,
			NewName:  toFlag,
			NewPath:  updated.FullPathDisplay,
		})
	}

	out.Success(fmt.Sprintf("Renamed %q to %q (path: %s)", oldName, toFlag, updated.FullPathDisplay))
	return nil
}
