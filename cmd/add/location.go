package add

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
)

// NewAddLocationCmd subcommand.
func NewAddLocationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "location <name>...",
		Short: "Add one or more locations",
		Long: `Add one or more locations to the hierarchy.

If --in is specified, locations are created as children of that parent.
Otherwise, locations are created at the root level.

Each location receives a unique ID and is validated for name uniqueness.

Examples:
  wherehouse add location Garage            # Create root location
  wherehouse add location Shelf --in Garage # Create child location
  wherehouse add location "Shelf A" "Shelf B" --in Garage # Multiple locations`,
		Args: cobra.MinimumNArgs(1), // Require at least one location name
		RunE: runAddLocation,
	}

	cmd.Flags().StringP("in", "i", "", "Parent location name or ID (optional, omit for root)")
	_ = cmd.RegisterFlagCompletionFunc(
		"in",
		func(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			return cli.LocationCompletions(cmd.Context())
		},
	)

	return cmd
}

// runAddLocation implements the add location command logic.
func runAddLocation(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	parentInput, _ := cmd.Flags().GetString("in")

	cfg := cli.MustGetConfig(ctx)
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	results, err := cli.AddLocations(ctx, args, parentInput)
	if err != nil {
		return err
	}

	for _, r := range results {
		if r.FullPathDisplay != "" {
			out.Success(fmt.Sprintf("Added location %q (path: %s)", r.DisplayName, r.FullPathDisplay))
		} else {
			out.Success(fmt.Sprintf("Added location %q (id: %s)", r.DisplayName, r.LocationID))
		}
	}

	return nil
}
