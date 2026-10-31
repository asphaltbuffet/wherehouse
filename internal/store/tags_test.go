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

func seedTagEntity(t *testing.T, s *store.Store) string {
	t.Helper()
	ctx := context.Background()
	e := &inventory.Entity{
		EntityID:      "e-tag-1",
		DisplayName:   "Wrench",
		CanonicalName: "wrench", FullPathDisplay: "Wrench",
		FullPathCanonical: "wrench",
		Depth:             0,
		Status:            inventory.EntityStatusOk,
		LastEventID:       1,
		UpdatedAt:         time.Now(),
	}
	err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
		return s.InsertEntityTx(ctx, tx, e)
	})
	require.NoError(t, err)
	return e.EntityID
}

func TestInsertTagTx(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	entityID := seedTagEntity(t, s)

	err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
		return s.InsertTagTx(ctx, tx, entityID, "tool")
	})
	require.NoError(t, err)

	tags, err := s.GetTagsByEntity(ctx, entityID)
	require.NoError(t, err)
	assert.Equal(t, []string{"tool"}, tags)
}

func TestInsertTagTx_Duplicate(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	entityID := seedTagEntity(t, s)

	insert := func() error {
		return s.ExecInTransaction(ctx, func(tx store.Tx) error {
			return s.InsertTagTx(ctx, tx, entityID, "tool")
		})
	}
	require.NoError(t, insert())
	require.NoError(t, insert(), "duplicate insert must be a no-op")

	tags, err := s.GetTagsByEntity(ctx, entityID)
	require.NoError(t, err)
	assert.Equal(t, []string{"tool"}, tags)
}

func TestDeleteTagTx(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	entityID := seedTagEntity(t, s)

	err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
		return s.InsertTagTx(ctx, tx, entityID, "tool")
	})
	require.NoError(t, err)

	err = s.ExecInTransaction(ctx, func(tx store.Tx) error {
		return s.DeleteTagTx(ctx, tx, entityID, "tool")
	})
	require.NoError(t, err)

	tags, err := s.GetTagsByEntity(ctx, entityID)
	require.NoError(t, err)
	assert.Empty(t, tags)
}

func TestDeleteTagTx_Missing(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	entityID := seedTagEntity(t, s)

	err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
		return s.DeleteTagTx(ctx, tx, entityID, "nonexistent")
	})
	require.NoError(t, err, "deleting a missing tag must be a no-op")
}

func TestGetTagsByEntity_Sorted(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	entityID := seedTagEntity(t, s)

	for _, tag := range []string{"tool", "hand_tool", "screwdriver"} {
		err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
			return s.InsertTagTx(ctx, tx, entityID, tag)
		})
		require.NoError(t, err)
	}

	tags, err := s.GetTagsByEntity(ctx, entityID)
	require.NoError(t, err)
	assert.Equal(t, []string{"hand_tool", "screwdriver", "tool"}, tags)
}

func TestGetTagsByEntity_RetainedAfterRemoval(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	entityID := seedTagEntity(t, s)

	err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
		return s.InsertTagTx(ctx, tx, entityID, "tool")
	})
	require.NoError(t, err)

	// Soft-delete the entity by updating its status to removed.
	err = s.ExecInTransaction(ctx, func(tx store.Tx) error {
		e, getErr := s.GetEntityTx(ctx, tx, entityID)
		if getErr != nil {
			return getErr
		}
		e.Status = inventory.EntityStatusRemoved
		return s.UpdateEntityTx(ctx, tx, e)
	})
	require.NoError(t, err)

	tags, err := s.GetTagsByEntity(ctx, entityID)
	require.NoError(t, err)
	assert.Equal(t, []string{"tool"}, tags, "tags must be retained after entity removal")
}

func TestTruncateTagsTx(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	entityID := seedTagEntity(t, s)

	err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
		return s.InsertTagTx(ctx, tx, entityID, "tool")
	})
	require.NoError(t, err)

	err = s.ExecInTransaction(ctx, func(tx store.Tx) error {
		return s.TruncateTagsTx(ctx, tx)
	})
	require.NoError(t, err)

	tags, err := s.GetTagsByEntity(ctx, entityID)
	require.NoError(t, err)
	assert.Empty(t, tags)
}

func TestGetDistinctTags(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	entityID := seedTagEntity(t, s)

	tags, err := s.GetDistinctTags(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{}, tags, "empty table yields an empty, non-nil slice")

	other := &inventory.Entity{
		EntityID: "e-tag-2", DisplayName: "Saw", CanonicalName: "saw",
		FullPathDisplay: "Saw", FullPathCanonical: "saw",
		Status: inventory.EntityStatusRemoved, LastEventID: 2, UpdatedAt: time.Now(),
	}
	err = s.ExecInTransaction(ctx, func(tx store.Tx) error {
		if insErr := s.InsertEntityTx(ctx, tx, other); insErr != nil {
			return insErr
		}
		for _, row := range [][2]string{
			{entityID, "tool"}, {entityID, "garage"}, {other.EntityID, "tool"}, {other.EntityID, "retired"},
		} {
			if insErr := s.InsertTagTx(ctx, tx, row[0], row[1]); insErr != nil {
				return insErr
			}
		}
		return nil
	})
	require.NoError(t, err)

	tags, err = s.GetDistinctTags(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"garage", "retired", "tool"}, tags,
		"distinct, sorted, and includes tags on removed entities")
}

func TestHasTagTx(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	entityID := seedTagEntity(t, s)

	err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
		if insErr := s.InsertTagTx(ctx, tx, entityID, "tool"); insErr != nil {
			return insErr
		}
		has, hasErr := s.HasTagTx(ctx, tx, entityID, "tool")
		require.NoError(t, hasErr)
		assert.True(t, has)
		has, hasErr = s.HasTagTx(ctx, tx, entityID, "")
		require.NoError(t, hasErr)
		assert.False(t, has)
		return nil
	})
	require.NoError(t, err)
}
