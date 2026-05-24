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

type addResult struct {
	EntityID string `json:"entity_id"`
	Path     string `json:"path"`
}

// NewDefaultAddCmd returns the add command wired to a real database opened from context config.
func NewDefaultAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add an entity to the inventory",
		Long: `Add a new entity. By default, entities are containers (movable, can hold things).
Use --type place for immovable locations like rooms or shelves.

Examples:
  wherehouse add "Toolbox"                           # Add a container
  wherehouse add "Garage" --type place               # Add a place
  wherehouse add "Garage:Toolbox:Wrench"             # Add nested by path`,
		Args: cobra.ExactArgs(1),
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
	return cmd
}

// NewAddCmd returns the add command wired to the provided addApp (for testing).
func NewAddCmd(a addApp) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add an entity to the inventory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(cmd, args, a)
		},
	}
	cmd.Flags().StringP("type", "t", "container", "Entity type: place, container, or leaf")
	return cmd
}

func runAdd(cmd *cobra.Command, args []string, a addApp) error {
	ctx := cmd.Context()

	p, err := entitypath.Parse(args[0])
	if err != nil {
		return fmt.Errorf("failed to add %q: %w", args[0], err)
	}

	typeFlag, _ := cmd.Flags().GetString("type")
	entityType, err := inventory.ParseEntityType(typeFlag)
	if err != nil {
		return err
	}

	name := p.Base()
	if name == "" {
		return fmt.Errorf("cannot determine entity name from %q", args[0])
	}
	parentPath := p.Dir().String()

	result, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: name,
		EntityType:  entityType,
		ParentPath:  parentPath,
		ActorID:     cli.GetActorUserID(ctx),
	})
	if err != nil {
		return err
	}

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	if cfg.IsJSON() {
		return out.JSON(addResult{EntityID: result.EntityID, Path: result.FullPathDisplay})
	}

	out.Success(fmt.Sprintf("Added %q (%s) at path %s", p, entityType, result.FullPathDisplay))
	out.KeyValue("ID", result.EntityID)
	return nil
}
