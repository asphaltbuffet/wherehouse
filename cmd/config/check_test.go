package config

import (
	"bytes"
	"context"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	internalconfig "github.com/asphaltbuffet/wherehouse/internal/config"
)

func TestConfigCheck(t *testing.T) {
	cmd := NewConfigCmd()
	require.NotNil(t, cmd)
}

func TestConfigCheck_Quiet_NoFiles_SuppressesInfo(t *testing.T) {
	origFS := cmdFS
	cmdFS = afero.NewMemMapFs()
	t.Cleanup(func() { cmdFS = origFS })

	cmd := NewCheckCmd()
	cmd.SetContext(
		context.WithValue(t.Context(), internalconfig.ConfigKey, apptesting.NewTestConfig(t, apptesting.WithQuiet())),
	)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	require.NoError(t, cmd.Execute())
	assert.Empty(t, stdout.String())
	assert.Empty(t, stderr.String())
}

func runCheckWithEnv(t *testing.T) (string, string, error) {
	t.Helper()
	origFS := cmdFS
	cmdFS = afero.NewMemMapFs()
	t.Cleanup(func() { cmdFS = origFS })

	cmd := NewCheckCmd()
	cmd.SetContext(
		context.WithValue(t.Context(), internalconfig.ConfigKey, apptesting.NewTestConfig(t)),
	)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func TestConfigCheck_EffectiveConfigValid(t *testing.T) {
	t.Setenv("WHEREHOUSE_LOGGING_FILE_PATH", "-")

	stdout, _, err := runCheckWithEnv(t)

	require.NoError(t, err)
	assert.Contains(t, stdout, "Effective config valid (files + environment)")
}

func TestConfigCheck_EffectiveConfigInvalidFromEnv(t *testing.T) {
	t.Setenv("WHEREHOUSE_LOGGING_FILE_PATH", "-")
	t.Setenv("WHEREHOUSE_LOGGING_MAX_SIZE_MB", "10")

	stdout, _, err := runCheckWithEnv(t)

	require.Error(t, err)
	assert.Contains(t, stdout, "Effective configuration invalid (files + environment)")
	assert.Contains(t, stdout, `file_path "-" (stderr)`)
}

func TestConfigCheck_LegacyLogPathIsInformational(t *testing.T) {
	t.Setenv(internalconfig.LegacyLogPathEnv, "/tmp/legacy.log")

	_, stderr, err := runCheckWithEnv(t)

	require.NoError(t, err, "legacy env var must not fail the check")
	assert.Contains(t, stderr, "WHEREHOUSE_LOG_PATH is no longer supported; use WHEREHOUSE_LOGGING_FILE_PATH")
}

func TestConfigCheck_ValidFileInvalidEffectiveFromEnv(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("WHEREHOUSE_LOGGING_FILE_PATH", "-")

	path, err := internalconfig.ExpandPath(internalconfig.GetGlobalConfigPath())
	require.NoError(t, err)

	origFS := cmdFS
	cmdFS = afero.NewMemMapFs()
	t.Cleanup(func() { cmdFS = origFS })
	require.NoError(t, afero.WriteFile(cmdFS, path, []byte("[logging]\nmax_size_mb = 10\n"), 0o644))

	cmd := NewCheckCmd()
	cmd.SetContext(
		context.WithValue(t.Context(), internalconfig.ConfigKey, apptesting.NewTestConfig(t)),
	)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	require.Error(t, cmd.Execute())
	assert.Contains(t, stdout.String(), "Global config valid")
	assert.Contains(t, stdout.String(), "Effective configuration invalid (files + environment)")
}
