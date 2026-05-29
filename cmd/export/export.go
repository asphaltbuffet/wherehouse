package export

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
)

// NewDefaultExportCmd returns the export command wired to the real database.
func NewDefaultExportCmd() *cobra.Command {
	cmd := buildExportCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		s, a, err := cli.OpenDatabase(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer s.Close()
		return runExport(cmd, args, a)
	}
	return cmd
}

// NewExportCmd returns the export command using the supplied exportApp (for testing).
func NewExportCmd(a exportApp) *cobra.Command {
	cmd := buildExportCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return runExport(cmd, args, a) }
	return cmd
}

func buildExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export",
		Short: "Export all events as JSON",
		Long: `Export all events from the event log as JSON.

Examples:
  wherehouse export`,
	}
}

func runExport(cmd *cobra.Command, _ []string, a exportApp) error {
	events, err := a.GetAllEvents(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to export events: %w", err)
	}

	if len(events) == 0 {
		if !cli.IsQuietMode(cmd) {
			fmt.Fprintln(cmd.ErrOrStderr(), "warning: no events found")
		}
		return nil
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	for _, ev := range events {
		err = enc.Encode(ev)
		if err != nil {
			return fmt.Errorf("encoding event %d: %w", ev.EventID, err)
		}
	}

	return nil
}
