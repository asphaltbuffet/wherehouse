package add

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/database"
	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
)

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
  wherehouse add "Wrench" --in <parent-id>           # Add under a parent entity`,
		Args: cobra.ExactArgs(1),
		RunE: runAdd,
	}
	cmd.Flags().StringP("in", "i", "", "Parent entity ID or unambiguous name")
	cmd.Flags().StringP("type", "t", "container", "Entity type: place, container, or leaf")
	return cmd
}

type addResult struct {
	EntityID string `json:"entity_id"`
	Path     string `json:"path"`
}

func runAdd(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	p, err := entitypath.Parse(args[0])
	if err != nil {
		return fmt.Errorf("failed to add %q: %w", args[0], err)
	}

	wh, err := cli.NewApp(ctx)
	if err != nil {
		return err
	}

	typeFlag, _ := cmd.Flags().GetString("type")

	entityType, err := database.ParseEntityType(typeFlag)
	if err != nil {
		return err
	}

	e, err := wh.AddItem(p, entityType)
	if err != nil {
		return err
	}

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}

	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	if cfg.IsJSON() {
		return out.JSON(
			addResult{
				EntityID: e.EntityID,
				Path:     e.FullPathDisplay,
			},
		)
	}

	out.Success(fmt.Sprintf(
		"Added %q (%s) at path %s",
		p,
		entityType,
		e.FullPathDisplay,
	))

	out.KeyValue("ID", e.EntityID)

	return nil
}
