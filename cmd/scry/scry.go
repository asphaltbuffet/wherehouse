package scry

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

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
	cmd := &cobra.Command{
		Use:   "scry [<name>]",
		Short: "Search for entities by name or list all",
		Long: `Search for entities by name, or list all entities if no name is given.

Examples:
  wherehouse scry                  # List all entities
  wherehouse scry "toolbox"        # Find entities matching "toolbox"
  wherehouse scry -v "toolbox"     # Include Levenshtein distance in output`,
		Args: cobra.MaximumNArgs(1),
	}
	cmd.Flags().BoolP("verbose", "v", false, "Show Levenshtein distance in search results")
	return cmd
}

func runScry(cmd *cobra.Command, args []string, a *app.App) error {
	ctx := cmd.Context()

	var results []app.FindResult
	if len(args) == 1 {
		found, err := a.FindEntities(ctx, app.FindEntitiesRequest{Query: args[0]})
		if err != nil {
			return fmt.Errorf("scry failed: %w", err)
		}
		results = found
	} else {
		entities, err := a.ListEntities(ctx)
		if err != nil {
			return fmt.Errorf("scry failed: %w", err)
		}
		results = make([]app.FindResult, len(entities))
		for i, e := range entities {
			results[i] = app.FindResult{Entity: e}
		}
	}

	verbose, _ := cmd.Flags().GetBool("verbose")
	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)
	return out.Render(app.ToScryItems(results, len(args) == 1), func() string {
		var b strings.Builder
		for _, r := range results {
			if len(args) == 1 && verbose {
				fmt.Fprintf(&b, "%s  %s  [%s] dist:%d\n",
					r.Entity.EntityID, r.Entity.FullPathDisplay, r.Entity.Status, r.Distance)
			} else {
				fmt.Fprintf(&b, "%s  %s  [%s]\n",
					r.Entity.EntityID, r.Entity.FullPathDisplay, r.Entity.Status)
			}
		}
		return b.String()
	})
}
