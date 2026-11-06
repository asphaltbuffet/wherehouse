package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/store"
)

func TestOpen_EmptyPath(t *testing.T) {
	_, err := store.Open(store.Config{})
	assert.ErrorIs(t, err, store.ErrDatabasePathRequired)
}

func TestOpen_ValidPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := store.Open(store.Config{Path: path, AutoMigrate: true})
	require.NoError(t, err)
	require.NotNil(t, s)
	assert.NoError(t, s.Close())
}

func TestExecInTransaction_Commit(t *testing.T) {
	s := openTestStore(t)

	err := s.ExecInTransaction(context.Background(), func(_ store.Tx) error {
		return nil
	})
	assert.NoError(t, err)
}

func TestExecInTransaction_Rollback(t *testing.T) {
	s := openTestStore(t)

	sentinelErr := errors.New("rollback me")
	err := s.ExecInTransaction(context.Background(), func(_ store.Tx) error {
		return sentinelErr
	})
	assert.ErrorIs(t, err, sentinelErr)
}

// A Tx must only come from ExecInTransaction (ADR 0043): its embedded Reader
// is unexported, so no package outside store can build one over the pool.
func TestTx_EmbeddedReaderIsUnexported(t *testing.T) {
	for f := range reflect.TypeFor[store.Tx]().Fields() {
		assert.False(t, f.IsExported(), "field %s lets callers construct a Tx", f.Name)
	}
}
