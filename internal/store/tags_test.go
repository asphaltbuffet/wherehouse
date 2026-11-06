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
		return tx.InsertEntity(ctx, e)
	})
	require.NoError(t, err)
	return e.EntityID
}

func TestInsertTag(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	entityID := seedTagEntity(t, s)

	err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
		return tx.InsertTag(ctx, entityID, "tool")
	})
	require.NoError(t, err)

	tags, err := s.Reader().GetTagsByEntity(ctx, entityID)
	require.NoError(t, err)
	assert.Equal(t, []string{"tool"}, tags)
}

func TestInsertTag_Duplicate(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	entityID := seedTagEntity(t, s)

	insert := func() error {
		return s.ExecInTransaction(ctx, func(tx store.Tx) error {
			return tx.InsertTag(ctx, entityID, "tool")
		})
	}
	require.NoError(t, insert())
	require.NoError(t, insert(), "duplicate insert must be a no-op")

	tags, err := s.Reader().GetTagsByEntity(ctx, entityID)
	require.NoError(t, err)
	assert.Equal(t, []string{"tool"}, tags)
}

func TestDeleteTag(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	entityID := seedTagEntity(t, s)

	err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
		return tx.InsertTag(ctx, entityID, "tool")
	})
	require.NoError(t, err)

	err = s.ExecInTransaction(ctx, func(tx store.Tx) error {
		return tx.DeleteTag(ctx, entityID, "tool")
	})
	require.NoError(t, err)

	tags, err := s.Reader().GetTagsByEntity(ctx, entityID)
	require.NoError(t, err)
	assert.Empty(t, tags)
}

func TestDeleteTag_Missing(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	entityID := seedTagEntity(t, s)

	err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
		return tx.DeleteTag(ctx, entityID, "nonexistent")
	})
	require.NoError(t, err, "deleting a missing tag must be a no-op")
}

func TestGetTagsByEntity_Sorted(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	entityID := seedTagEntity(t, s)

	for _, tag := range []string{"tool", "hand_tool", "screwdriver"} {
		err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
			return tx.InsertTag(ctx, entityID, tag)
		})
		require.NoError(t, err)
	}

	tags, err := s.Reader().GetTagsByEntity(ctx, entityID)
	require.NoError(t, err)
	assert.Equal(t, []string{"hand_tool", "screwdriver", "tool"}, tags)
}

func TestGetTagsByEntity_RetainedAfterRemoval(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	entityID := seedTagEntity(t, s)

	err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
		return tx.InsertTag(ctx, entityID, "tool")
	})
	require.NoError(t, err)

	// Soft-delete the entity by updating its status to removed.
	err = s.ExecInTransaction(ctx, func(tx store.Tx) error {
		e, getErr := tx.GetEntityAnyStatus(ctx, entityID)
		if getErr != nil {
			return getErr
		}
		e.Status = inventory.EntityStatusRemoved
		return tx.UpdateEntity(ctx, e)
	})
	require.NoError(t, err)

	tags, err := s.Reader().GetTagsByEntity(ctx, entityID)
	require.NoError(t, err)
	assert.Equal(t, []string{"tool"}, tags, "tags must be retained after entity removal")
}

func TestTruncateTags(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	entityID := seedTagEntity(t, s)

	err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
		return tx.InsertTag(ctx, entityID, "tool")
	})
	require.NoError(t, err)

	err = s.ExecInTransaction(ctx, func(tx store.Tx) error {
		return tx.TruncateTags(ctx)
	})
	require.NoError(t, err)

	tags, err := s.Reader().GetTagsByEntity(ctx, entityID)
	require.NoError(t, err)
	assert.Empty(t, tags)
}

func TestGetDistinctTags(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	entityID := seedTagEntity(t, s)

	tags, err := s.Reader().GetDistinctTags(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{}, tags, "empty table yields an empty, non-nil slice")

	other := &inventory.Entity{
		EntityID: "e-tag-2", DisplayName: "Saw", CanonicalName: "saw",
		FullPathDisplay: "Saw", FullPathCanonical: "saw",
		Status: inventory.EntityStatusRemoved, LastEventID: 2, UpdatedAt: time.Now(),
	}
	err = s.ExecInTransaction(ctx, func(tx store.Tx) error {
		if insErr := tx.InsertEntity(ctx, other); insErr != nil {
			return insErr
		}
		for _, row := range [][2]string{
			{entityID, "tool"}, {entityID, "garage"}, {other.EntityID, "tool"}, {other.EntityID, "retired"},
		} {
			if insErr := tx.InsertTag(ctx, row[0], row[1]); insErr != nil {
				return insErr
			}
		}
		return nil
	})
	require.NoError(t, err)

	tags, err = s.Reader().GetDistinctTags(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"garage", "retired", "tool"}, tags,
		"distinct, sorted, and includes tags on removed entities")
}

func TestHasTag(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	entityID := seedTagEntity(t, s)

	err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
		if insErr := tx.InsertTag(ctx, entityID, "tool"); insErr != nil {
			return insErr
		}
		has, hasErr := tx.HasTag(ctx, entityID, "tool")
		require.NoError(t, hasErr)
		assert.True(t, has)
		has, hasErr = tx.HasTag(ctx, entityID, "")
		require.NoError(t, hasErr)
		assert.False(t, has)
		return nil
	})
	require.NoError(t, err)
}
