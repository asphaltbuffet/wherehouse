package status

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// NewDefaultStatusCmd returns a status command that opens the database from context configuration at runtime.
func NewDefaultStatusCmd() *cobra.Command {
	cmd := buildStatusCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		s, a, err := cli.OpenDatabase(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer s.Close()
		return runStatus(cmd, args, a)
	}
	return cmd
}

// NewStatusCmd returns a status command using the provided statusApp. Intended for testing.
func NewStatusCmd(a *app.App) *cobra.Command {
	cmd := buildStatusCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return runStatus(cmd, args, a) }
	return cmd
}

func buildStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <path>",
		Short: "Change the status of an entity",
		Long: `Change the lifecycle status of an entity identified by its full path.

Valid statuses: ok, borrowed, missing, loaned, removed

Examples:
  wherehouse status "Garage:Toolbox:Wrench" --set borrowed --note "loaned to Bob"
  wherehouse status "Garage:Toolbox:Wrench" --set ok`,
		Args: cobra.ExactArgs(1),
	}
	cmd.Flags().StringP("set", "s", "", "New status value (REQUIRED): ok, borrowed, missing, loaned, removed")
	_ = cmd.MarkFlagRequired("set")
	cmd.Flags().StringP("note", "n", "", "Optional context note for the status change")
	return cmd
}

func runStatus(cmd *cobra.Command, args []string, a *app.App) error {
	ctx := cmd.Context()
	path := args[0]
	setFlag, _ := cmd.Flags().GetString("set")
	noteFlag, _ := cmd.Flags().GetString("note")

	newStatus, err := inventory.ParseEntityStatus(setFlag)
	if err != nil {
		return err
	}

	result, err := a.ChangeStatus(ctx, app.ChangeStatusRequest{
		EntityPath:    path,
		Status:        newStatus,
		StatusContext: noteFlag,
		Note:          noteFlag,
		ActorID:       cli.GetActorUserID(ctx),
	})
	if err != nil {
		return fmt.Errorf("failed to update status of %q: %w", path, err)
	}

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)
	if out.IsJSON() {
		return out.JSON(app.ToStatusOutput(result))
	}
	msg := fmt.Sprintf("Status of %q set to %s", result.FullPathDisplay, result.Status)
	if result.StatusContext != "" {
		msg += fmt.Sprintf(" (%s)", result.StatusContext)
	}
	out.Success(msg)
	return nil
}
