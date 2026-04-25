package add

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/database"
	"github.com/asphaltbuffet/wherehouse/internal/nanoid"
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
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := cli.OpenDatabase(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to open database: %w", err)
			}
			defer db.Close()
			return runAdd(cmd, args, db)
		},
	}
	cmd.Flags().StringP("in", "i", "", "Parent entity ID or unambiguous name")
	cmd.Flags().StringP("type", "t", "container", "Entity type: place, container, or leaf")
	return cmd
}

// NewAddCmd returns the add command with the given DB (for testing).
func NewAddCmd(db addDB) *cobra.Command {
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
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(cmd, args, db)
		},
	}
	cmd.Flags().StringP("in", "i", "", "Parent entity ID or unambiguous name")
	cmd.Flags().StringP("type", "t", "container", "Entity type: place, container, or leaf")
	return cmd
}

type addResult struct {
	EntityID string `json:"entity_id"`
	Path     string `json:"path"`
}

func runAdd(cmd *cobra.Command, args []string, db addDB) error {
	ctx := cmd.Context()
	name := args[0]

	inFlag, _ := cmd.Flags().GetString("in")
	typeFlag, _ := cmd.Flags().GetString("type")

	entityType, err := database.ParseEntityType(typeFlag)
	if err != nil {
		return err
	}

	var parentID *string
	if inFlag != "" {
		resolved, resolveErr := resolveParent(ctx, db, inFlag)
		if resolveErr != nil {
			return resolveErr
		}
		parentID = &resolved
	}

	entityID := nanoid.MustNew()
	actorUserID := cli.GetActorUserID(ctx)

	payload := map[string]any{
		"entity_id":    entityID,
		"display_name": name,
		"entity_type":  entityType.String(),
		"parent_id":    parentID,
	}

	if _, err = db.AppendEvent(ctx, database.EntityCreatedEvent, actorUserID, payload, ""); err != nil {
		return fmt.Errorf("failed to create entity: %w", err)
	}

	entity, err := db.GetEntity(ctx, entityID)
	if err != nil {
		return fmt.Errorf("failed to retrieve created entity: %w", err)
	}

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}

	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	if cfg.IsJSON() {
		return out.JSON(addResult{EntityID: entityID, Path: entity.FullPathDisplay})
	}

	out.Success(fmt.Sprintf("Added %q (%s) at path %s", name, entityType, entity.FullPathDisplay))
	out.KeyValue("ID", entityID)

	return nil
}

// resolveParent resolves the --in flag value to an entity ID.
// Accepts a nanoid directly (ID lookup), or a canonical name if unambiguous.
func resolveParent(ctx context.Context, db addDB, input string) (string, error) {
	// Try direct ID lookup first.
	if e, err := db.GetEntity(ctx, input); err == nil {
		return e.EntityID, nil
	}

	// Fall back to name lookup.
	canonical := database.CanonicalizeString(input)
	matches, err := db.GetEntitiesByCanonicalName(ctx, canonical)
	if err != nil {
		return "", fmt.Errorf("resolve parent %q: %w", input, err)
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no entity found with name or ID %q", input)
	case 1:
		return matches[0].EntityID, nil
	default:
		return "", fmt.Errorf("ambiguous parent %q: %d entities match; use an entity ID instead", input, len(matches))
	}
}
