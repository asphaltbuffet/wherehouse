package status

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

// NewDefaultStatusCmd returns a status command that opens the database from context configuration at runtime.
func NewDefaultStatusCmd() *cobra.Command {
	cmd := buildStatusCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		s, a, err := cli.OpenDatabase(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer s.Close()
		return runStatus(cmd, args, a)
	}
	return cmd
}

// NewStatusCmd returns a status command using the provided statusApp. Intended for testing.
func NewStatusCmd(a *app.App) *cobra.Command {
	cmd := buildStatusCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return runStatus(cmd, args, a) }
	return cmd
}

func buildStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <path>",
		Short: "Show the status of an entity",
		Long: `Show the lifecycle status of an entity identified by its full path.

Includes removed entities. If multiple entities have shared the same path
(e.g. after a remove and re-add), all are shown ranked most-recent first.

Examples:
  wherehouse status "Garage:Toolbox:Wrench"
  wherehouse status "Garage:Toolbox:Wrench" --json`,
		Args: cobra.ExactArgs(1),
	}
}

func runStatus(cmd *cobra.Command, args []string, a *app.App) error {
	ctx := cmd.Context()
	path := args[0]

	results, err := a.LookupEntityStatus(ctx, path)
	if err != nil {
		return fmt.Errorf("status %q: %w", path, err)
	}

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	return out.Render(app.ToStatusOutputs(results), func() string {
		var sb strings.Builder
		for _, r := range results {
			line := fmt.Sprintf("%s: %s", r.FullPathDisplay, r.Status)
			if r.StatusContext != "" {
				line += fmt.Sprintf(" (%s)", r.StatusContext)
			}
			sb.WriteString(line + "\n")
		}
		return strings.TrimRight(sb.String(), "\n")
	})
}
