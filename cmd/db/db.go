package db

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

// NewDefaultDBCmd wires the db command group for production use.
func NewDefaultDBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Manage database settings",
	}
	cmd.AddCommand(NewDefaultDBNameCmd())
	return cmd
}

// NewDBCmd creates the `db` parent command with the `name` subcommand injected with a.
func NewDBCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Manage database settings",
	}
	cmd.AddCommand(NewDBNameCmd(a))
	return cmd
}

// NewDefaultDBNameCmd wires the db name command for production use.
func NewDefaultDBNameCmd() *cobra.Command {
	cmd := buildDBNameCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		s, a, err := cli.OpenDatabase(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer s.Close()
		return runDBName(cmd, args, a)
	}
	return cmd
}

// NewDBNameCmd creates the `db name` subcommand injected with a.
func NewDBNameCmd(a *app.App) *cobra.Command {
	cmd := buildDBNameCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runDBName(cmd, args, a)
	}
	return cmd
}

func buildDBNameCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "name [display name]",
		Short: "Set the display name of this database",
		Long: `Set a human-readable display name for this database file.

The name is stored inside the database and is independent of the filename.

Examples:
  wherehouse db name "123 Fake Street - House"
  wherehouse db name "123 Fake Street - House" --db ~/inventories/rental.db
  wherehouse db name --clear`,
		Args: cobra.MaximumNArgs(1),
	}
	cmd.Flags().Bool("clear", false, "remove the display name")
	return cmd
}

func runDBName(cmd *cobra.Command, args []string, a *app.App) error {
	ctx := cmd.Context()
	clearFlag, _ := cmd.Flags().GetBool("clear")

	if clearFlag && len(args) > 0 {
		return errors.New("--clear cannot be combined with a name argument")
	}
	if !clearFlag && len(args) == 0 {
		return errors.New("provide a display name or use --clear")
	}

	if clearFlag {
		if err := a.ClearWherehouseName(ctx); err != nil {
			return fmt.Errorf("clear name: %w", err)
		}
	} else {
		if err := a.SetWherehouseName(ctx, args[0]); err != nil {
			return fmt.Errorf("set name: %w", err)
		}
	}

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	if out.IsJSON() {
		info, err := a.GetInfo(ctx)
		if err != nil {
			return fmt.Errorf("get info for json: %w", err)
		}
		return out.JSON(app.ToInfoOutput(info))
	}

	if clearFlag {
		out.Success("Display name cleared")
	} else {
		out.Success(fmt.Sprintf("Display name set to %q", args[0]))
	}
	return nil
}
