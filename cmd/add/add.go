package add

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// NewDefaultAddCmd returns the add command wired to a real database opened from context config.
func NewDefaultAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <path> [path...]",
		Short: "Add one or more entities to the inventory",
		Long: `Add one or more entities. By default, entities are containers (movable, can hold things).
Use --type place for immovable locations like rooms or shelves.

Examples:
  wherehouse add "Toolbox"                                         # Add a container
  wherehouse add "Garage" --type place                             # Add a place
  wherehouse add "Garage:Toolbox:Wrench"                           # Add nested by path
  wherehouse add "Basement:Game Shelf:"{Sorry,Monopoly,Chess}      # Add multiple via shell expansion`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, a, err := cli.OpenDatabase(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to open database: %w", err)
			}
			defer s.Close()
			return runAdd(cmd, args, a)
		},
	}
	cmd.Flags().StringP("type", "t", "container", "Entity type: place, container, or leaf")
	cmd.Flags().Bool("allow-duplicates", false, "Allow duplicate names within the batch")
	return cmd
}

// NewAddCmd returns the add command wired to the provided addApp (for testing).
func NewAddCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <path> [path...]",
		Short: "Add one or more entities to the inventory",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(cmd, args, a)
		},
	}
	cmd.Flags().Bool("locked", false, "Lock the entity (prevent direct reparenting)")
	cmd.Flags().Bool("discrete", false, "Mark the entity as discrete (no children allowed)")
	cmd.Flags().Bool("allow-duplicates", false, "Allow duplicate names within the batch")
	return cmd
}

func runAdd(cmd *cobra.Command, args []string, a *app.App) error {
	ctx := cmd.Context()

	locked, _ := cmd.Flags().GetBool("locked")
	discrete, _ := cmd.Flags().GetBool("discrete")
	allowDupes, _ := cmd.Flags().GetBool("allow-duplicates")

	if !allowDupes {
		seen := make(map[string]struct{}, len(args))
		for _, arg := range args {
			p, parseErr := entitypath.Parse(arg)
			if parseErr != nil {
				return fmt.Errorf("failed to add %q: %w", arg, parseErr)
			}
			key := inventory.CanonicalizeString(p.Dir().String()) + ":" + inventory.CanonicalizeString(p.Base())
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate name %q in batch (use --allow-duplicates to override)", p.Base())
			}
			seen[key] = struct{}{}
		}
	}

	reqs := make([]app.CreateEntityRequest, 0, len(args))
	for _, arg := range args {
		p, parseErr := entitypath.Parse(arg)
		if parseErr != nil {
			return fmt.Errorf("failed to add %q: %w", arg, parseErr)
		}
		name := p.Base()
		if name == "" {
			return fmt.Errorf("cannot determine entity name from %q", arg)
		}
		reqs = append(reqs, app.CreateEntityRequest{
			DisplayName: name,
			Locked:      locked,
			Discrete:    discrete,
			ParentPath:  p.Dir().String(),
			ActorID:     cli.GetActorUserID(ctx),
		})
	}

	results, err := a.CreateEntities(ctx, reqs)
	if err != nil {
		return err
	}

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	if out.IsJSON() {
		return out.JSON(app.ToAddOutputs(results))
	}

	for _, result := range results {
		out.Success(fmt.Sprintf("Added %q ID: %s", result.FullPathDisplay, result.EntityID))
	}
	return nil
}
