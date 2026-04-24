package list

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

// NewDefaultListCmd returns a list command that opens the database from context configuration at runtime.
func NewDefaultListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List entities in the inventory",
		Long: `List entities in the inventory.

Use --under to restrict to children of a specific entity, --type to filter
by entity type, and --status to filter by lifecycle status.

Examples:
  wherehouse list                           # All entities
  wherehouse list --under <id>              # Under a specific parent
  wherehouse list --type container          # Containers only
  wherehouse list --status missing`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := cli.OpenDatabase(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to open database: %w", err)
			}
			defer db.Close()
			return runList(cmd, args, db)
		},
	}
	cmd.Flags().String("under", "", "Restrict to entities under this entity ID")
	cmd.Flags().String("type", "", "Filter by type: place, container, or leaf")
	cmd.Flags().String("status", "", "Filter by status: ok, borrowed, missing, loaned, removed")
	return cmd
}

// NewListCmd returns a list command using the provided database. Intended for testing.
func NewListCmd(db listDB) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List entities in the inventory",
		Long: `List entities in the inventory.

Use --under to restrict to children of a specific entity, --type to filter
by entity type, and --status to filter by lifecycle status.

Examples:
  wherehouse list                           # All entities
  wherehouse list --under <id>              # Under a specific parent
  wherehouse list --type container          # Containers only
  wherehouse list --status missing`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, args, db)
		},
	}
	cmd.Flags().String("under", "", "Restrict to entities under this entity ID")
	cmd.Flags().String("type", "", "Filter by type: place, container, or leaf")
	cmd.Flags().String("status", "", "Filter by status: ok, borrowed, missing, loaned, removed")
	return cmd
}

type listEntry struct {
	EntityID string `json:"entity_id"`
	Path     string `json:"path"`
	Type     string `json:"type"`
	Status   string `json:"status"`
}

func runList(cmd *cobra.Command, _ []string, db listDB) error {
	ctx := cmd.Context()
	underID, _ := cmd.Flags().GetString("under")
	typeFilter, _ := cmd.Flags().GetString("type")
	statusFilter, _ := cmd.Flags().GetString("status")

	entities, err := db.ListEntities(ctx, underID, typeFilter, statusFilter)
	if err != nil {
		return fmt.Errorf("list failed: %w", err)
	}

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	if cfg.IsJSON() {
		entries := make([]listEntry, len(entities))
		for i, e := range entities {
			entries[i] = listEntry{
				EntityID: e.EntityID,
				Path:     e.FullPathDisplay,
				Type:     e.EntityType.String(),
				Status:   e.Status.String(),
			}
		}
		return out.JSON(entries)
	}

	for _, e := range entities {
		fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  [%s] (%s)\n",
			e.EntityID, e.FullPathDisplay, e.EntityType, e.Status)
	}
	return nil
}
