package list

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

type listEntry struct {
	EntityID string `json:"entity_id"`
	Path     string `json:"path"`
	Type     string `json:"type"`
	Status   string `json:"status"`
}

func NewDefaultListCmd() *cobra.Command {
	cmd := buildListCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		s, a, err := cli.OpenDatabase(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer s.Close()
		return runList(cmd, args, a)
	}
	return cmd
}

func NewListCmd(a listApp) *cobra.Command {
	cmd := buildListCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return runList(cmd, args, a) }
	return cmd
}

func buildListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List entities in the inventory",
		Long: `List entities in the inventory.

Use --under to restrict to children of a specific entity path, --type to filter
by entity type, and --status to filter by lifecycle status.

Examples:
  wherehouse list                                    # All entities
  wherehouse list --under "Garage:Toolbox"           # Under a specific path
  wherehouse list --type container                   # Containers only
  wherehouse list --status missing`,
		Args: cobra.NoArgs,
	}
	cmd.Flags().String("under", "", "Restrict to entities under this path (e.g. Garage:Toolbox)")
	cmd.Flags().String("type", "", "Filter by type: place, container, or leaf")
	cmd.Flags().String("status", "", "Filter by status: ok, borrowed, missing, loaned, removed")
	return cmd
}

func runList(cmd *cobra.Command, _ []string, a listApp) error {
	ctx := cmd.Context()
	underPath, _ := cmd.Flags().GetString("under")
	typeFilter, _ := cmd.Flags().GetString("type")
	statusFilter, _ := cmd.Flags().GetString("status")

	var underPrefix string
	if underPath != "" {
		underPrefix = underPath + ":"
	}

	all, err := a.ListEntities(ctx)
	if err != nil {
		return fmt.Errorf("list failed: %w", err)
	}

	var entities []app.EntityResult
	for _, e := range all {
		if underPrefix != "" && !strings.HasPrefix(e.FullPathDisplay, underPrefix) {
			continue
		}
		if typeFilter != "" && e.EntityType.String() != typeFilter {
			continue
		}
		if statusFilter != "" && e.Status.String() != statusFilter {
			continue
		}
		entities = append(entities, e)
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