package config

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadConfig_EnvWithoutConfigFile verifies WHEREHOUSE_* env vars apply
// even when no global or local config file exists (e.g. in a container).
func TestLoadConfig_EnvWithoutConfigFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/nonexistent")
	t.Setenv("WHEREHOUSE_DATABASE_PATH", "/data/wherehouse.db")
	t.Setenv("WHEREHOUSE_LOGGING_FILE_PATH", "/dev/stderr")
	t.Setenv("WHEREHOUSE_LOGGING_LEVEL", "debug")
	t.Setenv("WHEREHOUSE_USER_DEFAULT_IDENTITY", "web")
	t.Setenv("WHEREHOUSE_OUTPUT_DEFAULT_FORMAT", "json")

	cfg, err := loadConfig(afero.NewMemMapFs(), "")
	require.NoError(t, err)

	assert.Equal(t, "/data/wherehouse.db", cfg.Database.Path)
	assert.Equal(t, "/dev/stderr", cfg.Logging.FilePath)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "web", cfg.User.DefaultIdentity)
	assert.Equal(t, "json", cfg.Output.DefaultFormat)
}

// TestLoadConfig_MapEnvIgnored verifies a map-typed key set via env does not
// break loading; a single env string cannot decode into a map.
func TestLoadConfig_MapEnvIgnored(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/nonexistent")
	t.Setenv("WHEREHOUSE_USER_OS_USERNAME_MAP", "grue=Grue")
	t.Setenv("WHEREHOUSE_DATABASE_PATH", "/data/wherehouse.db")

	cfg, err := loadConfig(afero.NewMemMapFs(), "")
	require.NoError(t, err)

	assert.Equal(t, "/data/wherehouse.db", cfg.Database.Path)
	assert.Empty(t, cfg.User.OSUsernameMap)
}
