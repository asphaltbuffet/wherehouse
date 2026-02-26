package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

func TestOpenDatabase(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) context.Context
		wantErr bool
		errMsg  string
	}{
		{
			name: "success with valid config",
			setup: func(t *testing.T) context.Context {
				t.Helper()
				// Create temp dir and pre-initialize the database file so OpenDatabase
				// can find it (OpenDatabase now requires the file to exist).
				tmpDir := t.TempDir()
				dbPath := filepath.Join(tmpDir, "test.db")

				// Pre-create the database file via database.Open so it is a valid SQLite file.
				initDB, err := database.Open(database.Config{
					Path:        dbPath,
					BusyTimeout: database.DefaultBusyTimeout,
					AutoMigrate: true,
				})
				require.NoError(t, err)
				require.NoError(t, initDB.Close())

				cfg := &config.Config{
					Database: config.DatabaseConfig{
						Path: dbPath,
					},
				}
				return context.WithValue(context.Background(), config.ConfigKey, cfg)
			},
			wantErr: false,
		},
		{
			name: "error when config not in context",
			setup: func(t *testing.T) context.Context {
				t.Helper()
				return context.Background()
			},
			wantErr: true,
			errMsg:  "configuration not found in context",
		},
		{
			name: "error when config is nil",
			setup: func(t *testing.T) context.Context {
				t.Helper()
				var nilConfig *config.Config
				return context.WithValue(context.Background(), config.ConfigKey, nilConfig)
			},
			wantErr: true,
			errMsg:  "configuration not found in context",
		},
		{
			name: "error when database file does not exist",
			setup: func(t *testing.T) context.Context {
				t.Helper()
				// Provide a path to a file that was never created.
				tmpDir := t.TempDir()
				dbPath := filepath.Join(tmpDir, "nonexistent.db")

				cfg := &config.Config{
					Database: config.DatabaseConfig{
						Path: dbPath,
					},
				}
				return context.WithValue(context.Background(), config.ConfigKey, cfg)
			},
			wantErr: true,
			errMsg:  "database not found at",
		},
		{
			name: "success with empty path uses default",
			setup: func(t *testing.T) context.Context {
				t.Helper()
				// Empty path uses system default. Pre-create the file via env var override.
				tmpDir := t.TempDir()
				dbPath := filepath.Join(tmpDir, "default.db")

				// Set environment variable for this test
				t.Setenv("WHEREHOUSE_DB_PATH", dbPath)

				// Pre-create the database file.
				initDB, err := database.Open(database.Config{
					Path:        dbPath,
					BusyTimeout: database.DefaultBusyTimeout,
					AutoMigrate: true,
				})
				require.NoError(t, err)
				require.NoError(t, initDB.Close())

				cfg := &config.Config{
					Database: config.DatabaseConfig{
						Path: "", // Empty path will fall back to env var
					},
				}
				return context.WithValue(context.Background(), config.ConfigKey, cfg)
			},
			wantErr: false,
		},
		{
			name: "success with nested directory creation",
			setup: func(t *testing.T) context.Context {
				t.Helper()
				// Create temp dir and ensure parent directory exists, then pre-create the DB file.
				tmpDir := t.TempDir()
				nestedDir := filepath.Join(tmpDir, "subdir1", "subdir2")
				require.NoError(t, os.MkdirAll(nestedDir, 0o755))
				dbPath := filepath.Join(nestedDir, "test.db")

				// Pre-create the database file.
				initDB, err := database.Open(database.Config{
					Path:        dbPath,
					BusyTimeout: database.DefaultBusyTimeout,
					AutoMigrate: true,
				})
				require.NoError(t, err)
				require.NoError(t, initDB.Close())

				cfg := &config.Config{
					Database: config.DatabaseConfig{
						Path: dbPath,
					},
				}
				return context.WithValue(context.Background(), config.ConfigKey, cfg)
			},
			wantErr: false,
		},
		{
			name: "error when path points to directory",
			setup: func(t *testing.T) context.Context {
				t.Helper()
				// Create a directory; os.Stat on a directory succeeds (file exists check
				// passes), but database.Open will return its own error.
				tmpDir := t.TempDir()

				cfg := &config.Config{
					Database: config.DatabaseConfig{
						Path: tmpDir, // Points to directory, not file
					},
				}
				return context.WithValue(context.Background(), config.ConfigKey, cfg)
			},
			wantErr: true,
			errMsg:  "", // Database.Open will return its own error
		},
		{
			name: "error when path has invalid permissions",
			setup: func(t *testing.T) context.Context {
				t.Helper()
				if os.Getuid() == 0 {
					t.Skip("Skipping permissions test when running as root")
				}

				// Create temp dir with restricted permissions
				tmpDir := t.TempDir()
				restrictedDir := filepath.Join(tmpDir, "restricted")
				err := os.Mkdir(restrictedDir, 0o444) // Read-only directory
				require.NoError(t, err)

				dbPath := filepath.Join(restrictedDir, "test.db")

				cfg := &config.Config{
					Database: config.DatabaseConfig{
						Path: dbPath,
					},
				}
				return context.WithValue(context.Background(), config.ConfigKey, cfg)
			},
			wantErr: true,
			errMsg:  "", // Permission error or "database not found" depending on os.Stat behavior
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setup(t)

			db, err := OpenDatabase(ctx)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.ErrorContains(t, err, tt.errMsg)
				}
				assert.Nil(t, db)
			} else {
				require.NoError(t, err)
				require.NotNil(t, db)
				// Clean up
				assert.NoError(t, db.Close())
			}
		})
	}
}

func TestOpenDatabase_AutoMigration(t *testing.T) {
	// OpenDatabase requires the database file to already exist (use
	// `wherehouse initialize database` to create it). Pre-create the file here.
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Pre-initialize the database.
	initDB, err := database.Open(database.Config{
		Path:        dbPath,
		BusyTimeout: database.DefaultBusyTimeout,
		AutoMigrate: true,
	})
	require.NoError(t, err)
	require.NoError(t, initDB.Close())

	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Path: dbPath,
		},
	}
	ctx := context.WithValue(context.Background(), config.ConfigKey, cfg)

	// Open database (applies auto-migration on an existing file).
	db, err := OpenDatabase(ctx)
	require.NoError(t, err)
	require.NotNil(t, db)
	defer db.Close()

	// Verify database file still exists.
	_, err = os.Stat(dbPath)
	assert.NoError(t, err, "database file should still exist after open")
}

func TestOpenDatabase_ContextPropagation(t *testing.T) {
	// Verify that context is properly used throughout the call chain.
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Pre-initialize the database so CheckDatabaseExists passes.
	initDB, err := database.Open(database.Config{
		Path:        dbPath,
		BusyTimeout: database.DefaultBusyTimeout,
		AutoMigrate: true,
	})
	require.NoError(t, err)
	require.NoError(t, initDB.Close())

	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Path: dbPath,
		},
	}

	// Use a custom context value to verify propagation.
	type testKey string
	const customKey testKey = "test"
	ctx := context.WithValue(context.Background(), customKey, "value")
	ctx = context.WithValue(ctx, config.ConfigKey, cfg)

	db, err := OpenDatabase(ctx)
	require.NoError(t, err)
	require.NotNil(t, db)
	defer db.Close()

	// Verify original context still has custom value.
	assert.Equal(t, "value", ctx.Value(customKey))
}

func TestOpenDatabase_MultipleCallsSeparate(t *testing.T) {
	// Test that multiple calls to OpenDatabase create separate connections.
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Pre-initialize the database so CheckDatabaseExists passes.
	initDB, err := database.Open(database.Config{
		Path:        dbPath,
		BusyTimeout: database.DefaultBusyTimeout,
		AutoMigrate: true,
	})
	require.NoError(t, err)
	require.NoError(t, initDB.Close())

	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Path: dbPath,
		},
	}
	ctx := context.WithValue(context.Background(), config.ConfigKey, cfg)

	// Open first connection.
	db1, err := OpenDatabase(ctx)
	require.NoError(t, err)
	require.NotNil(t, db1)
	defer db1.Close()

	// Open second connection.
	db2, err := OpenDatabase(ctx)
	require.NoError(t, err)
	require.NotNil(t, db2)
	defer db2.Close()

	// Verify both are different instances (pointer comparison).
	assert.NotSame(t, db1, db2)
}

func TestOpenDatabase_ExistingDatabase(t *testing.T) {
	// Test opening an existing database file.
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Path: dbPath,
		},
	}
	ctx := context.WithValue(context.Background(), config.ConfigKey, cfg)

	// Initialize database directly (bypassing OpenDatabase, which requires the file to exist).
	db1, err := database.Open(database.Config{
		Path:        dbPath,
		BusyTimeout: database.DefaultBusyTimeout,
		AutoMigrate: true,
	})
	require.NoError(t, err)
	require.NotNil(t, db1)
	db1.Close()

	// Open existing database via OpenDatabase.
	db2, err := OpenDatabase(ctx)
	require.NoError(t, err)
	require.NotNil(t, db2)
	defer db2.Close()

	// Verify database file exists and is a file (not directory).
	info, err := os.Stat(dbPath)
	require.NoError(t, err)
	assert.False(t, info.IsDir())
}

func TestCheckDatabaseExists(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(t *testing.T) string
		wantErr      bool
		wantSentinel bool
		errContains  string
	}{
		{
			name: "file present returns nil",
			setup: func(t *testing.T) string {
				t.Helper()
				tmpDir := t.TempDir()
				dbPath := filepath.Join(tmpDir, "test.db")
				// Create a non-empty file so os.Stat sees a regular file.
				f, err := os.Create(dbPath)
				require.NoError(t, err)
				require.NoError(t, f.Close())
				return dbPath
			},
			wantErr:      false,
			wantSentinel: false,
		},
		{
			name: "file absent dir present returns ErrDatabaseNotInitialized",
			setup: func(t *testing.T) string {
				t.Helper()
				// Create the directory but not the file.
				tmpDir := t.TempDir()
				return filepath.Join(tmpDir, "missing.db")
			},
			wantErr:      true,
			wantSentinel: true,
			errContains:  "database not found at",
		},
		{
			name: "dir absent returns ErrDatabaseNotInitialized",
			setup: func(t *testing.T) string {
				t.Helper()
				// Point to a path whose parent directory does not exist.
				tmpDir := t.TempDir()
				return filepath.Join(tmpDir, "nonexistent-subdir", "missing.db")
			},
			wantErr:      true,
			wantSentinel: true,
			errContains:  "database not found at",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbPath := tt.setup(t)

			err := CheckDatabaseExists(dbPath)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					require.ErrorContains(t, err, tt.errContains)
				}
				if tt.wantSentinel {
					require.ErrorIs(t, err, ErrDatabaseNotInitialized)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
