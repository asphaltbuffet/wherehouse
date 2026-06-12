package store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

func TestMetadataRoundtrip(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	require.NoError(t, s.SetMetadata(ctx, "schema_version", "1"))

	val, err := s.GetMetadata(ctx, "schema_version")
	require.NoError(t, err)
	assert.Equal(t, "1", val)
}

func TestStorePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "myinventory.db")
	s, err := store.Open(store.Config{Path: path, AutoMigrate: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	assert.Equal(t, path, s.Path())
}

func TestCountEntitiesByStatus_Empty(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	counts, err := s.CountEntitiesByStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, counts["ok"])
	assert.Equal(t, 0, counts["missing"])
	assert.Equal(t, 0, counts["borrowed"])
	assert.Equal(t, 0, counts["loaned"])
	assert.Equal(t, 0, counts["removed"])
}

func TestCountEntitiesByStatus_WithEntities(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	insertEntity := func(id, status string) {
		e := &inventory.Entity{
			EntityID:          id,
			DisplayName:       id,
			CanonicalName:     id,
			FullPathDisplay:   id,
			FullPathCanonical: id,
			Depth:             0,
			Status:            inventory.EntityStatusOk,
			LastEventID:       1,
			UpdatedAt:         time.Now(),
		}
		switch status {
		case "missing":
			e.Status = inventory.EntityStatusMissing
		case "removed":
			e.Status = inventory.EntityStatusRemoved
		}
		err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
			return s.InsertEntityTx(ctx, tx, e)
		})
		require.NoError(t, err)
	}

	insertEntity("e1", "ok")
	insertEntity("e2", "ok")
	insertEntity("e3", "missing")
	insertEntity("e4", "removed")

	counts, err := s.CountEntitiesByStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, counts["ok"])
	assert.Equal(t, 1, counts["missing"])
	assert.Equal(t, 1, counts["removed"])
	assert.Equal(t, 0, counts["borrowed"])
	assert.Equal(t, 0, counts["loaned"])
}

func TestGetMetadata_Missing(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	_, err := s.GetMetadata(ctx, "nonexistent")
	assert.ErrorIs(t, err, store.ErrNotFound)
}
