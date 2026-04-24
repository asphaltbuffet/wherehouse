package history

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

// NewDefaultHistoryCmd returns a history command that opens the database from context configuration at runtime.
func NewDefaultHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history <entity-id>",
		Short: "Show event history for an entity",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := cli.OpenDatabase(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to open database: %w", err)
			}
			defer db.Close()
			return runHistory(cmd, args, db)
		},
	}
	return cmd
}

// NewHistoryCmd returns a history command using the provided database. Intended for testing.
func NewHistoryCmd(db historyDB) *cobra.Command {
	return &cobra.Command{
		Use:   "history <entity-id>",
		Short: "Show event history for an entity",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHistory(cmd, args, db)
		},
	}
}

type historyEntry struct {
	EventID   int64  `json:"event_id"`
	EventType string `json:"event_type"`
	Timestamp string `json:"timestamp"`
	ActorUser string `json:"actor_user"`
}

func runHistory(cmd *cobra.Command, args []string, db historyDB) error {
	ctx := cmd.Context()
	entityID := args[0]

	events, err := db.GetEventsByEntity(ctx, entityID)
	if err != nil {
		return fmt.Errorf("failed to get history: %w", err)
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
