package config

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteDefault_AllDefaultsRoundTrip verifies round-trip: write via WriteDefault,
// read back via viper, verify all keys match GetDefaults() values.
func TestWriteDefault_AllDefaultsRoundTrip(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/tmp/test-config.toml"

	// Write defaults
	err := WriteDefault(fs, path, false)
	require.NoError(t, err)

	// Read back through viper to verify all keys/values
	v := viper.New()
	v.SetFs(fs)
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	require.NoError(t, v.ReadInConfig())

	defaults := GetDefaults()

	// Verify all default values are present in the written file
	assert.Equal(t, defaults.Database.Path, v.GetString("database.path"))
	assert.Equal(t, defaults.Logging.Level, v.GetString("logging.level"))
	assert.Equal(t, defaults.Logging.FilePath, v.GetString("logging.file_path"))
	assert.Equal(t, defaults.Logging.MaxSizeMB, v.GetInt("logging.max_size_mb"))
	assert.Equal(t, defaults.Logging.MaxBackups, v.GetInt("logging.max_backups"))
	assert.Equal(t, defaults.User.DefaultIdentity, v.GetString("user.default_identity"))
	assert.Equal(t, defaults.Output.DefaultFormat, v.GetString("output.default_format"))
	assert.Equal(t, defaults.Output.Quiet, v.GetInt("output.quiet"))
}

// TestWriteDefault_CreatesFile verifies file is created in memfs.
func TestWriteDefault_CreatesFile(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/tmp/test-config.toml"

	err := WriteDefault(fs, path, false)
	require.NoError(t, err)

	// Verify file exists
	exists, err := afero.Exists(fs, path)
	require.NoError(t, err)
	assert.True(t, exists)
}

// TestWriteDefault_FailsIfExists verifies error when force=false and file exists.
func TestWriteDefault_FailsIfExists(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/tmp/test-config.toml"

	// Create initial file
	require.NoError(t, WriteDefault(fs, path, false))

	// Try to write again with force=false
	err := WriteDefault(fs, path, false)
	assert.ErrorContains(t, err, "already exists")
}

// TestWriteDefault_ForceOverwrites verifies force=true overwrites existing file.
func TestWriteDefault_ForceOverwrites(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/tmp/test-config.toml"

	// Create initial file
	require.NoError(t, WriteDefault(fs, path, false))

	// Verify first write succeeded
	exists, err := afero.Exists(fs, path)
	require.NoError(t, err)
	assert.True(t, exists)

	// Overwrite with force=true
	require.NoError(t, WriteDefault(fs, path, true))

	// Verify file still exists and is readable
	exists, err = afero.Exists(fs, path)
	require.NoError(t, err)
	assert.True(t, exists)

	// Verify content is valid
	v := viper.New()
	v.SetFs(fs)
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	require.NoError(t, v.ReadInConfig())
}

// TestWriteDefault_CreatesParentDirs verifies parent directories are created.
func TestWriteDefault_CreatesParentDirs(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/deep/nested/dir/config.toml"

	require.NoError(t, WriteDefault(fs, path, false))

	// Verify file exists
	exists, err := afero.Exists(fs, path)
	require.NoError(t, err)
	assert.True(t, exists)

	// Verify parent directories were created
	dir := filepath.Dir(path)
	assert.DirExists(t, dir)
}

// TestWriteDefault_OutputIsParseable verifies viper output is valid TOML with expected sections.
func TestWriteDefault_OutputIsParseable(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/tmp/test-config.toml"

	err := WriteDefault(fs, path, false)
	require.NoError(t, err)

	// Read file content directly and verify it's valid TOML
	data, err := afero.ReadFile(fs, path)
	require.NoError(t, err)

	// Verify we can parse it as TOML and unmarshal to Config
	var cfg Config
	require.NoError(t, unmarshalTOML(data, &cfg))

	// Verify all sections are present by checking we can read keys
	v := viper.New()
	v.SetFs(fs)
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	require.NoError(t, v.ReadInConfig())

	// Verify expected sections exist
	assert.True(t, v.IsSet("database"))
	assert.True(t, v.IsSet("logging"))
	assert.True(t, v.IsSet("user"))
	assert.True(t, v.IsSet("output"))
}

// TestSet_UpdatesValue tests updating each settable key and verifying via viper re-read.
func TestSet_UpdatesValue(t *testing.T) {
	cases := []struct {
		name  string
		key   string
		value string
		want  any
	}{
		{
			name:  "database.path",
			key:   "database.path",
			value: "/custom/db.sqlite",
			want:  "/custom/db.sqlite",
		},
		{
			name:  "logging.file_path",
			key:   "logging.file_path",
			value: "/custom/logfile.log",
			want:  "/custom/logfile.log",
		},
		{
			name:  "logging.level debug",
			key:   "logging.level",
			value: "debug",
			want:  "debug",
		},
		{
			name:  "logging.max_size_mb",
			key:   "logging.max_size_mb",
			value: "100",
			want:  100,
		},
		{
			name:  "logging.max_backups",
			key:   "logging.max_backups",
			value: "5",
			want:  5,
		},
		{
			name:  "user.default_identity",
			key:   "user.default_identity",
			value: "alice",
			want:  "alice",
		},
		{
			name:  "output.default_format human",
			key:   "output.default_format",
			value: "human",
			want:  "human",
		},
		{
			name:  "output.default_format json",
			key:   "output.default_format",
			value: "json",
			want:  "json",
		},
		{
			name:  "output.quiet true",
			key:   "output.quiet",
			value: "true",
			want:  true,
		},
		{
			name:  "output.quiet false",
			key:   "output.quiet",
			value: "false",
			want:  false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			path := "/tmp/test-config.toml"

			// Create initial config
			require.NoError(t, WriteDefault(fs, path, false))

			// Update key
			require.NoError(t, Set(fs, path, tt.key, tt.value))

			// Re-read and verify
			v := viper.New()
			v.SetFs(fs)
			v.SetConfigFile(path)
			v.SetConfigType("toml")
			require.NoError(t, v.ReadInConfig())

			assert.EqualValues(t, tt.want, v.Get(tt.key))
		})
	}
}

// TestSet_UnknownKey verifies error for unknown keys.
func TestSet_UnknownKey(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/tmp/test-config.toml"

	require.NoError(t, WriteDefault(fs, path, false))

	err := Set(fs, path, "unknown.key", "value")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown configuration key")
}

// TestSet_InvalidValue tests various invalid values for type-specific keys.
func TestSet_InvalidValue(t *testing.T) {
	cases := []struct {
		name        string
		key         string
		value       string
		expectError bool
		errorText   string
	}{
		{
			name:        "logging.level invalid",
			key:         "logging.level",
			value:       "verbose",
			expectError: true,
			errorText:   "must be one of",
		},
		{
			name:        "output.quiet invalid bool",
			key:         "output.quiet",
			value:       "maybe",
			expectError: true,
			errorText:   "must be 'true' or 'false'",
		},
		{
			name:        "output.default_format invalid",
			key:         "output.default_format",
			value:       "xml",
			expectError: true,
			errorText:   "must be 'human' or 'json'",
		},
		{
			name:        "logging.max_size_mb negative",
			key:         "logging.max_size_mb",
			value:       "-1",
			expectError: true,
			errorText:   "non-negative integer",
		},
		{
			name:        "logging.max_size_mb non-integer",
			key:         "logging.max_size_mb",
			value:       "not-a-number",
			expectError: true,
			errorText:   "non-negative integer",
		},
		{
			name:        "logging.max_backups negative",
			key:         "logging.max_backups",
			value:       "-1",
			expectError: true,
			errorText:   "non-negative integer",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			path := "/tmp/test-config.toml"

			require.NoError(t, WriteDefault(fs, path, false))

			err := Set(fs, path, tt.key, tt.value)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorText)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestSet_FileNotFound verifies error when file does not exist.
func TestSet_FileNotFound(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/nonexistent/config.toml"

	err := Set(fs, path, "database.path", "/some/path.db")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading config file")
}

// TestCheck_ValidFile verifies Check returns nil for valid TOML.
func TestCheck_ValidFile(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/tmp/test-config.toml"

	// Create valid config via WriteDefault
	require.NoError(t, WriteDefault(fs, path, false))

	err := Check(fs, path)
	require.NoError(t, err)
}

// TestCheck_InvalidToml verifies Check returns error for malformed TOML.
func TestCheck_InvalidToml(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/tmp/bad-config.toml"

	// Write malformed TOML
	badContent := `[database
path = "broken"`
	require.NoError(t, afero.WriteFile(fs, path, []byte(badContent), 0o644))

	err := Check(fs, path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing config file")
}

// TestCheck_FailsValidation verifies Check returns error for TOML with invalid values.
func TestCheck_FailsValidation(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/tmp/invalid-config.toml"

	// Write TOML with invalid output format
	content := `[database]
path = "/valid/db.sqlite"

[output]
default_format = "xml"
`
	require.NoError(t, afero.WriteFile(fs, path, []byte(content), 0o644))

	err := Check(fs, path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validating config")
}

// TestGetValue_AllKeys tests GetValue for all supported keys.
func TestGetValue_AllKeys(t *testing.T) {
	cases := []struct {
		name     string
		key      string
		setupCfg func(*Config)
		want     any
	}{
		{
			name: "database.path",
			key:  "database.path",
			setupCfg: func(cfg *Config) {
				cfg.Database.Path = "/custom/db.sqlite"
			},
			want: "/custom/db.sqlite",
		},
		{
			name: "logging.file_path",
			key:  "logging.file_path",
			setupCfg: func(cfg *Config) {
				cfg.Logging.FilePath = "/custom/log.txt"
			},
			want: "/custom/log.txt",
		},
		{
			name: "logging.level",
			key:  "logging.level",
			setupCfg: func(cfg *Config) {
				cfg.Logging.Level = "debug"
			},
			want: "debug",
		},
		{
			name: "logging.max_size_mb",
			key:  "logging.max_size_mb",
			setupCfg: func(cfg *Config) {
				cfg.Logging.MaxSizeMB = 100
			},
			want: 100,
		},
		{
			name: "logging.max_backups",
			key:  "logging.max_backups",
			setupCfg: func(cfg *Config) {
				cfg.Logging.MaxBackups = 5
			},
			want: 5,
		},
		{
			name: "user.default_identity",
			key:  "user.default_identity",
			setupCfg: func(cfg *Config) {
				cfg.User.DefaultIdentity = "alice"
			},
			want: "alice",
		},
		{
			name: "user.os_username_map",
			key:  "user.os_username_map",
			setupCfg: func(cfg *Config) {
				cfg.User.OSUsernameMap = map[string]string{
					"jdoe": "john.doe",
				}
			},
			want: map[string]string{"jdoe": "john.doe"},
		},
		{
			name: "output.default_format",
			key:  "output.default_format",
			setupCfg: func(cfg *Config) {
				cfg.Output.DefaultFormat = "json"
			},
			want: "json",
		},
		{
			name: "output.quiet",
			key:  "output.quiet",
			setupCfg: func(cfg *Config) {
				cfg.Output.Quiet = 1
			},
			want: 1,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			applyDefaults(cfg)
			tt.setupCfg(cfg)

			val, err := GetValue(cfg, tt.key)
			require.NoError(t, err)
			assert.EqualValues(t, tt.want, val)
		})
	}
}

// TestGetValue_UnknownKey verifies error for unknown keys.
func TestGetValue_UnknownKey(t *testing.T) {
	cfg := &Config{}
	applyDefaults(cfg)

	_, err := GetValue(cfg, "unknown.key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown configuration key")
}

// TestGetValue_InvalidFormat tests invalid key format.
func TestGetValue_InvalidFormat(t *testing.T) {
	cfg := &Config{}
	applyDefaults(cfg)

	// Test key without dot separator
	_, err := GetValue(cfg, "invalid_key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid key format")

	// Test key with section that doesn't exist
	_, err = GetValue(cfg, "nonexistent.field")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown configuration key")
}

// TestSet_PreservesOtherValues verifies that Set preserves other config values.
func TestSet_PreservesOtherValues(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/tmp/test-config.toml"

	// Create initial config
	require.NoError(t, WriteDefault(fs, path, false))

	// Set one key
	require.NoError(t, Set(fs, path, "database.path", "/custom/db.sqlite"))

	// Set another key and verify the first is preserved
	require.NoError(t, Set(fs, path, "output.quiet", "true"))

	// Re-read and verify both values
	v := viper.New()
	v.SetFs(fs)
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	require.NoError(t, v.ReadInConfig())

	assert.Equal(t, "/custom/db.sqlite", v.GetString("database.path"))
	assert.True(t, v.GetBool("output.quiet"))
}

// TestCheck_EmptyFile verifies Check handles empty TOML file gracefully.
func TestCheck_EmptyFile(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/tmp/empty-config.toml"

	// Write empty file
	require.NoError(t, afero.WriteFile(fs, path, []byte(""), 0o644))

	// Should not error; empty config gets defaults applied
	err := Check(fs, path)
	require.NoError(t, err)
}

// TestSet_MultipleUpdates verifies sequential Set calls work correctly.
func TestSet_MultipleUpdates(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/tmp/test-config.toml"

	// Create initial config
	require.NoError(t, WriteDefault(fs, path, false))

	// Make multiple updates
	updates := []struct {
		key   string
		value string
	}{
		{"database.path", "/path/1.db"},
		{"logging.level", "debug"},
		{"output.quiet", "true"},
		{"database.path", "/path/2.db"}, // Update again
	}

	for _, u := range updates {
		require.NoError(t, Set(fs, path, u.key, u.value))
	}

	// Verify final values
	v := viper.New()
	v.SetFs(fs)
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	require.NoError(t, v.ReadInConfig())

	assert.Equal(t, "/path/2.db", v.GetString("database.path"))
	assert.Equal(t, "debug", v.GetString("logging.level"))
	assert.True(t, v.GetBool("output.quiet"))
}

// TestWriteDefault_AllKeysPresent verifies all expected keys are in the output.
func TestWriteDefault_AllKeysPresent(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/tmp/test-config.toml"

	require.NoError(t, WriteDefault(fs, path, false))

	v := viper.New()
	v.SetFs(fs)
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	require.NoError(t, v.ReadInConfig())

	// Verify all expected keys exist
	expectedKeys := []string{
		"database.path",
		"logging.file_path",
		"logging.level",
		"logging.max_size_mb",
		"logging.max_backups",
		"user.default_identity",
		"user.os_username_map",
		"output.default_format",
		"output.quiet",
	}

	for _, key := range expectedKeys {
		assert.True(t, v.IsSet(key), "key %q should be set", key)
	}
}

// unmarshalTOML is a helper to unmarshal TOML data using go-toml.
// This mimics what Check() does internally.
func unmarshalTOML(data []byte, cfg *Config) error {
	// Import toml at the top of the test file and use it here
	// For now, we'll use viper to do the unmarshaling
	v := viper.New()
	v.SetConfigType("toml")
	if err := v.ReadConfig(bytes.NewReader(data)); err != nil {
		return err
	}
	return v.Unmarshal(cfg)
}
