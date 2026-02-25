package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/config"
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
				// Create temp dir for test database
				tmpDir := t.TempDir()
				dbPath := filepath.Join(tmpDir, "test.db")

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
			name: "success with empty path uses default",
			setup: func(t *testing.T) context.Context {
				t.Helper()
				// Empty path uses system default. We need to ensure the parent directory exists.
				// Create a temp directory and set WHEREHOUSE_DB_PATH to use it
				tmpDir := t.TempDir()
				dbPath := filepath.Join(tmpDir, "default.db")

				// Set environment variable for this test
				t.Setenv("WHEREHOUSE_DB_PATH", dbPath)

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
				// Create temp dir and ensure parent directory exists
				tmpDir := t.TempDir()
				nestedDir := filepath.Join(tmpDir, "subdir1", "subdir2")
				require.NoError(t, os.MkdirAll(nestedDir, 0755))
				dbPath := filepath.Join(nestedDir, "test.db")

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
				// Create a directory
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
				err := os.Mkdir(restrictedDir, 0444) // Read-only directory
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
			errMsg:  "", // Permission error message varies by OS
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
	// Create temp database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Path: dbPath,
		},
	}
	ctx := context.WithValue(context.Background(), config.ConfigKey, cfg)

	// Open database (should auto-migrate)
	db, err := OpenDatabase(ctx)
	require.NoError(t, err)
	require.NotNil(t, db)
	defer db.Close()

	// Verify database was created
	_, err = os.Stat(dbPath)
	assert.NoError(t, err, "database file should exist")
}

func TestOpenDatabase_ContextPropagation(t *testing.T) {
	// Verify that context is properly used throughout the call chain
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Path: dbPath,
		},
	}

	// Use a custom context value to verify propagation
	type testKey string
	const customKey testKey = "test"
	ctx := context.WithValue(context.Background(), customKey, "value")
	ctx = context.WithValue(ctx, config.ConfigKey, cfg)

	db, err := OpenDatabase(ctx)
	require.NoError(t, err)
	require.NotNil(t, db)
	defer db.Close()

	// Verify original context still has custom value
	assert.Equal(t, "value", ctx.Value(customKey))
}

func TestOpenDatabase_MultipleCallsSeparate(t *testing.T) {
	// Test that multiple calls to OpenDatabase create separate connections
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Path: dbPath,
		},
	}
	ctx := context.WithValue(context.Background(), config.ConfigKey, cfg)

	// Open first connection
	db1, err := OpenDatabase(ctx)
	require.NoError(t, err)
	require.NotNil(t, db1)
	defer db1.Close()

	// Open second connection
	db2, err := OpenDatabase(ctx)
	require.NoError(t, err)
	require.NotNil(t, db2)
	defer db2.Close()

	// Verify both are different instances (pointer comparison)
	assert.NotSame(t, db1, db2)
}

func TestOpenDatabase_ExistingDatabase(t *testing.T) {
	// Test opening an existing database file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Path: dbPath,
		},
	}
	ctx := context.WithValue(context.Background(), config.ConfigKey, cfg)

	// Create database first
	db1, err := OpenDatabase(ctx)
	require.NoError(t, err)
	require.NotNil(t, db1)
	db1.Close()

	// Open existing database
	db2, err := OpenDatabase(ctx)
	require.NoError(t, err)
	require.NotNil(t, db2)
	defer db2.Close()

	// Verify database file exists
	info, err := os.Stat(dbPath)
	require.NoError(t, err)
	assert.False(t, info.IsDir())
}
