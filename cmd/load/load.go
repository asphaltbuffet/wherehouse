package load

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
)

// NewLoadCmd returns the load command, initializing it if necessary.
func NewLoadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "load <file>...",
		Short:   "Load locations and items into wherehouse from CSV file(s)",
		Long:    "Load locations and items into wherehouse from CSV file(s).",
		Example: "wherehouse load ~/garage.csv ~/workshop.csv",
		Args:    cobra.MinimumNArgs(1),
		RunE:    runLoadCore,
	}

	return cmd
}

func runLoadCore(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cfg := cli.MustGetConfig(ctx)
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	var results []*cli.LoadResult
	var hardErrs []error

	for _, arg := range args {
		r, err := cli.LoadCSV(ctx, arg)
		if err != nil {
			hardErrs = append(hardErrs, fmt.Errorf("%s: %w", arg, err))
			continue
		}
		results = append(results, r)
	}

	// Report hard errors (file-level) to stderr.
	for _, err := range hardErrs {
		out.Error(err.Error())
	}

	if len(results) == 0 {
		return errors.New("no data loaded")
	}

	if cfg.IsJSON() {
		return out.JSON(results)
	}

	// Human-readable: summary line per file + invalid entry details.
	for _, r := range results {
		name := filepath.Base(r.Path)
		summary := fmt.Sprintf("%s: %d location(s), %d item(s)", name, r.LocationCount, r.ItemCount)
		if len(r.InvalidEntries) > 0 {
			summary += fmt.Sprintf(", %d invalid", len(r.InvalidEntries))
		}
		out.Println(summary)

		for _, inv := range r.InvalidEntries {
			out.Println(fmt.Sprintf("  Line %d: %q \u2014 %s", inv.Line, inv.Entry, inv.Error))
		}

		if len(r.InvalidEntries) > 0 {
			out.Println("")
		}
	}

	return nil
}
