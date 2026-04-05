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
	testFs, defaultCfgFile := NewTestFsWithDefaultFile(t)

	// Read back through viper to verify all keys/values
	v := viper.New()
	v.SetFs(testFs)
	v.SetConfigFile(defaultCfgFile)
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

// TestWriteDefault_CreatesFile verifies file is created.
func TestWriteDefault_CreatesFile(t *testing.T) {
	testFs, defaultCfgFile := NewTestFsWithDefaultFile(t)

	t.Run("file is created", func(t *testing.T) {
		// Verify file exists
		exists, err := afero.Exists(testFs, defaultCfgFile)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("no overwrite without force", func(t *testing.T) {
		// Try to write again with force=false
		err := WriteDefault(testFs, defaultCfgFile, false)
		assert.ErrorContains(t, err, "already exists")
	})

	t.Run("force overwrite", func(t *testing.T) {
		// Overwrite with force=true
		require.NoError(t, WriteDefault(testFs, defaultCfgFile, true))

		// Verify file still exists
		exists, err := afero.Exists(testFs, defaultCfgFile)
		require.NoError(t, err)
		assert.True(t, exists)
	})
}

// TestWriteDefault_OutputIsParseable verifies viper output is valid TOML with expected sections.
func TestWriteDefault_OutputIsParseable(t *testing.T) {
	testFs, defaultCfgFile := NewTestFsWithDefaultFile(t)

	// Read file content directly and verify it's valid TOML
	data, err := afero.ReadFile(testFs, defaultCfgFile)
	require.NoError(t, err)

	require.NoError(t, validateTOML(t, data))

	// Verify all sections are present by checking we can read keys
	v := viper.New()
	v.SetFs(testFs)
	v.SetConfigFile(defaultCfgFile)
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

	testFs, defaultCfgFile := NewTestFsWithDefaultFile(t)

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			// Update key
			require.NoError(t, Set(testFs, defaultCfgFile, tt.key, tt.value))

			// Re-read and verify
			v := viper.New()
			v.SetFs(testFs)
			v.SetConfigFile(defaultCfgFile)
			v.SetConfigType("toml")
			require.NoError(t, v.ReadInConfig())

			assert.EqualValues(t, tt.want, v.Get(tt.key))
		})
	}
}

// TestSet_InvalidValue tests various invalid values for type-specific keys.
func TestSet_Invalid(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		key       string
		value     string
		errorText string
	}{
		{
			name:      "unknown key",
			key:       "unknown.key",
			value:     "fake",
			errorText: "unknown configuration key",
		},
		{
			name:      "logging.level invalid",
			key:       "logging.level",
			value:     "verbose",
			errorText: "must be one of",
		},
		{
			name:      "output.quiet invalid bool",
			key:       "output.quiet",
			value:     "maybe",
			errorText: "must be 'true' or 'false'",
		},
		{
			name:      "output.default_format invalid",
			key:       "output.default_format",
			value:     "xml",
			errorText: "must be 'human' or 'json'",
		},
		{
			name:      "logging.max_size_mb negative",
			key:       "logging.max_size_mb",
			value:     "-1",
			errorText: "non-negative integer",
		},
		{
			name:      "logging.max_size_mb non-integer",
			key:       "logging.max_size_mb",
			value:     "not-a-number",
			errorText: "non-negative integer",
		},
		{
			name:      "logging.max_backups negative",
			key:       "logging.max_backups",
			value:     "-1",
			errorText: "non-negative integer",
		},
	}

	testFs, defaultCfgFile := NewTestFsWithDefaultFile(t)

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := Set(testFs, defaultCfgFile, tt.key, tt.value)
			assert.ErrorContains(t, err, tt.errorText)
		})
	}
}

// TestSet_FileNotFound verifies error when file does not exist.
func TestSet_FileNotFound(t *testing.T) {
	testFs, _ := NewTestFs(t)
	path := "fake/config/path/config.toml"

	err := Set(testFs, path, "database.path", "/some/path.db")
	assert.ErrorContains(t, err, "reading config file")
}

// TestCheck_ValidFile verifies Check returns nil for valid TOML.
func TestCheck_ValidFile(t *testing.T) {
	testFs, defaultCfgFile := NewTestFsWithDefaultFile(t)

	t.Run("valid toml", func(t *testing.T) {
		require.NoError(t, Check(testFs, defaultCfgFile))
	})

	t.Run("invalid toml", func(t *testing.T) {
		invalidCfgFile := filepath.Join(filepath.Dir(defaultCfgFile), "bad-config.toml")

		// Write malformed TOML
		badContent := "[database\npath = broken"
		require.NoError(t, afero.WriteFile(testFs, invalidCfgFile, []byte(badContent), 0o644))

		err := Check(testFs, invalidCfgFile)
		assert.ErrorContains(t, err, "parsing config file")
	})

	t.Run("invalid value", func(t *testing.T) {
		badValueFile := filepath.Join(filepath.Dir(defaultCfgFile), "bad-config.toml")
		// Write TOML with invalid output format
		content := `[database]
path = "/valid/db.sqlite"

[output]
default_format = "xml"
`
		require.NoError(t, afero.WriteFile(testFs, badValueFile, []byte(content), 0o644))

		err := Check(testFs, badValueFile)
		assert.ErrorContains(t, err, "validating config")
	})
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
	require.ErrorContains(t, err, "unknown configuration key")
}

// TestGetValue_InvalidFormat tests invalid key format.
func TestGetValue_InvalidFormat(t *testing.T) {
	cfg := &Config{}
	applyDefaults(cfg)

	// Test key without dot separator
	_, err := GetValue(cfg, "invalid_key")
	require.Error(t, err, "invalid key format")

	// Test key with section that doesn't exist
	_, err = GetValue(cfg, "nonexistent.field")
	require.Error(t, err, "unknown configuration key")
}

// TestSet_PreservesOtherValues verifies that Set preserves other config values.
func TestSet_PreservesOtherValues(t *testing.T) {
	testFs, defaultCfgFile := NewTestFsWithDefaultFile(t)

	// Set one key
	require.NoError(t, Set(testFs, defaultCfgFile, "database.path", "/custom/db.sqlite"))

	// Set another key and verify the first is preserved
	require.NoError(t, Set(testFs, defaultCfgFile, "output.quiet", "true"))

	// Re-read and verify both values
	v := viper.New()
	v.SetFs(testFs)
	v.SetConfigFile(defaultCfgFile)
	v.SetConfigType("toml")
	require.NoError(t, v.ReadInConfig())

	assert.Equal(t, "/custom/db.sqlite", v.GetString("database.path"))
	assert.True(t, v.GetBool("output.quiet"))
}

// TestCheck_EmptyFile verifies Check handles empty TOML file gracefully.
func TestCheck_EmptyFile(t *testing.T) {
	testFs, testDir := NewTestFs(t)

	// Write empty file
	emptyFile, err := afero.TempFile(testFs, testDir, "")
	require.NoError(t, err)

	// Should not error; empty config gets defaults applied
	err = Check(testFs, emptyFile.Name())
	require.NoError(t, err)
}

// TestSet_MultipleUpdates verifies sequential Set calls work correctly.
func TestSet_MultipleUpdates(t *testing.T) {
	testFs, defaultCfgFile := NewTestFsWithDefaultFile(t)

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
		require.NoError(t, Set(testFs, defaultCfgFile, u.key, u.value))
	}

	// Verify final values
	v := viper.New()
	v.SetFs(testFs)
	v.SetConfigFile(defaultCfgFile)
	v.SetConfigType("toml")
	require.NoError(t, v.ReadInConfig())

	assert.Equal(t, "/path/2.db", v.GetString("database.path"))
	assert.Equal(t, "debug", v.GetString("logging.level"))
	assert.True(t, v.GetBool("output.quiet"))
}

// TestWriteDefault_AllKeysPresent verifies all expected keys are in the output.
func TestWriteDefault_AllKeysPresent(t *testing.T) {
	testFs, defaultCfgFile := NewTestFsWithDefaultFile(t)

	v := viper.New()
	v.SetFs(testFs)
	v.SetConfigFile(defaultCfgFile)
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

func validateTOML(t *testing.T, data []byte) error {
	t.Helper()

	var cfg Config

	v := viper.New()
	v.SetConfigType("toml")
	require.NoError(t, v.ReadConfig(bytes.NewReader(data)))

	return v.Unmarshal(&cfg)
}
