package config

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/config"
)

func TestConfigInit_CreatesGlobalConfig(t *testing.T) {
	defer ResetForTesting()

	memFS := afero.NewMemMapFs()
	SetFilesystem(memFS)
	defer SetFilesystem(afero.NewOsFs())

	cmd := GetInitCmd()
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errBuf)
	cmd.SetContext(context.WithValue(context.Background(), config.ConfigKey, config.GetDefaults()))

	// Execute init command
	result := cmd.Execute()

	require.NoError(t, result)

	// Verify file was created (check for default path)
	globalPath := config.GetGlobalConfigPath()
	expandedPath, _ := config.ExpandPath(globalPath)
	exists, _ := afero.Exists(memFS, expandedPath)
	assert.True(t, exists, "Config file should exist at %s", expandedPath)
}

func TestConfigInit_FailsWhenFileExists(t *testing.T) {
	defer ResetForTesting()

	memFS := afero.NewMemMapFs()
	SetFilesystem(memFS)
	defer SetFilesystem(afero.NewOsFs())

	// Create config file first
	globalPath := config.GetGlobalConfigPath()
	expandedPath, _ := config.ExpandPath(globalPath)
	memFS.MkdirAll(filepath.Dir(expandedPath), 0755)
	afero.WriteFile(memFS, expandedPath, []byte("existing"), 0644)

	cmd := GetInitCmd()
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errBuf)
	cmd.SetContext(context.WithValue(context.Background(), config.ConfigKey, config.GetDefaults()))

	result := cmd.Execute()

	require.Error(t, result)
	assert.Contains(t, result.Error(), "already exists")
}

func TestConfigInit_OverwritesWithForce(t *testing.T) {
	defer ResetForTesting()

	memFS := afero.NewMemMapFs()
	SetFilesystem(memFS)
	defer SetFilesystem(afero.NewOsFs())

	// Create config file first
	globalPath := config.GetGlobalConfigPath()
	expandedPath, _ := config.ExpandPath(globalPath)
	memFS.MkdirAll(filepath.Dir(expandedPath), 0755)
	afero.WriteFile(memFS, expandedPath, []byte("old"), 0644)

	cmd := GetInitCmd()
	cmd.SetArgs([]string{"--force"})
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errBuf)
	cmd.SetContext(context.WithValue(context.Background(), config.ConfigKey, config.GetDefaults()))

	result := cmd.Execute()

	require.NoError(t, result)

	// Verify file was overwritten
	content, _ := afero.ReadFile(memFS, expandedPath)
	assert.NotEqual(t, []byte("old"), content)
}

func TestConfigInit_ForceFlag(t *testing.T) {
	defer ResetForTesting()

	cmd := GetInitCmd()
	force := cmd.Flags().Lookup("force")

	assert.NotNil(t, force)
	assert.Equal(t, "f", force.Shorthand)
}

func TestConfigInit_LocalFlag(t *testing.T) {
	defer ResetForTesting()

	cmd := GetInitCmd()
	local := cmd.Flags().Lookup("local")

	assert.NotNil(t, local)
	assert.Equal(t, "local", local.Name)
}

func TestConfigInit_OutputSuccess(t *testing.T) {
	defer ResetForTesting()

	memFS := afero.NewMemMapFs()
	SetFilesystem(memFS)
	defer SetFilesystem(afero.NewOsFs())

	cmd := GetInitCmd()
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errBuf)
	cmd.SetContext(context.WithValue(context.Background(), config.ConfigKey, config.GetDefaults()))

	result := cmd.Execute()
	require.NoError(t, result)

	output := out.String()
	assert.NotEmpty(t, output)
}

func TestConfigInit_CreatesParentDirectories(t *testing.T) {
	defer ResetForTesting()

	memFS := afero.NewMemMapFs()
	SetFilesystem(memFS)
	defer SetFilesystem(afero.NewOsFs())

	// Execute init
	cmd := GetInitCmd()
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errBuf)
	cmd.SetContext(context.WithValue(context.Background(), config.ConfigKey, config.GetDefaults()))

	result := cmd.Execute()
	require.NoError(t, result)

	// Verify parent directories exist
	globalPath := config.GetGlobalConfigPath()
	expandedPath, _ := config.ExpandPath(globalPath)
	parent := filepath.Dir(expandedPath)

	exists, _ := afero.DirExists(memFS, parent)
	assert.True(t, exists)
}
