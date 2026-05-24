package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// OpenDatabase opens the on-disk SQLite store from the config in ctx and
// returns both the store (so callers can Close it) and an App wired to it.
//
// Returns an error if:
//   - Configuration is not found in context
//   - Database path cannot be resolved
//   - Database connection or migration fails
func OpenDatabase(ctx context.Context) (*store.Store, *app.App, error) {
	cfg, ok := ctx.Value(config.ConfigKey).(*config.Config)
	if !ok || cfg == nil {
		return nil, nil, errors.New("configuration not found in context")
	}

	dbPath, err := cfg.GetDatabasePath()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve database path: %w", err)
	}

	s, err := store.Open(store.Config{
		Path:        dbPath,
		BusyTimeout: store.DefaultBusyTimeout,
		AutoMigrate: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("open store: %w", err)
	}

	return s, app.New(s), nil
}
