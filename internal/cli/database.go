package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// ErrDatabaseNotInitialized is returned when the database file does not exist on disk.
// Callers can use [errors.Is] to detect this case programmatically.
var ErrDatabaseNotInitialized = errors.New("database not initialized")

// CheckDatabaseExists returns ErrDatabaseNotInitialized if the file at dbPath
// does not exist. Returns nil if the file is present. Returns a wrapped os error
// for any other stat failure (permissions, etc.).
func CheckDatabaseExists(dbPath string) error {
	_, err := os.Stat(dbPath)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("database not found at %q: run `wherehouse initialize database` to create it: %w",
			dbPath, ErrDatabaseNotInitialized)
	}

	return err // nil or unexpected OS error
}

// OpenDatabase opens the database connection using config settings.
// It extracts the database path from the context config and opens
// a connection with auto-migration enabled.
//
// Returns an error if:
//   - Configuration is not found in context
//   - Database path cannot be resolved
//   - Database file does not exist (use `wherehouse initialize database` to create it)
//   - Database connection fails
func OpenDatabase(ctx context.Context) (*database.Database, error) {
	// Get config from context
	cfg, ok := ctx.Value(config.ConfigKey).(*config.Config)
	if !ok || cfg == nil {
		return nil, errors.New("configuration not found in context")
	}

	// Get database path from config
	dbPath, err := cfg.GetDatabasePath()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve database path: %w", err)
	}

	// Pre-flight: fail fast with a human-readable message if the DB file is absent.
	if err = CheckDatabaseExists(dbPath); err != nil {
		return nil, err
	}

	// Open database with auto-migration enabled
	dbConfig := database.Config{
		Path:        dbPath,
		BusyTimeout: database.DefaultBusyTimeout,
		AutoMigrate: true,
	}

	return database.Open(dbConfig)
}
