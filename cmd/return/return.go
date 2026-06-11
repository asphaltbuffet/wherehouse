package returncmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// NewDefaultReturnCmd returns a return command that opens the database from context configuration at runtime.
func NewDefaultReturnCmd() *cobra.Command {
	cmd := buildReturnCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		s, a, err := cli.OpenDatabase(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer s.Close()
		return runReturn(cmd, args, a)
	}
	return cmd
}

// NewReturnCmd returns a return command backed by the provided App (for tests).
func NewReturnCmd(a *app.App) *cobra.Command {
	cmd := buildReturnCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return runReturn(cmd, args, a) }
	return cmd
}

func buildReturnCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "return <path>...",
		Short: "Mark one or more loaned entities as returned",
		Long: `Mark one or more entities as ok (status: ok), recording that they have been returned.

All entities are updated atomically — if any path fails, none are changed.

Examples:
  wherehouse return "Garage:Toolbox:Wrench"
  wherehouse return "Garage:Toolbox:Wrench" "Shelf:Hammer" --note "returned after the weekend"`,
		Args: cobra.MinimumNArgs(1),
	}
	cmd.Flags().StringP("note", "n", "", "Optional note for the event")
	return cmd
}

func runReturn(cmd *cobra.Command, args []string, a *app.App) error {
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

	results, err := a.MarkReturned(ctx, reqs)
	if err != nil {
		return fmt.Errorf("failed to mark returned: %w", err)
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
		if result.Status == inventory.EntityStatusRemoved {
			out.Success(fmt.Sprintf("%q returned and removed from inventory", result.FullPathDisplay))
		} else {
			out.Success(fmt.Sprintf("%q marked as returned", result.FullPathDisplay))
		}
	}
	return nil
}
