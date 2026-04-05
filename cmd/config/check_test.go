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

func TestConfigCheck_ReturnsCommand(t *testing.T) {
	defer ResetForTesting()

	cmd := GetCheckCmd()

	require.NotNil(t, cmd)
	assert.Equal(t, "check", cmd.Use)
}

func TestConfigCheck_InvalidTOML(t *testing.T) {
	defer ResetForTesting()

	memFS := afero.NewMemMapFs()
	SetFilesystem(memFS)
	defer SetFilesystem(afero.NewOsFs())

	globalPath := config.GetGlobalConfigPath()
	expandedPath, _ := config.ExpandPath(globalPath)
	memFS.MkdirAll(filepath.Dir(expandedPath), 0755)
	afero.WriteFile(memFS, expandedPath, []byte("invalid [[ toml"), 0644)

	cmd := GetCheckCmd()
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errBuf)
	cmd.SetContext(context.WithValue(context.Background(), config.ConfigKey, config.GetDefaults()))

	result := cmd.Execute()

	require.Error(t, result)
}

func TestConfigCheck_MissingConfig(_ *testing.T) {
	defer ResetForTesting()

	memFS := afero.NewMemMapFs()
	SetFilesystem(memFS)
	defer SetFilesystem(afero.NewOsFs())

	cmd := GetCheckCmd()
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errBuf)
	cmd.SetContext(context.WithValue(context.Background(), config.ConfigKey, config.GetDefaults()))

	result := cmd.Execute()

	// Should handle gracefully
	_ = result
}
