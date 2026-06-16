package history

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

// NewDefaultHistoryCmd returns the history command wired to the real database.
func NewDefaultHistoryCmd() *cobra.Command {
	cmd := buildHistoryCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		s, a, err := cli.OpenDatabase(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer s.Close()
		return runHistory(cmd, args, a)
	}
	return cmd
}

// NewHistoryCmd returns the history command using the supplied historyApp (for testing).
func NewHistoryCmd(a *app.App) *cobra.Command {
	cmd := buildHistoryCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return runHistory(cmd, args, a) }
	return cmd
}

func buildHistoryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "history <path>",
		Short: "Show event history for an entity",
		Long: `Show the event history for an entity identified by its full colon-separated path.

Examples:
  wherehouse history "Garage:Toolbox:Wrench"`,
		Args: cobra.ExactArgs(1),
	}
}

func runHistory(cmd *cobra.Command, args []string, a *app.App) error {
	ctx := cmd.Context()
	path := args[0]

	entity, err := a.LookupEntityByPath(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to find %q: %w", path, err)
	}

	events, err := a.GetHistory(ctx, app.GetHistoryRequest{EntityID: entity.EntityID})
	if err != nil {
		return fmt.Errorf("failed to get history for %q: %w", path, err)
	}

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)
	return out.Render(app.ToHistoryItems(events), func() string {
		var b strings.Builder
		for _, e := range events {
			fmt.Fprintf(&b, "%d  %s  %s  %s\n",
				e.EventID, e.TimestampUTC, e.EventType, e.ActorUserID)
		}
		return b.String()
	})
}
