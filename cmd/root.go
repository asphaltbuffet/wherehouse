package cmd

import (
	"context"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/version"
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "wherehouse",
	Short: "a personal inventory tracker",
	Long: `Wherehouse helps you track where you put that thing.

Currently in early development. Future versions will launch an interactive TUI
when run without arguments.

Examples:
  wherehouse --version        Show version information
  wherehouse --help           Show this help message`,
	// RunE is nil - displays help by default when no subcommands exist
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
		rootCmd,
		fang.WithVersion(version.ShortVersion()),
		fang.WithCommit(version.GitCommit),
	)
}
