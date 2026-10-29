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
		Short: "Return entities: back from a loan, back to the lender, or back home",
		Long: `Return one or more entities. What happens depends on the entity's state.

A loaned entity is marked ok again, and if it is also displaced it goes back to
its home. A borrowed entity is removed from the inventory instead; a borrowed
entity that still contains other entities cannot be returned, so return or move
them first. A displaced entity (one that was 'use'd) is moved back to its home.
An entity that is ok and already at home, or that is missing, cannot be returned.

All entities are updated atomically — if any path fails, none are changed.

Examples:
  wherehouse return "Garage:Toolbox:Wrench"
  wherehouse return "Deck:Drill"   # displaced → back to its home
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
		entity, err := a.LookupEntityByPath(ctx, path)
		if err != nil {
			return fmt.Errorf("failed to find %q: %w", path, err)
		}
		reqs[i] = app.ChangeStatusRequest{
			EntityID: entity.EntityID,
			Note:     noteFlag,
			ActorID:  cli.GetActorUserID(ctx),
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
