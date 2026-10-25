package config

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadConfig_EnvWithoutConfigFile verifies every WHEREHOUSE_* env var
// applies even when no global or local config file exists (e.g. in a container).
func TestLoadConfig_EnvWithoutConfigFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/nonexistent")
	t.Setenv("WHEREHOUSE_DATABASE_PATH", "/data/wherehouse.db")
	t.Setenv("WHEREHOUSE_LOGGING_FILE_PATH", "/data/wherehouse.log")
	t.Setenv("WHEREHOUSE_LOGGING_LEVEL", "debug")
	t.Setenv("WHEREHOUSE_LOGGING_MAX_SIZE_MB", "10")
	t.Setenv("WHEREHOUSE_LOGGING_MAX_BACKUPS", "3")
	t.Setenv("WHEREHOUSE_USER_DEFAULT_IDENTITY", "web")
	t.Setenv("WHEREHOUSE_OUTPUT_DEFAULT_FORMAT", "json")
	t.Setenv("WHEREHOUSE_OUTPUT_QUIET", "2")
	t.Setenv("WHEREHOUSE_WEB_ACTOR_HEADER", "Tailscale-User-Login")

	cfg, err := loadConfig(afero.NewMemMapFs(), "")
	require.NoError(t, err)

	assert.Equal(t, "/data/wherehouse.db", cfg.Database.Path)
	assert.Equal(t, "/data/wherehouse.log", cfg.Logging.FilePath)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, 10, cfg.Logging.MaxSizeMB)
	assert.Equal(t, 3, cfg.Logging.MaxBackups)
	assert.Equal(t, "web", cfg.User.DefaultIdentity)
	assert.Equal(t, "json", cfg.Output.DefaultFormat)
	assert.Equal(t, 2, cfg.Output.Quiet)
	assert.Equal(t, "Tailscale-User-Login", cfg.Web.ActorHeader)
}

// TestLoadConfig_EnvOverridesExplicitFile verifies WHEREHOUSE_* env vars take
// precedence over every key in an explicitly specified config file, while
// file-only keys are still read from the file.
func TestLoadConfig_EnvOverridesExplicitFile(t *testing.T) {
	const cfgPath = "/cfg/wherehouse.toml"
	content := `[database]
path = "/file/wherehouse.db"

[logging]
file_path = "/file/wherehouse.log"
level = "warn"
max_size_mb = 5
max_backups = 1

[user]
default_identity = "file-user"

[user.os_username_map]
grue = "Grue"

[output]
default_format = "human"
quiet = 1

[web]
actor_header = "X-File-User"
`
	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, cfgPath, []byte(content), 0o644))

	t.Setenv("WHEREHOUSE_DATABASE_PATH", "/env/wherehouse.db")
	t.Setenv("WHEREHOUSE_LOGGING_FILE_PATH", "/env/wherehouse.log")
	t.Setenv("WHEREHOUSE_LOGGING_LEVEL", "debug")
	t.Setenv("WHEREHOUSE_LOGGING_MAX_SIZE_MB", "10")
	t.Setenv("WHEREHOUSE_LOGGING_MAX_BACKUPS", "3")
	t.Setenv("WHEREHOUSE_USER_DEFAULT_IDENTITY", "env-user")
	t.Setenv("WHEREHOUSE_OUTPUT_DEFAULT_FORMAT", "json")
	t.Setenv("WHEREHOUSE_OUTPUT_QUIET", "2")
	t.Setenv("WHEREHOUSE_WEB_ACTOR_HEADER", "X-Env-User")

	cfg, err := loadConfig(fs, cfgPath)
	require.NoError(t, err)

	assert.Equal(t, "/env/wherehouse.db", cfg.Database.Path)
	assert.Equal(t, "/env/wherehouse.log", cfg.Logging.FilePath)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, 10, cfg.Logging.MaxSizeMB)
	assert.Equal(t, 3, cfg.Logging.MaxBackups)
	assert.Equal(t, "env-user", cfg.User.DefaultIdentity)
	assert.Equal(t, "json", cfg.Output.DefaultFormat)
	assert.Equal(t, 2, cfg.Output.Quiet)
	assert.Equal(t, "X-Env-User", cfg.Web.ActorHeader)

	// Sentinel: not env-bound, so it can only come from the file.
	assert.Equal(t, map[string]string{"grue": "Grue"}, cfg.User.OSUsernameMap)
}

// TestLoadConfig_InvalidIntEnv verifies a non-integer value in an int-typed
// WHEREHOUSE_* env var fails loading with an error naming the config key.
func TestLoadConfig_InvalidIntEnv(t *testing.T) {
	tests := []struct {
		name            string
		envKey          string
		envValue        string
		wantErrContains string
	}{
		{
			name:            "quiet non-numeric",
			envKey:          "WHEREHOUSE_OUTPUT_QUIET",
			envValue:        "abc",
			wantErrContains: "output.quiet",
		},
		{
			name:            "max_size_mb non-numeric",
			envKey:          "WHEREHOUSE_LOGGING_MAX_SIZE_MB",
			envValue:        "abc",
			wantErrContains: "logging.max_size_mb",
		},
		{
			name:            "max_backups non-numeric",
			envKey:          "WHEREHOUSE_LOGGING_MAX_BACKUPS",
			envValue:        "abc",
			wantErrContains: "logging.max_backups",
		},
		{
			name:            "quiet float",
			envKey:          "WHEREHOUSE_OUTPUT_QUIET",
			envValue:        "1.5",
			wantErrContains: "output.quiet",
		},
		{
			name:            "quiet overflow",
			envKey:          "WHEREHOUSE_OUTPUT_QUIET",
			envValue:        "99999999999999999999",
			wantErrContains: "output.quiet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", "/nonexistent")
			t.Setenv(tt.envKey, tt.envValue)

			_, err := loadConfig(afero.NewMemMapFs(), "")
			require.ErrorContains(t, err, tt.wantErrContains)
		})
	}
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
