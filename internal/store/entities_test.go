package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

func TestInsertEntity(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	e := &inventory.Entity{
		EntityID:          "e1",
		DisplayName:       "Garage",
		CanonicalName:     "garage",
		ParentID:          nil,
		FullPathDisplay:   "Garage",
		FullPathCanonical: "garage",
		Depth:             0,
		Status:            inventory.EntityStatusOk,
		LastEventID:       1,
		UpdatedAt:         time.Now(),
	}

	err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
		return tx.InsertEntity(ctx, e)
	})
	require.NoError(t, err)

	got, err := s.Reader().GetLiveEntity(ctx, "e1")
	require.NoError(t, err)
	assert.Equal(t, "Garage", got.DisplayName)
	assert.False(t, got.Locked)
	assert.False(t, got.Discrete)
}

func TestGetEntity_NotFound(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	_, err := s.Reader().GetLiveEntity(ctx, "nonexistent")
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestGetEntitiesByCanonicalName(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	e := &inventory.Entity{
		EntityID: "e1", DisplayName: "Garage", CanonicalName: "garage",
		FullPathDisplay:   "Garage",
		FullPathCanonical: "garage", Status: inventory.EntityStatusOk,
		LastEventID: 1, UpdatedAt: time.Now(),
	}
	err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
		return tx.InsertEntity(ctx, e)
	})
	require.NoError(t, err)

	results, err := s.Reader().GetLiveEntitiesByCanonicalName(ctx, "garage")
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestTruncateEntities(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	insert := func(id, name string) {
		e := &inventory.Entity{
			EntityID:      id,
			DisplayName:   name,
			CanonicalName: strings.ToLower(name), FullPathDisplay: name,
			FullPathCanonical: strings.ToLower(name),
			Status:            inventory.EntityStatusOk,
			LastEventID:       1,
			UpdatedAt:         time.Now(),
		}
		err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
			return tx.InsertEntity(ctx, e)
		})
		require.NoError(t, err)
	}

	insert("e1", "Garage")
	insert("e2", "Attic")

	t.Run("committed truncate empties table", func(t *testing.T) {
		err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
			return tx.TruncateEntities(ctx)
		})
		require.NoError(t, err)

		entities, err := s.Reader().ListLiveEntities(ctx)
		require.NoError(t, err)
		assert.Empty(t, entities)
	})

	t.Run("rolled-back truncate restores rows", func(t *testing.T) {
		insert("e3", "Basement")

		errRollback := errors.New("rollback")
		err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
			require.NoError(t, tx.TruncateEntities(ctx))
			return errRollback
		})
		require.ErrorIs(t, err, errRollback)

		entities, err := s.Reader().ListLiveEntities(ctx)
		require.NoError(t, err)
		assert.Len(t, entities, 1)
	})
}
