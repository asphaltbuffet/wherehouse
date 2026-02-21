package config

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNew_WithDefaultPaths tests loading config from default locations.
func TestNew_WithDefaultPaths(t *testing.T) {
	tests := []struct {
		name           string
		setupFS        func(afero.Fs)
		expectedDBPath string
		expectedUser   string
		expectedFormat string
		expectError    bool
	}{
		{
			name: "no config files - use defaults",
			setupFS: func(_ afero.Fs) {
				// No config files
			},
			expectedDBPath: "/home/user/.wherehouse/inventory.db", // Uses HOME from t.Setenv
			expectedUser:   "",
			expectedFormat: "human",
			expectError:    false,
		},
		{
			name: "global config only",
			setupFS: func(fs afero.Fs) {
				globalPath := "/home/user/.config/wherehouse/wherehouse.toml"
				require.NoError(t, fs.MkdirAll(filepath.Dir(globalPath), 0755))
				content := `[database]
path = "/global/db.sqlite"

[user]
default_identity = "alice"

[output]
default_format = "json"
`
				require.NoError(t, afero.WriteFile(fs, globalPath, []byte(content), 0644))
			},
			expectedDBPath: "/global/db.sqlite",
			expectedUser:   "alice",
			expectedFormat: "json",
			expectError:    false,
		},
		{
			name: "local config only",
			setupFS: func(fs afero.Fs) {
				content := `[database]
path = "/local/db.sqlite"

[user]
default_identity = "bob"
`
				require.NoError(t, afero.WriteFile(fs, "./wherehouse.toml", []byte(content), 0644))
			},
			expectedDBPath: "/local/db.sqlite",
			expectedUser:   "bob",
			expectedFormat: "human", // default
			expectError:    false,
		},
		{
			name: "global and local config - local overrides",
			setupFS: func(fs afero.Fs) {
				// Global config
				globalPath := "/home/user/.config/wherehouse/wherehouse.toml"
				require.NoError(t, fs.MkdirAll(filepath.Dir(globalPath), 0755))
				globalContent := `[database]
path = "/global/db.sqlite"

[user]
default_identity = "alice"

[output]
default_format = "json"
quiet = false
`
				require.NoError(t, afero.WriteFile(fs, globalPath, []byte(globalContent), 0644))

				// Local config (overrides some values)
				localContent := `[database]
path = "/local/db.sqlite"

[output]
quiet = true
`
				require.NoError(t, afero.WriteFile(fs, "./wherehouse.toml", []byte(localContent), 0644))
			},
			expectedDBPath: "/local/db.sqlite", // from local
			expectedUser:   "alice",            // from global
			expectedFormat: "json",             // from global
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create in-memory filesystem
			fs := afero.NewMemMapFs()

			// Set up HOME for global config path resolution
			t.Setenv("HOME", "/home/user")
			t.Setenv("XDG_CONFIG_HOME", "")

			// Setup filesystem state
			tt.setupFS(fs)

			// Load config
			cfg, err := NewWithFS(fs, "")

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedDBPath, cfg.Database.Path)
			assert.Equal(t, tt.expectedUser, cfg.User.DefaultIdentity)
			assert.Equal(t, tt.expectedFormat, cfg.Output.DefaultFormat)
		})
	}
}

// TestNew_WithExplicitPath tests loading config from explicit filepath.
func TestNew_WithExplicitPath(t *testing.T) {
	tests := []struct {
		name        string
		filepath    string
		setupFS     func(afero.Fs)
		expectError bool
		errorMsg    string
	}{
		{
			name:     "explicit file exists",
			filepath: "/custom/config.toml",
			setupFS: func(fs afero.Fs) {
				require.NoError(t, fs.MkdirAll("/custom", 0755))
				content := `[database]
path = "/custom/db.sqlite"
`
				require.NoError(t, afero.WriteFile(fs, "/custom/config.toml", []byte(content), 0644))
			},
			expectError: false,
		},
		{
			name:     "explicit file not found",
			filepath: "/missing/config.toml",
			setupFS: func(_ afero.Fs) {
				// File doesn't exist
			},
			expectError: true,
			errorMsg:    "failed to read config file",
		},
		{
			name:     "explicit file with invalid TOML",
			filepath: "/bad/config.toml",
			setupFS: func(fs afero.Fs) {
				require.NoError(t, fs.MkdirAll("/bad", 0755))
				content := `[database
path = broken`
				require.NoError(t, afero.WriteFile(fs, "/bad/config.toml", []byte(content), 0644))
			},
			expectError: true,
			errorMsg:    "failed to read config file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			tt.setupFS(fs)

			cfg, err := NewWithFS(fs, tt.filepath)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, cfg)
		})
	}
}

// TestValidation tests configuration validation.
func TestValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: &Config{
				Database: DatabaseConfig{Path: "/path/to/db.sqlite"},
				User:     UserConfig{DefaultIdentity: "alice"},
				Output:   OutputConfig{DefaultFormat: "human"},
			},
			expectError: false,
		},
		{
			name: "empty database path",
			config: &Config{
				Database: DatabaseConfig{Path: ""},
				Output:   OutputConfig{DefaultFormat: "human"},
			},
			expectError: true,
			errorMsg:    "path is required",
		},
		{
			name: "invalid output format",
			config: &Config{
				Database: DatabaseConfig{Path: "/path/to/db.sqlite"},
				Output:   OutputConfig{DefaultFormat: "xml"},
			},
			expectError: true,
			errorMsg:    "must be one of [human, json]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				return
			}

			assert.NoError(t, err)
		})
	}
}

// TestDefaults tests default value application.
func TestDefaults(t *testing.T) {
	cfg := &Config{}
	applyDefaults(cfg)

	assert.NotEmpty(t, cfg.Database.Path)
	assert.Equal(t, "human", cfg.Output.DefaultFormat)
	assert.False(t, cfg.Output.Quiet)
	assert.NotNil(t, cfg.User.OSUsernameMap)
	assert.Empty(t, cfg.User.OSUsernameMap)
}

// TestExpandPath tests path expansion functionality.
func TestExpandPath(t *testing.T) {
	t.Setenv("HOME", "/home/testuser")
	t.Setenv("TESTVAR", "myvalue")

	tests := []struct {
		name        string
		input       string
		expectError bool
		checkResult func(t *testing.T, result string)
	}{
		{
			name:        "empty path",
			input:       "",
			expectError: false,
			checkResult: func(t *testing.T, result string) {
				assert.Empty(t, result)
			},
		},
		{
			name:        "absolute path unchanged",
			input:       "/absolute/path/to/db.sqlite",
			expectError: false,
			checkResult: func(t *testing.T, result string) {
				assert.Equal(t, "/absolute/path/to/db.sqlite", result)
			},
		},
		{
			name:        "tilde expansion",
			input:       "~/mydb.sqlite",
			expectError: false,
			checkResult: func(t *testing.T, result string) {
				assert.Equal(t, "/home/testuser/mydb.sqlite", result)
			},
		},
		{
			name:        "tilde only",
			input:       "~",
			expectError: false,
			checkResult: func(t *testing.T, result string) {
				assert.Equal(t, "/home/testuser", result)
			},
		},
		{
			name:        "tilde username rejected",
			input:       "~bob/mydb.sqlite",
			expectError: true,
			checkResult: func(_ *testing.T, _ string) {
				// Error should be returned
			},
		},
		{
			name:        "environment variable expansion",
			input:       "/path/$TESTVAR/db.sqlite",
			expectError: false,
			checkResult: func(t *testing.T, result string) {
				assert.Equal(t, "/path/myvalue/db.sqlite", result)
			},
		},
		{
			name:        "relative path made absolute",
			input:       "relative/path/db.sqlite",
			expectError: false,
			checkResult: func(t *testing.T, result string) {
				assert.True(t, filepath.IsAbs(result))
				assert.Contains(t, result, "relative/path/db.sqlite")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExpandPath(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			tt.checkResult(t, result)
		})
	}
}

// TestGetCurrentUsername tests OS username retrieval.
func TestGetCurrentUsername(t *testing.T) {
	tests := []struct {
		name     string
		setupEnv func(*testing.T)
		expected string
	}{
		{
			name: "USER environment variable set",
			setupEnv: func(t *testing.T) {
				t.Setenv("USER", "alice")
				t.Setenv("USERNAME", "")
			},
			expected: "alice",
		},
		{
			name: "USERNAME environment variable set",
			setupEnv: func(t *testing.T) {
				t.Setenv("USER", "")
				t.Setenv("USERNAME", "bob")
			},
			expected: "bob",
		},
		{
			name: "no environment variables set",
			setupEnv: func(t *testing.T) {
				t.Setenv("USER", "")
				t.Setenv("USERNAME", "")
			},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv(t)
			result := GetCurrentUsername()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestEnvironmentVariableOverride tests environment variable precedence.
func TestEnvironmentVariableOverride(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Set up HOME for global config path resolution
	t.Setenv("HOME", "/home/user")
	t.Setenv("XDG_CONFIG_HOME", "")

	// Create global config
	globalPath := "/home/user/.config/wherehouse/wherehouse.toml"
	require.NoError(t, fs.MkdirAll(filepath.Dir(globalPath), 0755))
	content := `[database]
path = "/from/config.sqlite"

[user]
default_identity = "alice"
`
	require.NoError(t, afero.WriteFile(fs, globalPath, []byte(content), 0644))

	// Set environment variables to override config
	t.Setenv("WHEREHOUSE_DATABASE_PATH", "/from/env.sqlite")
	t.Setenv("WHEREHOUSE_USER_DEFAULT_IDENTITY", "bob")

	cfg, err := NewWithFS(fs, "")
	require.NoError(t, err)

	// Environment variables should override config file
	assert.Equal(t, "/from/env.sqlite", cfg.Database.Path)
	assert.Equal(t, "bob", cfg.User.DefaultIdentity)
}

// TestOSUsernameMap tests the OS username mapping feature.
func TestOSUsernameMap(t *testing.T) {
	fs := afero.NewMemMapFs()

	content := `[user]
default_identity = ""

[user.os_username_map]
"jdoe" = "john.doe"
"asmith" = "alice.smith"
`
	require.NoError(t, afero.WriteFile(fs, "./wherehouse.toml", []byte(content), 0644))

	t.Setenv("HOME", "/home/user")

	cfg, err := NewWithFS(fs, "")
	require.NoError(t, err)

	assert.NotNil(t, cfg.User.OSUsernameMap)
	assert.Equal(t, "john.doe", cfg.User.OSUsernameMap["jdoe"])
	assert.Equal(t, "alice.smith", cfg.User.OSUsernameMap["asmith"])
}

// TestQuietFlag tests the quiet output configuration.
func TestQuietFlag(t *testing.T) {
	fs := afero.NewMemMapFs()

	content := `[output]
quiet = true
`
	require.NoError(t, afero.WriteFile(fs, "./wherehouse.toml", []byte(content), 0644))

	t.Setenv("HOME", "/home/user")

	cfg, err := NewWithFS(fs, "")
	require.NoError(t, err)

	assert.True(t, cfg.Output.Quiet)
}

// TestGetDefaults tests the GetDefaults helper function.
func TestGetDefaults(t *testing.T) {
	cfg := GetDefaults()

	assert.NotNil(t, cfg)
	assert.NotEmpty(t, cfg.Database.Path)
	assert.Equal(t, "human", cfg.Output.DefaultFormat)
	assert.False(t, cfg.Output.Quiet)
	assert.NotNil(t, cfg.User.OSUsernameMap)
}

// TestWHEREHOUSE_CONFIG tests the WHEREHOUSE_CONFIG environment variable.
func TestWHEREHOUSE_CONFIG(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Set up HOME for global config path resolution
	t.Setenv("HOME", "/home/user")
	t.Setenv("XDG_CONFIG_HOME", "")

	// Create a custom config file
	customPath := "/custom/my-config.toml"
	require.NoError(t, fs.MkdirAll(filepath.Dir(customPath), 0755))
	content := `[database]
path = "/custom/db.sqlite"

[user]
default_identity = "custom-user"
`
	require.NoError(t, afero.WriteFile(fs, customPath, []byte(content), 0644))

	// Also create global and local configs (should be ignored)
	globalPath := "/home/user/.config/wherehouse/wherehouse.toml"
	require.NoError(t, fs.MkdirAll(filepath.Dir(globalPath), 0755))
	globalContent := `[database]
path = "/global/db.sqlite"
`
	require.NoError(t, afero.WriteFile(fs, globalPath, []byte(globalContent), 0644))

	localContent := `[database]
path = "/local/db.sqlite"
`
	require.NoError(t, afero.WriteFile(fs, "./wherehouse.toml", []byte(localContent), 0644))

	// Set WHEREHOUSE_CONFIG environment variable
	t.Setenv("WHEREHOUSE_CONFIG", customPath)

	// Load config with empty filepath (should use WHEREHOUSE_CONFIG)
	cfg, err := NewWithFS(fs, "")
	require.NoError(t, err)

	// Should load from custom path, not global or local
	assert.Equal(t, "/custom/db.sqlite", cfg.Database.Path)
	assert.Equal(t, "custom-user", cfg.User.DefaultIdentity)
}

// TestConfigPrecedence tests the full precedence order.
func TestConfigPrecedence(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Set up HOME for global config path resolution
	t.Setenv("HOME", "/home/user")
	t.Setenv("XDG_CONFIG_HOME", "")

	// Create global config
	globalPath := "/home/user/.config/wherehouse/wherehouse.toml"
	require.NoError(t, fs.MkdirAll(filepath.Dir(globalPath), 0755))
	globalContent := `[database]
path = "/global/db.sqlite"

[user]
default_identity = "global-user"

[output]
default_format = "json"
quiet = false
`
	require.NoError(t, afero.WriteFile(fs, globalPath, []byte(globalContent), 0644))

	// Create local config (overrides some global values)
	localContent := `[database]
path = "/local/db.sqlite"

[output]
quiet = true
`
	require.NoError(t, afero.WriteFile(fs, "./wherehouse.toml", []byte(localContent), 0644))

	// Set environment variables (override both config files)
	t.Setenv("WHEREHOUSE_OUTPUT_DEFAULT_FORMAT", "human")

	cfg, err := NewWithFS(fs, "")
	require.NoError(t, err)

	// Verify precedence:
	// - database.path from local config (overrides global)
	// - user.default_identity from global config (not in local)
	// - output.default_format from environment (overrides both)
	// - output.quiet from local config (overrides global)
	assert.Equal(t, "/local/db.sqlite", cfg.Database.Path)
	assert.Equal(t, "global-user", cfg.User.DefaultIdentity)
	assert.Equal(t, "human", cfg.Output.DefaultFormat)
	assert.True(t, cfg.Output.Quiet)
}

// TestXDG_CONFIG_HOME tests XDG_CONFIG_HOME environment variable support.
func TestXDG_CONFIG_HOME(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Set up HOME and XDG_CONFIG_HOME
	t.Setenv("HOME", "/home/user")
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")

	// Create config at XDG location
	xdgPath := "/custom/config/wherehouse/wherehouse.toml"
	require.NoError(t, fs.MkdirAll(filepath.Dir(xdgPath), 0755))
	content := `[database]
path = "/xdg/db.sqlite"
`
	require.NoError(t, afero.WriteFile(fs, xdgPath, []byte(content), 0644))

	cfg, err := NewWithFS(fs, "")
	require.NoError(t, err)

	assert.Equal(t, "/xdg/db.sqlite", cfg.Database.Path)
}

// TestExplicitFileOverridesDefaults tests that explicit filepath bypasses default locations.
func TestExplicitFileOverridesDefaults(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Set up HOME for global config path resolution
	t.Setenv("HOME", "/home/user")
	t.Setenv("XDG_CONFIG_HOME", "")

	// Create global and local configs
	globalPath := "/home/user/.config/wherehouse/wherehouse.toml"
	require.NoError(t, fs.MkdirAll(filepath.Dir(globalPath), 0755))
	require.NoError(t, afero.WriteFile(fs, globalPath, []byte(`[database]
path = "/global/db.sqlite"
`), 0644))

	require.NoError(t, afero.WriteFile(fs, "./wherehouse.toml", []byte(`[database]
path = "/local/db.sqlite"
`), 0644))

	// Create explicit config file
	customPath := "/custom/override.toml"
	require.NoError(t, fs.MkdirAll(filepath.Dir(customPath), 0755))
	require.NoError(t, afero.WriteFile(fs, customPath, []byte(`[database]
path = "/explicit/db.sqlite"
`), 0644))

	// Load with explicit path
	cfg, err := NewWithFS(fs, customPath)
	require.NoError(t, err)

	// Should use explicit path, not global or local
	assert.Equal(t, "/explicit/db.sqlite", cfg.Database.Path)
}

// TestPathExpansionInConfig tests that tilde paths are preserved in config.
// Path expansion is done by caller using ExpandPath() function.
func TestPathExpansionInConfig(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Set up HOME
	t.Setenv("HOME", "/home/testuser")

	content := `[database]
path = "~/mydb.sqlite"
`
	require.NoError(t, afero.WriteFile(fs, "./wherehouse.toml", []byte(content), 0644))

	cfg, err := NewWithFS(fs, "")
	require.NoError(t, err)

	// Path should be preserved as-is (expansion happens on usage)
	assert.Equal(t, "~/mydb.sqlite", cfg.Database.Path)

	// When expanded, should produce correct path
	expanded, err := ExpandPath(cfg.Database.Path)
	require.NoError(t, err, "ExpandPath should succeed")
	assert.Equal(t, "/home/testuser/mydb.sqlite", expanded)
}

// TestEmptyLocalConfigWithGlobalValues tests merging when local config is empty.
func TestEmptyLocalConfigWithGlobalValues(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Set up HOME for global config path resolution
	t.Setenv("HOME", "/home/user")
	t.Setenv("XDG_CONFIG_HOME", "")

	// Create global config
	globalPath := "/home/user/.config/wherehouse/wherehouse.toml"
	require.NoError(t, fs.MkdirAll(filepath.Dir(globalPath), 0755))
	globalContent := `[database]
path = "/global/db.sqlite"

[user]
default_identity = "alice"

[output]
default_format = "json"
quiet = false
`
	require.NoError(t, afero.WriteFile(fs, globalPath, []byte(globalContent), 0644))

	// Create empty local config
	require.NoError(t, afero.WriteFile(fs, "./wherehouse.toml", []byte(""), 0644))

	cfg, err := NewWithFS(fs, "")
	require.NoError(t, err)

	// Should use all values from global (empty local doesn't override)
	assert.Equal(t, "/global/db.sqlite", cfg.Database.Path)
	assert.Equal(t, "alice", cfg.User.DefaultIdentity)
	assert.Equal(t, "json", cfg.Output.DefaultFormat)
	assert.False(t, cfg.Output.Quiet)
}

// TestValidateExportedFunction tests the exported Validate function.
func TestValidateExportedFunction(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{Path: "/valid/path.sqlite"},
		User:     UserConfig{DefaultIdentity: "alice"},
		Output:   OutputConfig{DefaultFormat: "human"},
	}

	err := Validate(cfg)
	require.NoError(t, err, "validation should succeed for valid config")

	// Test validation error
	cfg.Output.DefaultFormat = "invalid"
	err = Validate(cfg)
	require.Error(t, err, "validation should fail for invalid format")
	assert.Contains(t, err.Error(), "must be one of [human, json]")
}

// TestEnvironmentVariableOverrideAllLevels tests env vars override all config sources.
func TestEnvironmentVariableOverrideAllLevels(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Set up HOME for global config path resolution
	t.Setenv("HOME", "/home/user")
	t.Setenv("XDG_CONFIG_HOME", "")

	// Create global config
	globalPath := "/home/user/.config/wherehouse/wherehouse.toml"
	require.NoError(t, fs.MkdirAll(filepath.Dir(globalPath), 0755))
	globalContent := `[database]
path = "/global/db.sqlite"

[user]
default_identity = "alice"

[output]
default_format = "json"
quiet = false
`
	require.NoError(t, afero.WriteFile(fs, globalPath, []byte(globalContent), 0644))

	// Create local config with different values
	localContent := `[database]
path = "/local/db.sqlite"

[user]
default_identity = "bob"

[output]
default_format = "human"
quiet = false
`
	require.NoError(t, afero.WriteFile(fs, "./wherehouse.toml", []byte(localContent), 0644))

	// Set environment variables for all values
	t.Setenv("WHEREHOUSE_DATABASE_PATH", "/env/db.sqlite")
	t.Setenv("WHEREHOUSE_USER_DEFAULT_IDENTITY", "charlie")
	t.Setenv("WHEREHOUSE_OUTPUT_DEFAULT_FORMAT", "json")
	t.Setenv("WHEREHOUSE_OUTPUT_QUIET", "true")

	cfg, err := NewWithFS(fs, "")
	require.NoError(t, err)

	// All values should come from environment variables
	assert.Equal(t, "/env/db.sqlite", cfg.Database.Path)
	assert.Equal(t, "charlie", cfg.User.DefaultIdentity)
	assert.Equal(t, "json", cfg.Output.DefaultFormat)
	assert.True(t, cfg.Output.Quiet)
}

// TestPartialLocalConfigOverride tests that local config can override subset of global.
func TestPartialLocalConfigOverride(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Set up HOME for global config path resolution
	t.Setenv("HOME", "/home/user")
	t.Setenv("XDG_CONFIG_HOME", "")

	// Create global config with multiple values
	globalPath := "/home/user/.config/wherehouse/wherehouse.toml"
	require.NoError(t, fs.MkdirAll(filepath.Dir(globalPath), 0755))
	globalContent := `[database]
path = "/global/db.sqlite"

[user]
default_identity = "alice"

[output]
default_format = "json"
quiet = false
`
	require.NoError(t, afero.WriteFile(fs, globalPath, []byte(globalContent), 0644))

	// Create local config that overrides ONLY database path
	localContent := `[database]
path = "/local/db.sqlite"
`
	require.NoError(t, afero.WriteFile(fs, "./wherehouse.toml", []byte(localContent), 0644))

	cfg, err := NewWithFS(fs, "")
	require.NoError(t, err)

	// Local overrides database path, but others come from global
	assert.Equal(t, "/local/db.sqlite", cfg.Database.Path)
	assert.Equal(t, "alice", cfg.User.DefaultIdentity)
	assert.Equal(t, "json", cfg.Output.DefaultFormat)
	assert.False(t, cfg.Output.Quiet)
}

// TestComplexMergeScenario tests complex three-level merge with all config sources.
func TestComplexMergeScenario(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Set up HOME for global config path resolution
	t.Setenv("HOME", "/home/user")
	t.Setenv("XDG_CONFIG_HOME", "")

	// Create global config
	globalPath := "/home/user/.config/wherehouse/wherehouse.toml"
	require.NoError(t, fs.MkdirAll(filepath.Dir(globalPath), 0755))
	globalContent := `[database]
path = "/global/db.sqlite"

[user]
default_identity = "alice"

[output]
default_format = "json"
quiet = false
`
	require.NoError(t, afero.WriteFile(fs, globalPath, []byte(globalContent), 0644))

	// Create local config (overrides database and quiet)
	localContent := `[database]
path = "/local/db.sqlite"

[output]
quiet = true
`
	require.NoError(t, afero.WriteFile(fs, "./wherehouse.toml", []byte(localContent), 0644))

	// Set environment variables (override output format)
	t.Setenv("WHEREHOUSE_OUTPUT_DEFAULT_FORMAT", "human")

	cfg, err := NewWithFS(fs, "")
	require.NoError(t, err)

	// Verify complex merge:
	// - database.path from local (lowest precedence in local)
	// - user.default_identity from global (not overridden)
	// - output.default_format from environment (highest precedence)
	// - output.quiet from local (overrides global)
	assert.Equal(t, "/local/db.sqlite", cfg.Database.Path)
	assert.Equal(t, "alice", cfg.User.DefaultIdentity)
	assert.Equal(t, "human", cfg.Output.DefaultFormat)
	assert.True(t, cfg.Output.Quiet)
}
