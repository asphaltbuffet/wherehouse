package list

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
)

type listEntry struct {
	EntityID string `json:"entity_id"`
	Path     string `json:"path"`
	Type     string `json:"type"`
	Status   string `json:"status"`
}

// NewDefaultListCmd returns the list command wired to the real database.
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

// NewListCmd returns the list command using the supplied listApp (for testing).
func NewListCmd(a *app.App) *cobra.Command {
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

func runList(cmd *cobra.Command, _ []string, a *app.App) error {
	ctx := cmd.Context()
	underPath, _ := cmd.Flags().GetString("under")
	typeFilter, _ := cmd.Flags().GetString("type")
	statusFilter, _ := cmd.Flags().GetString("status")

	var underParsed entitypath.Path
	if underPath != "" {
		var parseErr error

		underParsed, parseErr = entitypath.Parse(underPath)
		if parseErr != nil {
			return fmt.Errorf("invalid --under path: %w", parseErr)
		}
	}

	all, err := a.ListEntities(ctx)
	if err != nil {
		return fmt.Errorf("list failed: %w", err)
	}

	entities := filterEntities(all, underParsed, typeFilter, statusFilter)

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

func filterEntities(all []app.EntityResult, under entitypath.Path, typeFilter, statusFilter string) []app.EntityResult {
	var out []app.EntityResult

	for _, e := range all {
		if typeFilter != "" && e.EntityType.String() != typeFilter {
			continue
		}

		if statusFilter != "" && e.Status.String() != statusFilter {
			continue
		}

		if !under.IsEmpty() {
			ep, parseErr := entitypath.Parse(e.FullPathDisplay)
			if parseErr != nil {
				continue
			}

			if !under.IsAncestor(ep) {
				continue
			}
		}

		out = append(out, e)
	}

	return out
}
