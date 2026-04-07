package cli

import (
	"context"

	"github.com/spf13/cobra"
)

// LocationCompletions returns the full canonical paths of all non-system
// locations for use as shell completions. It opens its own database connection
// via OpenDatabase(ctx) so that it can be called from cobra RegisterFlagCompletionFunc
// handlers, which run before RunE and outside the command's normal DB lifecycle.
//
// On success it returns (paths, ShellCompDirectiveNoFileComp).
// On any error it returns (nil, ShellCompDirectiveError) so that the shell
// silently offers no completions rather than printing an error.
func LocationCompletions(ctx context.Context) ([]string, cobra.ShellCompDirective) {
	db, err := OpenDatabase(ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	defer db.Close()

	locs, err := db.GetAllLocations(ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var completions []string
	for _, loc := range locs {
		if loc.IsSystem {
			continue
		}
		completions = append(completions, loc.FullPathCanonical)
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}
