package scry

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

type scryEntry struct {
	EntityID string `json:"entity_id"`
	Path     string `json:"path"`
	Type     string `json:"type"`
	Status   string `json:"status"`
}

// NewDefaultScryCmd returns the scry command wired to the real database.
func NewDefaultScryCmd() *cobra.Command {
	cmd := buildScryCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		s, a, err := cli.OpenDatabase(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer s.Close()
		return runScry(cmd, args, a)
	}
	return cmd
}

// NewScryCmd returns the scry command using the supplied scryApp (for testing).
func NewScryCmd(a *app.App) *cobra.Command {
	cmd := buildScryCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return runScry(cmd, args, a) }
	return cmd
}

func buildScryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "scry [<name>]",
		Short: "Search for entities by name or list all",
		Long: `Search for entities by name, or list all entities if no name is given.

Examples:
  wherehouse scry                  # List all entities
  wherehouse scry "toolbox"        # Find entities matching "toolbox"`,
		Args: cobra.MaximumNArgs(1),
	}
}

func runScry(cmd *cobra.Command, args []string, a *app.App) error {
	ctx := cmd.Context()

	var entities []app.EntityResult
	if len(args) == 1 {
		results, err := a.FindEntities(ctx, app.FindEntitiesRequest{Query: args[0]})
		if err != nil {
			return fmt.Errorf("scry failed: %w", err)
		}
		entities = make([]app.EntityResult, len(results))
		for i, r := range results {
			entities[i] = r.Entity
		}
	} else {
		var err error
		entities, err = a.ListEntities(ctx)
		if err != nil {
			return fmt.Errorf("scry failed: %w", err)
		}
	}

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)
	if cfg.IsJSON() {
		entries := make([]scryEntry, len(entities))
		for i, e := range entities {
			entries[i] = scryEntry{
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
