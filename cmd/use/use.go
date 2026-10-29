package use

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

// NewDefaultUseCmd returns a use command that opens the database from context configuration at runtime.
func NewDefaultUseCmd() *cobra.Command {
	cmd := buildUseCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		s, a, err := cli.OpenDatabase(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer s.Close()
		return runUse(cmd, args, a)
	}
	return cmd
}

// NewUseCmd returns a use command bound to a. Intended for testing.
func NewUseCmd(a *app.App) *cobra.Command {
	cmd := buildUseCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return runUse(cmd, args, a) }
	return cmd
}

func buildUseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "use <path>",
		Short: "Temporarily move an entity, keeping its home",
		Long: `Temporarily move an entity somewhere else. Its home does not change, so it is
shown as displaced until you 'return' it (or 'move' it to make the new place its home).

The same rules as 'move' apply. The destination cannot be the entity's home;
use 'return' to send it back.

Examples:
  wherehouse use "Garage:Shelf:Drill" --to "Deck"
  wherehouse use "Garage:Shelf:Drill" --to "Deck" --note "deck project"`,
		Args: cobra.ExactArgs(1),
	}
	cmd.Flags().StringP("to", "t", "", "Destination parent path")
	_ = cmd.MarkFlagRequired("to")
	cmd.Flags().StringP("note", "n", "", "Optional note for the event")
	return cmd
}

func runUse(cmd *cobra.Command, args []string, a *app.App) error {
	ctx := cmd.Context()
	srcPath := args[0]
	destPath, _ := cmd.Flags().GetString("to")
	noteFlag, _ := cmd.Flags().GetString("note")

	src, err := a.LookupEntityByPath(ctx, srcPath)
	if err != nil {
		return fmt.Errorf("failed to find %q: %w", srcPath, err)
	}
	dest, err := a.LookupEntityByPath(ctx, destPath)
	if err != nil {
		return fmt.Errorf("failed to find destination %q: %w", destPath, err)
	}

	updated, err := a.UseEntity(ctx, app.UseEntityRequest{
		EntityID:    src.EntityID,
		NewParentID: dest.EntityID,
		ActorID:     cli.GetActorUserID(ctx),
		Note:        noteFlag,
	})
	if err != nil {
		return fmt.Errorf("failed to use %q: %w", srcPath, err)
	}

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)
	if out.IsJSON() {
		return out.JSON(app.ToMoveOutput(updated))
	}
	out.Success(fmt.Sprintf("Using %q at %s (home: %s)", srcPath, updated.FullPathDisplay, updated.HomeLabel()))
	return nil
}
