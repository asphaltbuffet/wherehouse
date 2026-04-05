package cmd

import (
	"context"
	"fmt"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/cmd/add"
	configpkg "github.com/asphaltbuffet/wherehouse/cmd/config"
	"github.com/asphaltbuffet/wherehouse/cmd/find"
	"github.com/asphaltbuffet/wherehouse/cmd/history"
	"github.com/asphaltbuffet/wherehouse/cmd/loan"
	"github.com/asphaltbuffet/wherehouse/cmd/lost"
	"github.com/asphaltbuffet/wherehouse/cmd/move"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/version"
)

// Global configuration instance accessible to all commands.
var globalConfig *config.Config

// rootCmd represents the base command when called without any subcommands.
var rootCmd *cobra.Command

// GetRootCmd returns the root command, initializing it if necessary.
func GetRootCmd() *cobra.Command {
	if rootCmd != nil {
		return rootCmd
	}

	rootCmd = &cobra.Command{
		Use:   "wherehouse",
		Short: "a personal inventory tracker",
		Long: `Wherehouse helps you track where you put that thing.

Examples:
  wherehouse --version        Show version information
  wherehouse --help           Show this help message`,
		PersistentPreRunE: initConfig,
		// RunE is nil - displays help by default when no subcommands exist
	}

	// Add persistent flags for configuration
	rootCmd.PersistentFlags().StringP("config", "c", "", "config file path (default searches global and local configs)")
	rootCmd.PersistentFlags().Bool("no-config", false, "skip all config files (use defaults only)")
	rootCmd.MarkFlagsMutuallyExclusive("config", "no-config")

	// Add other global flags (to be bound to config values)
	rootCmd.PersistentFlags().String("db", "", "database file path")
	rootCmd.PersistentFlags().String("as", "", "override acting user identity")
	rootCmd.PersistentFlags().Bool("json", false, "machine-readable JSON output")
	rootCmd.PersistentFlags().CountP("quiet", "q", "quiet mode (-q = minimal, -qq = silent)")

	rootCmd.AddCommand(configpkg.GetConfigCmd())
	rootCmd.AddCommand(add.GetAddCmd())
	rootCmd.AddCommand(find.GetFindCmd())
	rootCmd.AddCommand(history.GetHistoryCmd())
	rootCmd.AddCommand(loan.GetLoanCmd())
	rootCmd.AddCommand(lost.GetLostCmd())
	rootCmd.AddCommand(move.GetMoveCmd())

	return rootCmd
}

// initConfig initializes the configuration system.
// Called before each command runs (PersistentPreRunE).
func initConfig(cmd *cobra.Command, _ []string) error {
	configPath, _ := cmd.Flags().GetString("config")
	noConfig, _ := cmd.Flags().GetBool("no-config")

	cfg, err := loadConfigOrDefaults(configPath, noConfig)
	if err != nil {
		return err
	}

	globalConfig = cfg
	ctx := context.WithValue(cmd.Context(), config.ConfigKey, globalConfig)
	cmd.SetContext(ctx)
	return nil
}

// loadConfigOrDefaults loads configuration from file or returns defaults.
// Returns an error only if an explicit config path was provided but failed to load.
func loadConfigOrDefaults(configPath string, noConfig bool) (*config.Config, error) {
	if noConfig {
		return config.GetDefaults(), nil
	}

	cfg, err := config.New(configPath)
	if err != nil {
		if configPath != "" {
			return nil, fmt.Errorf("failed to load config from %q: %w", configPath, err)
		}
		return config.GetDefaults(), nil
	}

	return cfg, nil
}

// Execute runs the root command using fang for enhanced styling and error handling.
// This is called by main.main() and is the application entry point.
func Execute(ctx context.Context) error {
	return fang.Execute(
		ctx,
		GetRootCmd(),
		fang.WithVersion(version.ShortVersion()),
		fang.WithCommit(version.GitCommit),
	)
}
