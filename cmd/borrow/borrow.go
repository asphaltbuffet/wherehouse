package borrow

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
)

// NewDefaultBorrowCmd returns a borrow command that opens the database from context configuration at runtime.
func NewDefaultBorrowCmd() *cobra.Command {
	cmd := buildBorrowCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		s, a, err := cli.OpenDatabase(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer s.Close()
		return runBorrow(cmd, args, a)
	}
	return cmd
}

// NewBorrowCmd returns a borrow command backed by the provided App (for tests).
func NewBorrowCmd(a *app.App) *cobra.Command {
	cmd := buildBorrowCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return runBorrow(cmd, args, a) }
	return cmd
}

func buildBorrowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "borrow <path>...",
		Short: "Add one or more borrowed entities to the inventory",
		Long: `Create one or more entities in borrowed status. Borrowed entities are externally owned
and can only be closed out via 'return', which removes them from the inventory.

All entities are created atomically — if any path fails, none are created.

Examples:
  wherehouse borrow "Garage:Alice's Drill" --from "Alice"
  wherehouse borrow "Shelf:Bob's Ladder" "Shelf:Bob's Saw" --from "Bob" --note "borrowed for the weekend"`,
		Args: cobra.MinimumNArgs(1),
	}
	cmd.Flags().StringP("from", "f", "", "Who the entity is borrowed from")
	_ = cmd.MarkFlagRequired("from")
	cmd.Flags().StringP("note", "n", "", "Optional note for the event")
	return cmd
}

func runBorrow(cmd *cobra.Command, args []string, a *app.App) error {
	ctx := cmd.Context()
	fromFlag, _ := cmd.Flags().GetString("from")
	noteFlag, _ := cmd.Flags().GetString("note")

	reqs := make([]app.BorrowEntityRequest, 0, len(args))
	for _, arg := range args {
		p, err := entitypath.Parse(arg)
		if err != nil {
			return fmt.Errorf("failed to parse path %q: %w", arg, err)
		}
		name := p.Base()
		if name == "" {
			return fmt.Errorf("cannot determine entity name from %q", arg)
		}
		reqs = append(reqs, app.BorrowEntityRequest{
			DisplayName:   name,
			ParentPath:    p.Dir().String(),
			StatusContext: fromFlag,
			ActorID:       cli.GetActorUserID(ctx),
			Note:          noteFlag,
		})
	}

	results, err := a.BorrowEntities(ctx, reqs)
	if err != nil {
		return fmt.Errorf("failed to borrow: %w", err)
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
		msg := fmt.Sprintf("Borrowed %q", result.FullPathDisplay)
		if result.StatusContext != "" {
			msg += fmt.Sprintf(" (from: %s)", result.StatusContext)
		}
		out.Success(msg)
	}
	return nil
}
