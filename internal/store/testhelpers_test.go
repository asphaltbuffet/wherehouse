package store_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := store.Open(store.Config{Path: path, AutoMigrate: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}
