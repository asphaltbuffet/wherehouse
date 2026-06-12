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

func TestGetRootEntities(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	garage := &inventory.Entity{
		EntityID: "e1", DisplayName: "Garage", CanonicalName: "garage",
		ParentID: nil, FullPathDisplay: "Garage", FullPathCanonical: "garage",
		Depth: 0, Status: inventory.EntityStatusOk, LastEventID: 1, UpdatedAt: time.Now(),
	}
	toolbox := &inventory.Entity{
		EntityID: "e2", DisplayName: "Toolbox", CanonicalName: "toolbox",
		ParentID: &garage.EntityID, FullPathDisplay: "Garage:Toolbox", FullPathCanonical: "garage:toolbox",
		Depth: 1, Status: inventory.EntityStatusOk, LastEventID: 2, UpdatedAt: time.Now(),
	}
	shelf := &inventory.Entity{
		EntityID: "e3", DisplayName: "Shelf", CanonicalName: "shelf",
		ParentID: nil, FullPathDisplay: "Shelf", FullPathCanonical: "shelf",
		Depth: 0, Status: inventory.EntityStatusOk, LastEventID: 3, UpdatedAt: time.Now(),
	}
	removed := &inventory.Entity{
		EntityID: "e4", DisplayName: "Old Box", CanonicalName: "old_box",
		ParentID: nil, FullPathDisplay: "Old Box", FullPathCanonical: "old_box",
		Depth: 0, Status: inventory.EntityStatusRemoved, LastEventID: 4, UpdatedAt: time.Now(),
	}

	for _, e := range []*inventory.Entity{garage, toolbox, shelf, removed} {
		err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
			return s.InsertEntityTx(ctx, tx, e)
		})
		require.NoError(t, err)
	}

	t.Run("returns only depth-0 non-removed entities", func(t *testing.T) {
		rows, err := s.GetRootEntities(ctx)
		require.NoError(t, err)

		require.Len(t, rows, 2)
		names := []string{rows[0].Entity.DisplayName, rows[1].Entity.DisplayName}
		assert.Contains(t, names, "Garage")
		assert.Contains(t, names, "Shelf")
	})

	t.Run("marks has_children correctly", func(t *testing.T) {
		rows, err := s.GetRootEntities(ctx)
		require.NoError(t, err)

		byName := map[string]store.ChildRow{}
		for _, r := range rows {
			byName[r.Entity.DisplayName] = r
		}

		assert.True(t, byName["Garage"].HasChildren, "Garage has Toolbox child")
		assert.False(t, byName["Shelf"].HasChildren, "Shelf has no children")
	})
}
