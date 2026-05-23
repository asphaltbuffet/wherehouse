package history

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

type historyEntry struct {
	EventID   int64  `json:"event_id"`
	EventType string `json:"event_type"`
	Timestamp string `json:"timestamp"`
	ActorUser string `json:"actor_user"`
}

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

func NewHistoryCmd(a historyApp) *cobra.Command {
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

func runHistory(cmd *cobra.Command, args []string, a historyApp) error {
	ctx := cmd.Context()
	events, err := a.GetHistory(ctx, app.GetHistoryRequest{EntityPath: args[0]})
	if err != nil {
		return fmt.Errorf("failed to get history for %q: %w", args[0], err)
	}

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)
	if cfg.IsJSON() {
		entries := make([]historyEntry, len(events))
		for i, e := range events {
			entries[i] = historyEntry{
				EventID:   e.EventID,
				EventType: e.EventType.String(),
				Timestamp: e.TimestampUTC,
				ActorUser: e.ActorUserID,
			}
		}
		return out.JSON(entries)
	}
	for _, e := range events {
		fmt.Fprintf(cmd.OutOrStdout(), "%d  %s  %s  %s\n",
			e.EventID, e.TimestampUTC, e.EventType, e.ActorUserID)
	}
	return nil
}
