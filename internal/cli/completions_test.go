package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// makeCompletionCtx returns a context with config pointing at a freshly
// initialised (auto-migrated) SQLite database in t.TempDir().
// The returned *database.Database is for test seeding only;
// LocationCompletions opens its own connection via OpenDatabase.
func makeCompletionCtx(t *testing.T) (context.Context, *database.Database) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "completion_test.db")

	db, err := database.Open(database.Config{
		Path:        dbPath,
		BusyTimeout: database.DefaultBusyTimeout,
		AutoMigrate: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	cfg := &config.Config{
		Database: config.DatabaseConfig{Path: dbPath},
	}
	ctx := context.WithValue(context.Background(), config.ConfigKey, cfg)
	return ctx, db
}

func TestLocationCompletions_ReturnsNonSystemLocations(t *testing.T) {
	ctx, db := makeCompletionCtx(t)

	// Create two regular locations and confirm system locations already exist
	err := db.CreateLocation(ctx, "loc001", "Garage", nil, false, 0, "2026-01-01T00:00:00Z")
	require.NoError(t, err)
	err = db.CreateLocation(ctx, "loc002", "Toolbox", &[]string{"loc001"}[0], false, 0, "2026-01-01T00:00:00Z")
	require.NoError(t, err)

	completions, directive := LocationCompletions(ctx)

	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
	assert.Contains(t, completions, "garage")
	assert.Contains(t, completions, "garage:toolbox")
	// System locations must not appear
	assert.NotContains(t, completions, "missing")
	assert.NotContains(t, completions, "borrowed")
	assert.NotContains(t, completions, "loaned")
	assert.NotContains(t, completions, "removed")
}

func TestLocationCompletions_EmptyDatabase(t *testing.T) {
	ctx, _ := makeCompletionCtx(t)
	// Fresh DB has only system locations — all should be filtered out
	completions, directive := LocationCompletions(ctx)

	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
	assert.Empty(t, completions)
}

func TestLocationCompletions_ErrorOnMissingConfig(t *testing.T) {
	// Context with no config causes OpenDatabase to fail
	completions, directive := LocationCompletions(context.Background())

	assert.Equal(t, cobra.ShellCompDirectiveError, directive)
	assert.Nil(t, completions)
}

func TestLocationCompletions_ErrorOnClosedDatabase(t *testing.T) {
	// Create a valid DB file, then corrupt it so CheckDatabaseExists passes
	// but database.Open fails on the invalid SQLite header.
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "closed.db")

	// Initialize the DB file so it exists on disk
	initDB, err := database.Open(database.Config{
		Path:        dbPath,
		BusyTimeout: database.DefaultBusyTimeout,
		AutoMigrate: true,
	})
	require.NoError(t, err)
	require.NoError(t, initDB.Close())

	// Corrupt the file so SQLite cannot open it
	require.NoError(t, os.WriteFile(dbPath, []byte("not a sqlite file"), 0o600))

	cfg := &config.Config{
		Database: config.DatabaseConfig{Path: dbPath},
	}
	ctx := context.WithValue(context.Background(), config.ConfigKey, cfg)

	completions, directive := LocationCompletions(ctx)

	assert.Equal(t, cobra.ShellCompDirectiveError, directive)
	assert.Nil(t, completions)
}
