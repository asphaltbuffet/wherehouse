package scry

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// NewDefaultScryCmd returns a scry command that opens the database from context configuration at runtime.
func NewDefaultScryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scry [<name>]",
		Short: "Search for entities by name or list all",
		Long: `Search for entities by canonical name, or list all entities if no name is given.

Examples:
  wherehouse scry                  # List all entities
  wherehouse scry "toolbox"        # Find entities named "toolbox"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := cli.OpenDatabase(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to open database: %w", err)
			}
			defer db.Close()
			return runScry(cmd, args, db)
		},
	}
	return cmd
}

// NewScryCmd returns a scry command using the provided database. Intended for testing.
func NewScryCmd(db scryDB) *cobra.Command {
	return &cobra.Command{
		Use:   "scry [<name>]",
		Short: "Search for entities by name or list all",
		Long: `Search for entities by canonical name, or list all entities if no name is given.

Examples:
  wherehouse scry                  # List all entities
  wherehouse scry "toolbox"        # Find entities named "toolbox"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScry(cmd, args, db)
		},
	}
}

type scryEntry struct {
	EntityID string `json:"entity_id"`
	Path     string `json:"path"`
	Type     string `json:"type"`
	Status   string `json:"status"`
}

func runScry(cmd *cobra.Command, args []string, db scryDB) error {
	ctx := cmd.Context()

	var entities []*database.Entity
	var err error

	if len(args) == 1 {
		canonical := database.CanonicalizeString(args[0])
		entities, err = db.GetEntitiesByCanonicalName(ctx, canonical)
	} else {
		entities, err = db.ListEntities(ctx, "", "", "")
	}

	if err != nil {
		return fmt.Errorf("scry failed: %w", err)
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
