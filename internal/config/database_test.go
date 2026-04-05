package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultDatabasePath(t *testing.T) {
	tests := []struct {
		name         string
		platform     string
		envSetup     func(*testing.T)
		wantContains string // Path should contain this substring
		wantSuffix   string // Path should end with this
	}{
		{
			name:     "linux with XDG_DATA_HOME",
			platform: goosLinux,
			envSetup: func(t *testing.T) {
				t.Setenv("XDG_DATA_HOME", "/home/testuser/.local/share")
				t.Setenv("HOME", "/home/testuser")
			},
			wantContains: "/home/testuser/.local/share/wherehouse",
			wantSuffix:   "wherehouse.db",
		},
		{
			name:     "linux without XDG_DATA_HOME",
			platform: goosLinux,
			envSetup: func(t *testing.T) {
				t.Setenv("XDG_DATA_HOME", "")
				t.Setenv("HOME", "/home/testuser")
			},
			wantContains: "/home/testuser/.local/share/wherehouse",
			wantSuffix:   "wherehouse.db",
		},
		{
			name:     "linux without HOME falls back gracefully",
			platform: goosLinux,
			envSetup: func(t *testing.T) {
				t.Setenv("XDG_DATA_HOME", "")
				t.Setenv("HOME", "")
			},
			wantSuffix: "wherehouse.db",
		},
		{
			name:     "darwin with HOME",
			platform: goosDarwin,
			envSetup: func(t *testing.T) {
				t.Setenv("HOME", "/Users/testuser")
			},
			wantContains: "/Users/testuser/Library/Application Support/wherehouse",
			wantSuffix:   "wherehouse.db",
		},
		{
			name:     "darwin without HOME falls back gracefully",
			platform: goosDarwin,
			envSetup: func(t *testing.T) {
				t.Setenv("HOME", "")
			},
			wantSuffix: "wherehouse.db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip tests for platforms that don't match current GOOS
			if tt.platform != runtime.GOOS {
				t.Skipf("skipping %s test on %s platform", tt.platform, runtime.GOOS)
			}

			// Setup environment for this test
			tt.envSetup(t)

			// Get path
			path := DefaultDatabasePath()

			// Verify path is not empty
			assert.NotEmpty(t, path, "path should not be empty")

			// Verify suffix
			assert.Equal(t, "wherehouse.db", filepath.Base(path),
				"path should end with wherehouse.db, got: %s", path)

			// Verify contains expected substring (if specified)
			if tt.wantContains != "" {
				assert.Contains(t, path, tt.wantContains,
					"path should contain %q, got: %s", tt.wantContains, path)
			}

			// Verify path is absolute or relative (for fallback cases)
			if tt.wantContains != "" {
				assert.True(t, filepath.IsAbs(path) || path == "./wherehouse.db",
					"path should be absolute or fallback, got: %s", path)
			}
		})
	}
}

func TestDefaultDatabasePath_CurrentPlatform(t *testing.T) {
	// Test current platform without mocking environment
	path := DefaultDatabasePath()

	// Verify path is not empty
	require.NotEmpty(t, path, "path should not be empty")

	// Verify filename
	assert.Equal(t, "wherehouse.db", filepath.Base(path),
		"database filename should be wherehouse.db")

	// Verify path contains wherehouse directory
	assert.Contains(t, path, "wherehouse",
		"path should contain wherehouse directory")

	// Platform-specific checks
	switch runtime.GOOS {
	case goosLinux, goosFreeBSD, goosOpenBSD, goosNetBSD:
		// Should use XDG or .local/share
		if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
			assert.Contains(t, path, xdgDataHome)
		} else {
			assert.Contains(t, path, ".local/share")
		}

	case goosDarwin:
		assert.Contains(t, path, "Library/Application Support")

	case goosWindows:
		// Should contain APPDATA or fallback
		assert.True(t,
			os.Getenv("APPDATA") != "" || os.Getenv("USERPROFILE") != "",
			"Windows should have APPDATA or USERPROFILE set")
	}
}

func TestConfig_GetDatabasePath(t *testing.T) {
	tests := []struct {
		name            string
		configPath      string
		envPath         string
		wantContains    string
		wantErr         bool
		wantErrContains string
	}{
		{
			name:         "explicit config path takes precedence",
			configPath:   "/explicit/path/to/db.db",
			envPath:      "/env/path/to/db.db",
			wantContains: "/explicit/path/to/db.db",
			wantErr:      false,
		},
		{
			name:         "env var used when config empty",
			configPath:   "",
			envPath:      "/env/path/to/db.db",
			wantContains: "/env/path/to/db.db",
			wantErr:      false,
		},
		{
			name:         "default used when both empty",
			configPath:   "",
			envPath:      "",
			wantContains: "wherehouse.db",
			wantErr:      false,
		},
		{
			name:         "tilde expansion in config path",
			configPath:   "~/custom/wherehouse.db",
			envPath:      "",
			wantContains: "custom/wherehouse.db",
			wantErr:      false,
		},
		{
			name:         "tilde expansion in env path",
			configPath:   "",
			envPath:      "~/env/wherehouse.db",
			wantContains: "env/wherehouse.db",
			wantErr:      false,
		},
		{
			name:            "invalid ~username pattern in config",
			configPath:      "~someuser/wherehouse.db",
			envPath:         "",
			wantErr:         true,
			wantErrContains: "~username expansion not supported",
		},
		{
			name:            "invalid ~username pattern in env",
			configPath:      "",
			envPath:         "~someuser/wherehouse.db",
			wantErr:         true,
			wantErrContains: "~username expansion not supported",
		},
		{
			name:         "env var expansion in config path",
			configPath:   "$HOME/custom/wherehouse.db",
			envPath:      "",
			wantContains: "custom/wherehouse.db",
			wantErr:      false,
		},
		{
			name:         "relative path in config",
			configPath:   "./local/wherehouse.db",
			envPath:      "",
			wantContains: "wherehouse.db",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			if tt.envPath != "" {
				t.Setenv("WHEREHOUSE_DB_PATH", tt.envPath)
			}

			// Create config
			cfg := &Config{
				Database: DatabaseConfig{
					Path: tt.configPath,
				},
			}

			// Call GetDatabasePath
			dbPath, pathErr := cfg.GetDatabasePath()

			if tt.wantErr {
				require.ErrorContains(t, pathErr, tt.wantErrContains)
			} else {
				require.NoError(t, pathErr)
				assert.Contains(t, dbPath, tt.wantContains)
				assert.True(t, filepath.IsAbs(dbPath), "path should be absolute, got: %s", dbPath)
			}
		})
	}
}

func TestConfig_GetDatabasePath_Precedence(t *testing.T) {
	// Test 1: Config path overrides env
	t.Run("config overrides env", func(t *testing.T) {
		t.Setenv("WHEREHOUSE_DB_PATH", "/env/path/db.db")

		cfg := &Config{
			Database: DatabaseConfig{
				Path: "/config/path/db.db",
			},
		}
		path, err := cfg.GetDatabasePath()
		require.NoError(t, err)
		assert.Contains(t, path, "/config/path/db.db")
	})

	// Test 2: Env used when config empty
	t.Run("env used when config empty", func(t *testing.T) {
		t.Setenv("WHEREHOUSE_DB_PATH", "/env/path/db.db")

		cfg := &Config{
			Database: DatabaseConfig{
				Path: "",
			},
		}
		path, err := cfg.GetDatabasePath()
		require.NoError(t, err)
		assert.Contains(t, path, "/env/path/db.db")
	})

	// Test 3: Default used when both empty
	t.Run("default used when both empty", func(t *testing.T) {
		cfg := &Config{
			Database: DatabaseConfig{
				Path: "",
			},
		}
		path, err := cfg.GetDatabasePath()
		require.NoError(t, err)
		assert.Contains(t, path, "wherehouse.db")
		assert.Contains(t, path, "wherehouse") // Should contain directory
	})
}

func TestConfig_GetDatabasePath_ExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	tests := []struct {
		name         string
		inputPath    string
		wantContains string
	}{
		{
			name:         "tilde expansion",
			inputPath:    "~/mydata/wherehouse.db",
			wantContains: filepath.Join(home, "mydata", "wherehouse.db"),
		},
		{
			name:         "tilde alone",
			inputPath:    "~/wherehouse.db",
			wantContains: filepath.Join(home, "wherehouse.db"),
		},
		{
			name:         "env var expansion",
			inputPath:    "$HOME/mydata/wherehouse.db",
			wantContains: filepath.Join(home, "mydata", "wherehouse.db"),
		},
		{
			name:         "absolute path unchanged",
			inputPath:    "/absolute/path/wherehouse.db",
			wantContains: "/absolute/path/wherehouse.db",
		},
		{
			name:         "relative path made absolute",
			inputPath:    "relative/wherehouse.db",
			wantContains: "wherehouse.db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Database: DatabaseConfig{
					Path: tt.inputPath,
				},
			}

			path, pathErr := cfg.GetDatabasePath()
			require.NoError(t, pathErr)
			assert.Contains(t, path, tt.wantContains)
			assert.True(t, filepath.IsAbs(path))
		})
	}
}
