package store_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/store"
)

func TestRunMigrations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "migrate.db")
	s, err := store.Open(store.Config{Path: path})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	require.NoError(t, s.RunMigrations())

	ver, dirty, err := s.GetMigrationVersion()
	require.NoError(t, err)
	assert.False(t, dirty)
	assert.Positive(t, ver)
}
