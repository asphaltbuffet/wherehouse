package initialize

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

var databaseCmd *cobra.Command

// GetDatabaseCmd returns the `initialize database` subcommand.
func GetDatabaseCmd() *cobra.Command {
	if databaseCmd != nil {
		return databaseCmd
	}

	databaseCmd = &cobra.Command{
		Use:   "database",
		Short: "Create the wherehouse database",
		Long: `Create the SQLite database and apply all migrations.

Fails if the database already exists. Use --force to reinitialize.
The --force flag renames the existing database to <path>.backup.<YYYYMMDD>
before creating a fresh one.

The database path is controlled by the root --db flag or the database.path
config value. Default: $XDG_DATA_HOME/wherehouse/wherehouse.db

Examples:
  wherehouse initialize database           # Create the database
  wherehouse initialize database --force   # Reinitialize (backs up existing)`,
		RunE: runInitializeDatabase,
	}

	databaseCmd.Flags().Bool("force", false, "reinitialize: back up existing DB then create fresh")

	return databaseCmd
}

// initResult is the structured output for JSON mode.
type initResult struct {
	Status     string `json:"status"`
	Path       string `json:"path"`
	BackupPath string `json:"backup_path,omitempty"`
}

func runInitializeDatabase(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	cfg, ok := ctx.Value(config.ConfigKey).(*config.Config)
	if !ok || cfg == nil {
		return errors.New("configuration not found in context")
	}

	dbPath, err := cfg.GetDatabasePath()
	if err != nil {
		return fmt.Errorf("failed to resolve database path: %w", err)
	}

	force, _ := cmd.Flags().GetBool("force")

	backupPath, err := handleExistingDatabase(cmd, dbPath, force)
	if err != nil {
		return err
	}

	// Ensure parent directory exists (XDG data dir may not exist on fresh install).
	if mkdirErr := os.MkdirAll(filepath.Dir(dbPath), 0o700); mkdirErr != nil {
		return fmt.Errorf("could not create database directory: %w", mkdirErr)
	}

	// Open (creates file) with migrations.
	dbConfig := database.Config{
		Path:        dbPath,
		BusyTimeout: database.DefaultBusyTimeout,
		AutoMigrate: true,
	}

	db, openErr := database.Open(dbConfig)
	if openErr != nil {
		return fmt.Errorf("database initialization failed: %w", openErr)
	}

	_ = db.Close()

	// Output.
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)
	return printInitResult(out, cfg, dbPath, backupPath)
}

// handleExistingDatabase checks whether a database file already exists at dbPath
// and handles it according to the force flag:
//   - not exists: returns ("", nil) — proceed normally
//   - exists, !force: returns an error
//   - exists, force: attempts backup rename; on failure warns and removes original
//
// Returns (backupPath, nil) on success, or ("", error) on fatal failure.
func handleExistingDatabase(cmd *cobra.Command, dbPath string, force bool) (string, error) {
	_, statErr := os.Stat(dbPath)
	if errors.Is(statErr, os.ErrNotExist) {
		return "", nil
	}

	if statErr != nil {
		return "", fmt.Errorf("could not check database path: %w", statErr)
	}

	// File exists.
	if !force {
		return "", fmt.Errorf("database already exists at %q: use --force to reinitialize", dbPath)
	}

	// --force: attempt timestamped backup.
	backupPath, err := backupDatabase(dbPath)
	if err != nil {
		// Backup failed: warn and continue (best-effort).
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not back up existing database: %v\n", err)

		// Remove the existing file so Open can create a fresh one.
		if removeErr := os.Remove(dbPath); removeErr != nil {
			return "", fmt.Errorf("could not remove existing database for reinitialization: %w", removeErr)
		}

		return "", nil
	}

	return backupPath, nil
}

// backupDatabase renames dbPath to dbPath.backup.<YYYYMMDD>.
// If that target already exists, it appends a counter suffix (.1, .2, ...).
// Returns the backup path on success, or an error.
func backupDatabase(dbPath string) (string, error) {
	dateSuffix := time.Now().Format("20060102")
	candidate := dbPath + ".backup." + dateSuffix

	// Resolve collision with counter suffix.
	if _, err := os.Stat(candidate); err == nil {
		found := false

		for i := 1; i <= 99; i++ {
			candidate = fmt.Sprintf("%s.backup.%s.%d", dbPath, dateSuffix, i)
			if _, statErr := os.Stat(candidate); errors.Is(statErr, os.ErrNotExist) {
				found = true
				break
			}
		}

		if !found {
			return "", fmt.Errorf("too many backups for %s on date %s", dbPath, dateSuffix)
		}
	}

	if err := os.Rename(dbPath, candidate); err != nil {
		return "", err
	}

	return candidate, nil
}

func printInitResult(out *cli.OutputWriter, cfg *config.Config, dbPath, backupPath string) error {
	if cfg.IsJSON() {
		result := initResult{
			Status:     "initialized",
			Path:       dbPath,
			BackupPath: backupPath,
		}
		return out.JSON(result)
	}

	if cfg.IsQuiet() {
		return nil
	}

	// Human-readable.
	if backupPath != "" {
		out.Info(fmt.Sprintf("Backed up existing database to %s", backupPath))
	}

	out.Success(fmt.Sprintf("Database initialized at %s", dbPath))

	return nil
}
