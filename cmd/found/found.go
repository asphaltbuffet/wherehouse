package found

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// NewDefaultFoundCmd returns a found command that opens the database from context configuration at runtime.
func NewDefaultFoundCmd() *cobra.Command {
	cmd := buildFoundCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		s, a, err := cli.OpenDatabase(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer s.Close()
		return runFound(cmd, args, a)
	}
	return cmd
}

// NewFoundCmd returns a found command backed by the provided App (for tests).
func NewFoundCmd(a *app.App) *cobra.Command {
	cmd := buildFoundCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return runFound(cmd, args, a) }
	return cmd
}

func buildFoundCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "found <path>...",
		Short: "Mark one or more entities as ok",
		Long: `Mark one or more entities as ok (status: ok).

All entities are updated atomically — if any path fails, none are changed.

Examples:
  wherehouse found "Garage:Toolbox:Wrench"
  wherehouse found "Garage:Toolbox:Wrench" "Shelf:Hammer" --note "recovered after move"`,
		Args: cobra.MinimumNArgs(1),
	}
	cmd.Flags().StringP("note", "n", "", "Optional note for the event")
	return cmd
}

func runFound(cmd *cobra.Command, args []string, a *app.App) error {
	ctx := cmd.Context()
	noteFlag, _ := cmd.Flags().GetString("note")

	reqs := make([]app.ChangeStatusRequest, len(args))
	for i, path := range args {
		reqs[i] = app.ChangeStatusRequest{
			EntityPath: path,
			Status:     inventory.EntityStatusOk,
			Note:       noteFlag,
			ActorID:    cli.GetActorUserID(ctx),
		}
	}

	results, err := a.MarkFound(ctx, reqs)
	if err != nil {
		return fmt.Errorf("failed to mark found: %w", err)
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
		out.Success(fmt.Sprintf("%q marked as ok", result.FullPathDisplay))
	}
	return nil
}
