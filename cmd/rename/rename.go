package rename

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

type renameResult struct {
	EntityID string `json:"entity_id"`
	NewName  string `json:"new_name"`
	NewPath  string `json:"new_path"`
}

// NewDefaultRenameCmd returns the rename command wired to the real database.
func NewDefaultRenameCmd() *cobra.Command {
	cmd := buildRenameCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		s, a, err := cli.OpenDatabase(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer s.Close()
		return runRename(cmd, args, a)
	}
	return cmd
}

// NewRenameCmd returns the rename command using the supplied renameApp (for testing).
func NewRenameCmd(a *app.App) *cobra.Command {
	cmd := buildRenameCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return runRename(cmd, args, a) }
	return cmd
}

func buildRenameCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rename <path>",
		Short: "Rename an entity by its full colon-separated path",
		Long: `Rename an entity. Identify the entity by its full path (parent:child:...).

Examples:
  wherehouse rename "Garage:Toolbox" --to "Tool Chest"
  wherehouse rename "Garage:Toolbox:Wrench" --to "Pipe Wrench"`,
		Args: cobra.ExactArgs(1),
	}
	cmd.Flags().StringP("to", "t", "", "New display name")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}

func runRename(cmd *cobra.Command, args []string, a *app.App) error {
	ctx := cmd.Context()
	path := args[0]
	toFlag, _ := cmd.Flags().GetString("to")

	updated, err := a.RenameEntity(ctx, app.RenameEntityRequest{
		EntityPath: path,
		NewName:    toFlag,
		ActorID:    cli.GetActorUserID(ctx),
	})
	if err != nil {
		return fmt.Errorf("failed to rename %q: %w", path, err)
	}

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	if cfg.IsJSON() {
		return out.JSON(renameResult{
			EntityID: updated.EntityID,
			NewName:  toFlag,
			NewPath:  updated.FullPathDisplay,
		})
	}
	out.Success(fmt.Sprintf("Renamed %q to %q (path: %s)", path, toFlag, updated.FullPathDisplay))
	return nil
}
