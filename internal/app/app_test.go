package app_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

func TestClose_ReleasesStore(t *testing.T) {
	s, err := store.Open(store.Config{Path: filepath.Join(t.TempDir(), "test.db"), AutoMigrate: true})
	require.NoError(t, err)
	a := app.New(s)

	require.NoError(t, a.Close())

	_, err = a.HasEvents(t.Context())
	assert.Error(t, err, "queries after Close must fail")
}
