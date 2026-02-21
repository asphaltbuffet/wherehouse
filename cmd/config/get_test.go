package config

import (
	"bytes"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	result := cmd.Execute()

	require.Error(t, result)
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
