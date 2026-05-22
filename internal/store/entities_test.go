package store_test

import (
	"context"
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
		EntityType:        inventory.EntityTypePlace,
		ParentID:          nil,
		FullPathDisplay:   "Garage",
		FullPathCanonical: "garage",
		Depth:             0,
		Status:            inventory.EntityStatusOk,
		LastEventID:       1,
		UpdatedAt:         time.Now(),
	}

	err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
		return s.InsertEntityTx(ctx, tx, e)
	})
	require.NoError(t, err)

	got, err := s.GetEntity(ctx, "e1")
	require.NoError(t, err)
	assert.Equal(t, "Garage", got.DisplayName)
	assert.Equal(t, inventory.EntityTypePlace, got.EntityType)
}

func TestGetEntity_NotFound(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	_, err := s.GetEntity(ctx, "nonexistent")
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestGetEntitiesByCanonicalName(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	e := &inventory.Entity{
		EntityID: "e1", DisplayName: "Garage", CanonicalName: "garage",
		EntityType: inventory.EntityTypePlace, FullPathDisplay: "Garage",
		FullPathCanonical: "garage", Status: inventory.EntityStatusOk,
		LastEventID: 1, UpdatedAt: time.Now(),
	}
	err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
		return s.InsertEntityTx(ctx, tx, e)
	})
	require.NoError(t, err)

	results, err := s.GetEntitiesByCanonicalName(ctx, "garage")
	require.NoError(t, err)
	assert.Len(t, results, 1)
}
