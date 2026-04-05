package config

import (
	"bytes"
	"context"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/config"
)

func TestConfigGet_MissingConfig(t *testing.T) {
	defer ResetForTesting()

	memFS := afero.NewMemMapFs()
	SetFilesystem(memFS)
	defer SetFilesystem(afero.NewOsFs())

	cmd := GetGetCmd()
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errBuf)
	cmd.SetContext(context.WithValue(context.Background(), config.ConfigKey, config.GetDefaults()))

	result := cmd.Execute()

	// With no config file, command succeeds and shows default values
	require.NoError(t, result)
}

func TestConfigGet_ReturnsCommand(t *testing.T) {
	defer ResetForTesting()

	cmd := GetGetCmd()

	assert.NotNil(t, cmd)
	assert.NotEmpty(t, cmd.Use)
}

func TestConfigGet_HasRunE(t *testing.T) {
	defer ResetForTesting()

	cmd := GetGetCmd()

	assert.NotNil(t, cmd.RunE)
}
