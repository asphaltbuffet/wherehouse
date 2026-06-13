package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/tui"
)

// NewTUICmd returns a cobra.Command that runs the TUI against the provided app.
func NewTUICmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		RunE: func(_ *cobra.Command, _ []string) error {
			return runTUI(a)
		},
	}
}

// NewDefaultTUICmd returns a cobra.Command that opens its own DB connection.
func NewDefaultTUICmd() *cobra.Command {
	return &cobra.Command{
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, a, err := cli.OpenDatabase(cmd.Context())
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer s.Close()
			return runTUI(a)
		},
	}
}

func runTUI(a *app.App) error {
	m := tui.New(a)
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}
