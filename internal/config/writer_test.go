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
	assert.Equal(t, defaults.Web.ActorHeader, v.GetString("web.actor_header"))
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
