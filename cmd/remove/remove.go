package remove

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

// NewDefaultRemoveCmd returns a remove command that opens the database from context configuration at runtime.
func NewDefaultRemoveCmd() *cobra.Command {
	cmd := buildRemoveCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		s, a, err := cli.OpenDatabase(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer s.Close()
		return runRemove(cmd, args, a)
	}
	return cmd
}

// NewRemoveCmd returns a remove command using the provided removeApp. Intended for testing.
func NewRemoveCmd(a *app.App) *cobra.Command {
	cmd := buildRemoveCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return runRemove(cmd, args, a) }
	return cmd
}

func buildRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <path>",
		Short: "Remove an entity from the inventory",
		Long: `Remove an entity by its full colon-separated path.

Examples:
  wherehouse remove "Garage:Toolbox:Wrench"
  wherehouse remove "Garage:Toolbox" --note "disposed"`,
		Args: cobra.ExactArgs(1),
	}
	cmd.Flags().StringP("note", "n", "", "Optional note about why the entity was removed")
	return cmd
}

func runRemove(cmd *cobra.Command, args []string, a *app.App) error {
	ctx := cmd.Context()
	path := args[0]
	noteFlag, _ := cmd.Flags().GetString("note")

	entity, err := a.LookupEntityByPath(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to find %q: %w", path, err)
	}

	err = a.RemoveEntity(ctx, app.RemoveEntityRequest{
		EntityID: entity.EntityID,
		ActorID:  cli.GetActorUserID(ctx),
		Note:     noteFlag,
	})
	if err != nil {
		return fmt.Errorf("failed to remove %q: %w", path, err)
	}

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)
	if cfg.IsJSON() {
		return out.JSON(map[string]string{
			"path": path,
		})
	}
	out.Success(fmt.Sprintf("Removed %q", path))
	return nil
}
