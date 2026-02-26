package config

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetFilesystem(t *testing.T) {
	original := cmdFS
	defer func() { cmdFS = original }()

	memFS := afero.NewMemMapFs()
	SetFilesystem(memFS)

	assert.Same(t, memFS, cmdFS)
}

func TestFileExists_FileExists(t *testing.T) {
	fs := afero.NewMemMapFs()
	afero.WriteFile(fs, "/test/file.txt", []byte("content"), 0644)

	exists, err := fileExists(fs, "/test/file.txt")

	require.NoError(t, err)
	assert.True(t, exists)
}

func TestFileExists_FileNotFound(t *testing.T) {
	fs := afero.NewMemMapFs()

	exists, err := fileExists(fs, "/nonexistent/file.txt")

	require.NoError(t, err)
	assert.False(t, exists)
}

func TestFileExists_NestedPath(t *testing.T) {
	fs := afero.NewMemMapFs()
	afero.WriteFile(fs, "/a/b/c/file.txt", []byte("content"), 0644)

	exists, err := fileExists(fs, "/a/b/c/file.txt")

	require.NoError(t, err)
	assert.True(t, exists)
}

func TestEnsureDir_CreateNew(t *testing.T) {
	fs := afero.NewMemMapFs()

	err := ensureDir(fs, "/test/dir")

	require.NoError(t, err)
	exists, err := afero.DirExists(fs, "/test/dir")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestEnsureDir_CreateNested(t *testing.T) {
	fs := afero.NewMemMapFs()

	err := ensureDir(fs, "/a/b/c/d")

	require.NoError(t, err)
	exists, err := afero.DirExists(fs, "/a/b/c/d")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestEnsureDir_AlreadyExists(t *testing.T) {
	fs := afero.NewMemMapFs()
	fs.MkdirAll("/test/dir", 0755)

	err := ensureDir(fs, "/test/dir")

	require.NoError(t, err)
}
