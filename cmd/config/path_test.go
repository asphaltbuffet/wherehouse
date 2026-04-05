package config

import (
	"bytes"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigPath_ShowsGlobalPath(t *testing.T) {
	defer ResetForTesting()

	memFS := afero.NewMemMapFs()
	SetFilesystem(memFS)
	defer SetFilesystem(afero.NewOsFs())

	cmd := GetPathCmd()
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(errBuf)

	result := cmd.Execute()

	require.NoError(t, result)
	output := out.String()
	assert.NotEmpty(t, output)
}

func TestConfigPath_AllFlag(t *testing.T) {
	defer ResetForTesting()

	memFS := afero.NewMemMapFs()
	SetFilesystem(memFS)
	defer SetFilesystem(afero.NewOsFs())

	cmd := GetPathCmd()
	all := cmd.Flags().Lookup("all")

	assert.NotNil(t, all)
}

func TestConfigPath_HasCorrectStructure(t *testing.T) {
	defer ResetForTesting()

	cmd := GetPathCmd()

	assert.NotNil(t, cmd)
	assert.Equal(t, "path", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}
