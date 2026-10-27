package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/cmd/add"
	"github.com/asphaltbuffet/wherehouse/cmd/attr"
	"github.com/asphaltbuffet/wherehouse/cmd/borrow"
	configpkg "github.com/asphaltbuffet/wherehouse/cmd/config"
	dbcmd "github.com/asphaltbuffet/wherehouse/cmd/db"
	"github.com/asphaltbuffet/wherehouse/cmd/doctor"
	"github.com/asphaltbuffet/wherehouse/cmd/export"
	"github.com/asphaltbuffet/wherehouse/cmd/found"
	"github.com/asphaltbuffet/wherehouse/cmd/history"
	importcmd "github.com/asphaltbuffet/wherehouse/cmd/import"
	infocmd "github.com/asphaltbuffet/wherehouse/cmd/info"
	listcmd "github.com/asphaltbuffet/wherehouse/cmd/list"
	"github.com/asphaltbuffet/wherehouse/cmd/loan"
	"github.com/asphaltbuffet/wherehouse/cmd/lost"
	"github.com/asphaltbuffet/wherehouse/cmd/man"
	"github.com/asphaltbuffet/wherehouse/cmd/move"
	"github.com/asphaltbuffet/wherehouse/cmd/remove"
	"github.com/asphaltbuffet/wherehouse/cmd/rename"
	returncmd "github.com/asphaltbuffet/wherehouse/cmd/return"
	"github.com/asphaltbuffet/wherehouse/cmd/scry"
	"github.com/asphaltbuffet/wherehouse/cmd/serve"
	"github.com/asphaltbuffet/wherehouse/cmd/status"
	"github.com/asphaltbuffet/wherehouse/cmd/tag"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/logging"
	versioncmd "github.com/asphaltbuffet/wherehouse/internal/version"
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
		Short: "A personal inventory tracker",
		Long: `Wherehouse helps you track where you put that thing.

Examples:
  wherehouse --version        Show version information
  wherehouse --help           Show this help message`,
		Version:           versioncmd.ShortVersion(),
		SilenceUsage:      true,
		SilenceErrors:     true,
		PersistentPreRunE: initConfig,
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

	rootCmd.AddCommand(configpkg.NewConfigCmd())
	rootCmd.AddCommand(add.NewDefaultAddCmd())
	rootCmd.AddCommand(attr.NewDefaultAttrCmd())
	rootCmd.AddCommand(borrow.NewDefaultBorrowCmd())
	rootCmd.AddCommand(export.NewDefaultExportCmd())
	rootCmd.AddCommand(found.NewDefaultFoundCmd())
	rootCmd.AddCommand(history.NewDefaultHistoryCmd())
	rootCmd.AddCommand(importcmd.NewDefaultImportCmd())
	rootCmd.AddCommand(listcmd.NewDefaultListCmd())
	rootCmd.AddCommand(loan.NewDefaultLoanCmd())
	rootCmd.AddCommand(lost.NewDefaultLostCmd())
	rootCmd.AddCommand(move.NewDefaultMoveCmd())
	rootCmd.AddCommand(remove.NewDefaultRemoveCmd())
	rootCmd.AddCommand(rename.NewDefaultRenameCmd())
	rootCmd.AddCommand(returncmd.NewDefaultReturnCmd())
	rootCmd.AddCommand(scry.NewDefaultScryCmd())
	rootCmd.AddCommand(serve.NewDefaultServeCmd())
	rootCmd.AddCommand(status.NewDefaultStatusCmd())
	rootCmd.AddCommand(tag.NewDefaultTagCmd())
	rootCmd.AddCommand(doctor.NewDefaultDoctorCmd())
	rootCmd.AddCommand(dbcmd.NewDefaultDBCmd())
	rootCmd.AddCommand(infocmd.NewDefaultInfoCmd())

	// hidden internal commands — not shown in help output
	rootCmd.AddCommand(man.NewManCmd())

	return rootCmd
}

// bindFlagsToConfig applies persistent flag overrides onto cfg after loading.
// Only flags explicitly provided by the user (Changed == true) are applied,
// so flag zero-values do not silently clobber config file values.
func bindFlagsToConfig(cmd *cobra.Command, cfg *config.Config) {
	if cmd.Flags().Changed("db") {
		if val, _ := cmd.Flags().GetString("db"); val != "" {
			cfg.Database.Path = val
		}
	}
	if cmd.Flags().Changed("as") {
		if val, _ := cmd.Flags().GetString("as"); val != "" {
			cfg.User.DefaultIdentity = val
		}
	}
	if cmd.Flags().Changed("json") {
		cfg.Output.DefaultFormat = "json"
	}
	if cmd.Flags().Changed("quiet") {
		if count, err := cmd.Flags().GetCount("quiet"); err == nil {
			cfg.Output.Quiet = count
		}
	}
}

// initConfig initializes the configuration system.
// Called before each command runs (PersistentPreRunE).
func initConfig(cmd *cobra.Command, _ []string) error {
	configPath, _ := cmd.Flags().GetString("config")
	noConfig, _ := cmd.Flags().GetBool("no-config")

	cfg, err := loadConfig(configPath, noConfig)
	if err != nil {
		if !allowsInvalidConfig(cmd) {
			return err
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: invalid configuration, using defaults: %v\n", err)
		cfg = config.GetDefaults()
	}

	if config.LegacyLogPathSet() {
		fmt.Fprintf(
			cmd.ErrOrStderr(),
			"warning: %s is no longer supported; use WHEREHOUSE_LOGGING_FILE_PATH\n",
			config.LegacyLogPathEnv,
		)
	}

	if config.LegacyDBPathSet() {
		fmt.Fprintf(
			cmd.ErrOrStderr(),
			"warning: %s is no longer supported; use WHEREHOUSE_DATABASE_PATH\n",
			config.LegacyDBPathEnv,
		)
	}

	bindFlagsToConfig(cmd, cfg)

	globalConfig = cfg

	// Initialize logging. Non-fatal: a warning is written to stderr and
	// execution continues with a no-op (discard) logger.
	logPath, err := cfg.GetLogPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not resolve log path: %v\n", err)
	} else if initErr := logging.Init(afero.NewOsFs(), logPath, cfg.Logging.Level, cfg.Logging.MaxSizeMB, cfg.Logging.MaxBackups); initErr != nil {
		fmt.Fprintf(os.Stderr, "warning: logging unavailable: %v\n", initErr)
	}

	ctx := context.WithValue(cmd.Context(), config.ConfigKey, globalConfig)
	cmd.SetContext(ctx)
	return nil
}

// loadConfig loads configuration from files and environment, or returns
// defaults for --no-config. Missing default config files are not an error;
// every returned error means the configuration is unreadable or invalid.
func loadConfig(configPath string, noConfig bool) (*config.Config, error) {
	if noConfig {
		return config.GetDefaults(), nil
	}

	cfg, err := config.New(configPath)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// allowsInvalidConfig reports whether cmd or an ancestor opted in to running
// with defaults when the configuration is invalid.
func allowsInvalidConfig(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if _, ok := c.Annotations[configpkg.AllowInvalidConfigAnnotation]; ok {
			return true
		}
	}
	return false
}

// Execute runs the root command and renders any error to stderr.
// This is called by main.main() and is the application entry point.
func Execute(ctx context.Context) error {
	defer logging.Close()

	root := GetRootCmd()
	if err := root.ExecuteContext(ctx); err != nil {
		cli.NewOutputWriter(os.Stdout, os.Stderr, false, false).Error(err.Error())
		return err
	}
	return nil
}
