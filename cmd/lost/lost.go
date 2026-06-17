package lost

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// NewDefaultLostCmd returns a lost command that opens the database from context configuration at runtime.
func NewDefaultLostCmd() *cobra.Command {
	cmd := buildLostCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		s, a, err := cli.OpenDatabase(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer s.Close()
		return runLost(cmd, args, a)
	}
	return cmd
}

// NewLostCmd returns a lost command backed by the provided App (for tests).
func NewLostCmd(a *app.App) *cobra.Command {
	cmd := buildLostCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return runLost(cmd, args, a) }
	return cmd
}

func buildLostCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lost <path>...",
		Short: "Mark one or more entities as missing",
		Long: `Mark one or more entities as missing (status: missing).

All entities are updated atomically — if any path fails, none are changed.

Examples:
  wherehouse lost "Garage:Toolbox:Wrench"
  wherehouse lost "Garage:Toolbox:Wrench" "Shelf:Hammer" --note "can't find after move"`,
		Args: cobra.MinimumNArgs(1),
	}
	cmd.Flags().StringP("note", "n", "", "Optional note for the event")
	return cmd
}

func runLost(cmd *cobra.Command, args []string, a *app.App) error {
	ctx := cmd.Context()
	noteFlag, _ := cmd.Flags().GetString("note")

	reqs := make([]app.ChangeStatusRequest, len(args))
	for i, path := range args {
		entity, err := a.LookupEntityByPath(ctx, path)
		if err != nil {
			return fmt.Errorf("failed to find %q: %w", path, err)
		}
		reqs[i] = app.ChangeStatusRequest{
			EntityID: entity.EntityID,
			Status:   inventory.EntityStatusMissing,
			Note:     noteFlag,
			ActorID:  cli.GetActorUserID(ctx),
		}
	}

	results, err := a.MarkLost(ctx, reqs)
	if err != nil {
		return fmt.Errorf("failed to mark lost: %w", err)
	}

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	if out.IsJSON() {
		return out.JSON(app.ToStatusOutputs(results))
	}

	for _, result := range results {
		out.Success(fmt.Sprintf("%q marked as missing", result.FullPathDisplay))
	}
	return nil
}
