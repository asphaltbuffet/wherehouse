package info

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// NewDefaultInfoCmd wires the info command for production use.
func NewDefaultInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show database information and entity counts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, a, err := cli.OpenDatabase(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to open database: %w", err)
			}
			defer s.Close()
			return runInfo(cmd, a)
		},
	}
	return cmd
}

// NewInfoCmd creates the `info` command injected with a.
func NewInfoCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show database information and entity counts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInfo(cmd, a)
		},
	}
	return cmd
}

var statusOrder = []string{
	inventory.EntityStatusOk.String(),
	inventory.EntityStatusMissing.String(),
	inventory.EntityStatusBorrowed.String(),
	inventory.EntityStatusLoaned.String(),
	inventory.EntityStatusRemoved.String(),
}

func runInfo(cmd *cobra.Command, a *app.App) error {
	ctx := cmd.Context()

	result, err := a.GetInfo(ctx)
	if err != nil {
		return fmt.Errorf("info: %w", err)
	}

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	if out.IsJSON() {
		return out.JSON(app.ToInfoOutput(result))
	}

	maxWidth := 1
	for _, s := range statusOrder {
		if n := result.EntityCounts[s]; digits(n) > maxWidth {
			maxWidth = digits(n)
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Name: %s\n", result.Name)
	fmt.Fprintf(&sb, "Database: %s\n", result.DatabasePath)
	sb.WriteString("\nEntities:\n")
	for _, s := range statusOrder {
		fmt.Fprintf(&sb, "  %-9s %*d\n", strings.ToUpper(s)+":", maxWidth, result.EntityCounts[s])
	}

	out.Print(sb.String())
	return nil
}

func digits(n int) int {
	if n == 0 {
		return 1
	}
	d := 0
	for n > 0 {
		d++
		n /= 10
	}
	return d
}
