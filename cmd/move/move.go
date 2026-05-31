package move

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

type moveResult struct {
	EntityID    string `json:"entity_id"`
	DisplayName string `json:"display_name"`
	OldPath     string `json:"old_path"`
	NewPath     string `json:"new_path"`
}

// NewDefaultMoveCmd returns a move command that opens the database from context configuration at runtime.
func NewDefaultMoveCmd() *cobra.Command {
	cmd := buildMoveCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		s, a, err := cli.OpenDatabase(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer s.Close()
		return runMove(cmd, args, a)
	}
	return cmd
}

// NewMoveCmd returns a move command using the provided moveApp. Intended for testing.
func NewMoveCmd(a *app.App) *cobra.Command {
	cmd := buildMoveCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return runMove(cmd, args, a) }
	return cmd
}

func buildMoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "move <path>",
		Short: "Move an entity to a new parent",
		Long: `Move an entity to a new parent entity.

Place-type entities cannot be moved. Only containers and leaf entities are movable.

Examples:
  wherehouse move "Garage:Toolbox:Wrench" --to "Workshop"
  wherehouse move "Garage:Toolbox" --to "Basement"`,
		Args: cobra.ExactArgs(1),
	}
	cmd.Flags().StringP("to", "t", "", "Destination parent path (REQUIRED)")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}

func runMove(cmd *cobra.Command, args []string, a *app.App) error {
	ctx := cmd.Context()
	srcPath := args[0]
	destPath, _ := cmd.Flags().GetString("to")

	updated, err := a.ReparentEntity(ctx, app.ReparentEntityRequest{
		EntityPath:    srcPath,
		NewParentPath: destPath,
		ActorID:       cli.GetActorUserID(ctx),
	})
	if err != nil {
		return fmt.Errorf("failed to move %q: %w", srcPath, err)
	}

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)
	if cfg.IsJSON() {
		return out.JSON(moveResult{
			EntityID:    updated.EntityID,
			DisplayName: updated.DisplayName,
			OldPath:     srcPath,
			NewPath:     updated.FullPathDisplay,
		})
	}
	out.Success(fmt.Sprintf("Moved %q → %s", srcPath, updated.FullPathDisplay))
	return nil
}
