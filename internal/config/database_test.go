package config

import (
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
				t.Helper()
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
				t.Helper()
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
				t.Helper()
				t.Setenv("XDG_DATA_HOME", "")
				t.Setenv("HOME", "")
			},
			wantSuffix: "wherehouse.db",
		},
		{
			name:     "darwin with HOME",
			platform: goosDarwin,
			envSetup: func(t *testing.T) {
				t.Helper()
				t.Setenv("HOME", "/Users/testuser")
			},
			wantContains: "/Users/testuser/Library/Application Support/wherehouse",
			wantSuffix:   "wherehouse.db",
		},
		{
			name:     "darwin without HOME falls back gracefully",
			platform: goosDarwin,
			envSetup: func(t *testing.T) {
				t.Helper()
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

			tt.envSetup(t)
			path := DefaultDatabasePath()

			assert.NotEmpty(t, path)
			assert.Equal(t, "wherehouse.db", filepath.Base(path))

			if tt.wantContains != "" {
				assert.Contains(t, path, tt.wantContains)
			}
		})
	}
}

func TestConfig_GetDatabasePath(t *testing.T) {
	tests := []struct {
		name            string
		configPath      string
		wantContains    string
		wantErrContains string
	}{
		{
			name:         "explicit config path takes precedence",
			configPath:   "/explicit/path/to/db.db",
			wantContains: "/explicit/path/to/db.db",
		},
		{
			name:         "default used when config empty",
			configPath:   "",
			wantContains: "wherehouse.db",
		},
		{
			name:         "tilde expansion in config path",
			configPath:   "~/custom/wherehouse.db",
			wantContains: "custom/wherehouse.db",
		},
		{
			name:            "invalid ~username pattern in config",
			configPath:      "~someuser/wherehouse.db",
			wantErrContains: "~username expansion not supported",
		},
		{
			name:         "env var expansion in config path",
			configPath:   "$HOME/custom/wherehouse.db",
			wantContains: "custom/wherehouse.db",
		},
		{
			name:         "relative path in config",
			configPath:   "./local/wherehouse.db",
			wantContains: "wherehouse.db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Database: DatabaseConfig{
					Path: tt.configPath,
				},
			}

			dbPath, pathErr := cfg.GetDatabasePath()

			if tt.wantErrContains != "" {
				require.ErrorContains(t, pathErr, tt.wantErrContains)
			} else {
				require.NoError(t, pathErr)
				assert.Contains(t, dbPath, tt.wantContains)
				assert.True(t, filepath.IsAbs(dbPath), "path should be absolute, got: %s", dbPath)
			}
		})
	}
}

// TestGetDatabasePath_LegacyEnvIgnored verifies the removed WHEREHOUSE_DB_PATH
// no longer influences the database path.
func TestGetDatabasePath_LegacyEnvIgnored(t *testing.T) {
	t.Setenv(LegacyDBPathEnv, "/env/path/db.db")

	t.Run("config path used", func(t *testing.T) {
		cfg := &Config{Database: DatabaseConfig{Path: "/config/path/db.db"}}

		path, err := cfg.GetDatabasePath()

		require.NoError(t, err)
		assert.Equal(t, "/config/path/db.db", path)
	})

	t.Run("default used when config empty", func(t *testing.T) {
		cfg := &Config{}

		path, err := cfg.GetDatabasePath()

		require.NoError(t, err)
		assert.Equal(t, DefaultDatabasePath(), path)
	})
}

func TestLegacyDBPathSet(t *testing.T) {
	t.Setenv(LegacyDBPathEnv, "")
	assert.False(t, LegacyDBPathSet())

	t.Setenv(LegacyDBPathEnv, "/x.db")
	assert.True(t, LegacyDBPathSet())
}

func TestConfig_GetDatabasePath_ExpandPath(t *testing.T) {
	home := "/home/fake-user"
	t.Setenv("HOME", home)
	pwd := t.TempDir()
	t.Chdir(pwd)

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
			wantContains: filepath.Join(pwd, "relative", "wherehouse.db"),
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
		})
	}
}
