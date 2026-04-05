package config

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// NewTestFs is a helper function to create a MemMapFS and temporary directory.
func NewTestFs(t *testing.T) (afero.Fs, string) {
	t.Helper()

	testFs := afero.NewMemMapFs()
	testDir, err := afero.TempDir(testFs, "", "")
	require.NoError(t, err)

	return testFs, testDir
}

func NewTestFsWithDefaultFile(t *testing.T) (afero.Fs, string) {
	t.Helper()

	testFs := afero.NewMemMapFs()

	testDir, err := afero.TempDir(testFs, "", "")
	require.NoError(t, err)
	defaultConfigFile := filepath.Join(testDir, "test-config.toml")

	require.NoError(t, WriteDefault(testFs, defaultConfigFile, false))

	return testFs, defaultConfigFile
}
