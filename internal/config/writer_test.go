package config

import (
	"bytes"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteDefault_AllDefaultsRoundTrip verifies the written file decodes back
// to exactly GetDefaults(), field for field.
func TestWriteDefault_AllDefaultsRoundTrip(t *testing.T) {
	testFs, defaultCfgFile := NewTestFsWithDefaultFile(t)

	data, err := afero.ReadFile(testFs, defaultCfgFile)
	require.NoError(t, err)

	var got Config

	require.NoError(t, toml.Unmarshal(data, &got))
	applyDefaults(&got)

	assert.Equal(t, GetDefaults(), &got)
}

// TestWriteDefault_QuotesUnusualPaths verifies a default database path with
// quote and backslash characters survives the write/read round trip.
func TestWriteDefault_QuotesUnusualPaths(t *testing.T) {
	if runtime.GOOS != goosLinux {
		t.Skip("default database path honours XDG_DATA_HOME only on linux")
	}

	t.Setenv("XDG_DATA_HOME", `/data/o'brien "quoted" \share`)

	testFs, defaultCfgFile := NewTestFsWithDefaultFile(t)

	data, err := afero.ReadFile(testFs, defaultCfgFile)
	require.NoError(t, err)

	var got Config

	require.NoError(t, toml.Unmarshal(data, &got))

	assert.Equal(t, GetDefaults().Database.Path, got.Database.Path)
}

// TestWriteDefault_EnvOverridesGeneratedFile verifies a WHEREHOUSE_* env var
// still wins over a freshly generated config file.
func TestWriteDefault_EnvOverridesGeneratedFile(t *testing.T) {
	t.Setenv("WHEREHOUSE_OUTPUT_DEFAULT_FORMAT", OutputFormatJSON)

	testFs, defaultCfgFile := NewTestFsWithDefaultFile(t)

	cfg, err := NewWithFS(testFs, defaultCfgFile)
	require.NoError(t, err)

	assert.True(t, cfg.IsJSON(), "env default_format should override the generated file")
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

// TestWriteDefault_AllKeysPresent verifies every Config leaf key is written,
// including zero-valued ones, so `config init` shows every setting.
func TestWriteDefault_AllKeysPresent(t *testing.T) {
	testFs, defaultCfgFile := NewTestFsWithDefaultFile(t)

	v := viper.New()
	v.SetFs(testFs)
	v.SetConfigFile(defaultCfgFile)
	v.SetConfigType("toml")
	require.NoError(t, v.ReadInConfig())

	for _, key := range leafKeys(t) {
		assert.True(t, v.IsSet(key), "key %q should be written", key)
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
