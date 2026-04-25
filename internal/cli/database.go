package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// OpenDatabase opens the database connection using config settings.
// It extracts the database path from the context config and opens
// a connection with auto-migration enabled. If the database file does not
// exist it will be created automatically (AutoMigrate bootstraps a fresh file).
//
// Returns an error if:
//   - Configuration is not found in context
//   - Database path cannot be resolved
//   - Database connection or migration fails
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

	// Open database with auto-migration enabled (creates the file if absent)
	dbConfig := database.Config{
		Path:        dbPath,
		BusyTimeout: database.DefaultBusyTimeout,
		AutoMigrate: true,
	}

	return database.Open(dbConfig)
}
