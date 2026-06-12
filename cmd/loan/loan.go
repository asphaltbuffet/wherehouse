package loan

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// NewDefaultLoanCmd returns a loan command that opens the database from context configuration at runtime.
func NewDefaultLoanCmd() *cobra.Command {
	cmd := buildLoanCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		s, a, err := cli.OpenDatabase(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer s.Close()
		return runLoan(cmd, args, a)
	}
	return cmd
}

// NewLoanCmd returns a loan command backed by the provided App (for tests).
func NewLoanCmd(a *app.App) *cobra.Command {
	cmd := buildLoanCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return runLoan(cmd, args, a) }
	return cmd
}

func buildLoanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "loan <path>...",
		Short: "Mark one or more entities as loaned out",
		Long: `Mark one or more entities as loaned (status: loaned).

All entities are updated atomically — if any path fails, none are changed.

Examples:
  wherehouse loan "Garage:Toolbox:Wrench" --to "Alice"
  wherehouse loan "Garage:Toolbox:Wrench" "Shelf:Hammer" --to "Bob" --note "borrowed for the weekend"`,
		Args: cobra.MinimumNArgs(1),
	}
	cmd.Flags().StringP("to", "t", "", "Who the entity is loaned to")
	_ = cmd.MarkFlagRequired("to")
	cmd.Flags().StringP("note", "n", "", "Optional note for the event")
	return cmd
}

func runLoan(cmd *cobra.Command, args []string, a *app.App) error {
	ctx := cmd.Context()
	toFlag, _ := cmd.Flags().GetString("to")
	noteFlag, _ := cmd.Flags().GetString("note")

	reqs := make([]app.ChangeStatusRequest, len(args))
	for i, path := range args {
		reqs[i] = app.ChangeStatusRequest{
			EntityPath:    path,
			Status:        inventory.EntityStatusLoaned,
			StatusContext: toFlag,
			Note:          noteFlag,
			ActorID:       cli.GetActorUserID(ctx),
		}
	}

	results, err := a.MarkLoaned(ctx, reqs)
	if err != nil {
		return fmt.Errorf("failed to mark loaned: %w", err)
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
		msg := fmt.Sprintf("%q marked as loaned", result.FullPathDisplay)
		if result.StatusContext != "" {
			msg += fmt.Sprintf(" (to: %s)", result.StatusContext)
		}
		out.Success(msg)
	}
	return nil
}
