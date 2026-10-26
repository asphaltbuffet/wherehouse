package apptesting

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// OpenApp opens an *app.App backed by a SQLite file in t.TempDir() (WAL mode,
// so concurrent connections behave as in production).
// The store is closed automatically when t completes.
func OpenApp(t *testing.T) *app.App {
	t.Helper()
	s, err := store.Open(store.Config{
		Path:        filepath.Join(t.TempDir(), "test.db"),
		AutoMigrate: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return app.New(s)
}
