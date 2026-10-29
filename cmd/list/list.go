package listcmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
)

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

Use --under to restrict to children of a specific entity path, and --status to filter by lifecycle status.

Examples:
  wherehouse list                                    # All entities
  wherehouse list --under "Garage:Toolbox"           # Under a specific path
  wherehouse list --status missing
  wherehouse list --displaced                        # Entities away from their home
  wherehouse list --displaced --home-under "Garage"  # ...whose home is in the Garage`,
		Args: cobra.NoArgs,
	}
	cmd.Flags().String("under", "", "Restrict to entities under this path (e.g. Garage:Toolbox)")
	cmd.Flags().String("status", "", "Filter by status: ok, borrowed, missing, loaned, removed")
	cmd.Flags().Bool("displaced", false, "Only entities away from their home")
	cmd.Flags().String("home-under", "", "Only entities whose home is at or under this path")
	cmd.Flags().BoolP("verbose", "v", false, "Show entity ID and tags for each entity")
	return cmd
}

func runList(cmd *cobra.Command, _ []string, a *app.App) error {
	ctx := cmd.Context()
	underPath, _ := cmd.Flags().GetString("under")
	statusFilter, _ := cmd.Flags().GetString("status")
	verbose, _ := cmd.Flags().GetBool("verbose")

	displaced, _ := cmd.Flags().GetBool("displaced")
	homeUnderPath, _ := cmd.Flags().GetString("home-under")

	var underParsed, homeUnderParsed entitypath.Path
	if underPath != "" {
		var parseErr error

		underParsed, parseErr = entitypath.Parse(underPath)
		if parseErr != nil {
			return fmt.Errorf("invalid --under path: %w", parseErr)
		}
	}

	if homeUnderPath != "" {
		var parseErr error

		homeUnderParsed, parseErr = entitypath.Parse(homeUnderPath)
		if parseErr != nil {
			return fmt.Errorf("invalid --home-under path: %w", parseErr)
		}
	}

	all, err := a.ListEntities(ctx)
	if err != nil {
		return fmt.Errorf("list failed: %w", err)
	}

	matched := filterEntities(all, listFilter{
		under:     underParsed,
		homeUnder: homeUnderParsed,
		status:    statusFilter,
		displaced: displaced,
	})

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	return out.Render(app.ToListItems(matched), func() string {
		return buildTree(all, matched, verbose)
	})
}

// listFilter is the AND of list's filter flags; zero values match everything.
type listFilter struct {
	under     entitypath.Path
	homeUnder entitypath.Path
	status    string
	displaced bool
}

func filterEntities(all []app.EntityResult, f listFilter) []app.EntityResult {
	var out []app.EntityResult

	for _, e := range all {
		if f.status != "" && e.Status.String() != f.status {
			continue
		}

		if f.displaced && !e.IsDisplaced() {
			continue
		}

		if !f.under.IsEmpty() && !pathHasPrefix(e.FullPathDisplay, f.under) {
			continue
		}

		if !f.homeUnder.IsEmpty() && !pathHasPrefix(homePath(e), f.homeUnder) {
			continue
		}

		out = append(out, e)
	}

	return out
}

// homePath is the display path of e's home: its parent's path when at home.
// Root-homed entities yield "" and never match a prefix.
func homePath(e app.EntityResult) string {
	if e.IsDisplaced() {
		return e.HomePathDisplay
	}

	p, err := entitypath.Parse(e.FullPathDisplay)
	if err != nil {
		return ""
	}

	return p.Dir().String()
}

func pathHasPrefix(p string, prefix entitypath.Path) bool {
	ep, err := entitypath.Parse(p)
	if err != nil {
		return false
	}

	return ep.HasPrefix(prefix)
}
