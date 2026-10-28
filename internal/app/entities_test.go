package app_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

func openTestApp(t *testing.T) *app.App {
	t.Helper()
	s, err := store.Open(store.Config{
		Path:        filepath.Join(t.TempDir(), "test.db"),
		AutoMigrate: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return app.New(s)
}

func TestCreateEntity_Root(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	result, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage",
		ActorID:     "alice",
	})
	require.NoError(t, err)
	assert.Equal(t, "Garage", result.DisplayName)
	assert.Equal(t, "Garage", result.FullPathDisplay)
	assert.False(t, result.Locked)
	assert.False(t, result.Discrete)
}

func TestCreateEntity_Locked(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	result, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage",
		Locked:      true,
		ActorID:     "alice",
	})
	require.NoError(t, err)
	assert.True(t, result.Locked)
	assert.False(t, result.Discrete)
}

func TestCreateEntity_Discrete(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	result, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Box of Nails",
		Discrete:    true,
		ActorID:     "alice",
	})
	require.NoError(t, err)
	assert.False(t, result.Locked)
	assert.True(t, result.Discrete)
}

func TestCreateEntity_NestedPath(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage",
		ActorID:     "alice",
	})
	require.NoError(t, err)

	result, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Toolbox",
		ParentID:    apptesting.MustLookupID(t, a, "Garage"),
		ActorID:     "alice",
	})
	require.NoError(t, err)
	assert.Equal(t, "Garage:Toolbox", result.FullPathDisplay)
}

func TestCreateEntity_DiscreteParent_Error(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Box of Nails",
		Discrete:    true,
		ActorID:     "alice",
	})
	require.NoError(t, err)

	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Nail",
		ParentID:    apptesting.MustLookupID(t, a, "Box of Nails"),
		ActorID:     "alice",
	})
	require.ErrorIs(t, err, app.ErrInvalidDestination)
	assert.ErrorContains(t, err, "discrete")
}

func TestReparentEntity_Locked_Error(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	garage, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage",
		Locked:      true,
		ActorID:     "alice",
	})
	require.NoError(t, err)

	office, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Office",
		ActorID:     "alice",
	})
	require.NoError(t, err)

	_, err = a.ReparentEntity(ctx, app.ReparentEntityRequest{
		EntityID:    garage.EntityID,
		NewParentID: office.EntityID,
		ActorID:     "alice",
	})
	assert.ErrorContains(t, err, "locked")
}

func TestReparentEntity_IntoDiscrete_Error(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	box, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Box of Nails",
		Discrete:    true,
		ActorID:     "alice",
	})
	require.NoError(t, err)

	wrench, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Wrench",
		ActorID:     "alice",
	})
	require.NoError(t, err)

	_, err = a.ReparentEntity(ctx, app.ReparentEntityRequest{
		EntityID:    wrench.EntityID,
		NewParentID: box.EntityID,
		ActorID:     "alice",
	})
	assert.ErrorContains(t, err, "discrete")
}

func TestReparentEntity_LockedChildMovesWithParent(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage",
		ActorID:     "alice",
	})
	require.NoError(t, err)

	fileCabinet, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "File Cabinet",
		ParentID:    apptesting.MustLookupID(t, a, "Garage"),
		ActorID:     "alice",
	})
	require.NoError(t, err)

	// Top Drawer is locked — cannot be directly moved, but moves with parent.
	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Top Drawer",
		Locked:      true,
		ParentID:    apptesting.MustLookupID(t, a, "Garage:File Cabinet"),
		ActorID:     "alice",
	})
	require.NoError(t, err)

	office, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Office",
		ActorID:     "alice",
	})
	require.NoError(t, err)

	// Moving File Cabinet (not locked) to Office should also move Top Drawer.
	_, err = a.ReparentEntity(ctx, app.ReparentEntityRequest{
		EntityID:    fileCabinet.EntityID,
		NewParentID: office.EntityID,
		ActorID:     "alice",
	})
	require.NoError(t, err)

	// Top Drawer should now be under Office:File Cabinet.
	drawer, err := a.LookupEntityByPath(ctx, "Office:File Cabinet:Top Drawer")
	require.NoError(t, err)
	assert.Equal(t, "Office:File Cabinet:Top Drawer", drawer.FullPathDisplay)
	assert.True(t, drawer.Locked)
}

func TestLookupEntity_ByPath(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage",
		ActorID:     "alice",
	})
	require.NoError(t, err)

	result, err := a.LookupEntityByPath(ctx, "Garage")
	require.NoError(t, err)
	assert.Equal(t, "Garage", result.DisplayName)
}

func TestLookupEntityByPath_Disambiguation(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Shelf", ActorID: "alice",
	})
	require.NoError(t, err)

	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage", ActorID: "alice",
	})
	require.NoError(t, err)

	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Shelf", ParentID: apptesting.MustLookupID(t, a, "Garage"), ActorID: "alice",
	})
	require.NoError(t, err)

	result, err := a.LookupEntityByPath(ctx, "Garage:Shelf")
	require.NoError(t, err)
	assert.Equal(t, "Garage:Shelf", result.FullPathDisplay)

	result2, err := a.LookupEntityByPath(ctx, "Shelf")
	require.NoError(t, err)
	assert.Equal(t, "Shelf", result2.FullPathDisplay)
}

func TestRemoveEntity_ThenMutate_Error(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	garage, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage", ActorID: "alice",
	})
	require.NoError(t, err)

	err = a.RemoveEntity(ctx, app.RemoveEntityRequest{
		EntityID: garage.EntityID, ActorID: "alice",
	})
	require.NoError(t, err)

	_, err = a.RenameEntity(ctx, app.RenameEntityRequest{
		EntityID: garage.EntityID, NewName: "Workshop", ActorID: "alice",
	})
	assert.Error(t, err)
}

func TestListEntities(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	for _, name := range []string{"Garage", "Basement", "Kitchen"} {
		_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: name,
			ActorID:     "alice",
		})
		require.NoError(t, err)
	}

	results, err := a.ListEntities(ctx)
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestApp_GetChildren(t *testing.T) {
	t.Run("two active children", func(t *testing.T) {
		ctx := context.Background()
		a := openTestApp(t)

		parent, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: "Parent",
			ActorID:     "alice",
		})
		require.NoError(t, err)

		child1, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: "Child1",
			ParentID:    apptesting.MustLookupID(t, a, "Parent"),
			ActorID:     "alice",
		})
		require.NoError(t, err)

		child2, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: "Child2",
			ParentID:    apptesting.MustLookupID(t, a, "Parent"),
			ActorID:     "alice",
		})
		require.NoError(t, err)

		results, err := a.GetChildren(ctx, parent.EntityID)
		require.NoError(t, err)
		require.Len(t, results, 2)

		ids := []string{results[0].EntityID, results[1].EntityID}
		assert.ElementsMatch(t, []string{child1.EntityID, child2.EntityID}, ids)
	})

	t.Run("removed child excluded", func(t *testing.T) {
		ctx := context.Background()
		a := openTestApp(t)

		parent, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: "Parent",
			ActorID:     "alice",
		})
		require.NoError(t, err)

		active, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: "Active",
			ParentID:    apptesting.MustLookupID(t, a, "Parent"),
			ActorID:     "alice",
		})
		require.NoError(t, err)

		removedEntity, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: "Removed",
			ParentID:    apptesting.MustLookupID(t, a, "Parent"),
			ActorID:     "alice",
		})
		require.NoError(t, err)

		err = a.RemoveEntity(ctx, app.RemoveEntityRequest{
			EntityID: removedEntity.EntityID,
			ActorID:  "alice",
		})
		require.NoError(t, err)

		results, err := a.GetChildren(ctx, parent.EntityID)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, active.EntityID, results[0].EntityID)
	})

	t.Run("no children", func(t *testing.T) {
		ctx := context.Background()
		a := openTestApp(t)

		parent, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: "Lonely",
			ActorID:     "alice",
		})
		require.NoError(t, err)

		results, err := a.GetChildren(ctx, parent.EntityID)
		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("HasChildren true when child has grandchildren", func(t *testing.T) {
		ctx := context.Background()
		a := openTestApp(t)

		gp, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: "GP",
			ActorID:     "alice",
		})
		require.NoError(t, err)

		_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: "Mid",
			ParentID:    apptesting.MustLookupID(t, a, "GP"),
			ActorID:     "alice",
		})
		require.NoError(t, err)

		_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: "Leaf",
			ParentID:    apptesting.MustLookupID(t, a, "GP:Mid"),
			ActorID:     "alice",
		})
		require.NoError(t, err)

		results, err := a.GetChildren(ctx, gp.EntityID)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.True(t, results[0].HasChildren, "Mid should report HasChildren=true")
	})

	t.Run("HasChildren false when only grandchildren are removed", func(t *testing.T) {
		ctx := context.Background()
		a := openTestApp(t)

		gp, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: "GP",
			ActorID:     "alice",
		})
		require.NoError(t, err)

		_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: "Mid",
			ParentID:    apptesting.MustLookupID(t, a, "GP"),
			ActorID:     "alice",
		})
		require.NoError(t, err)

		leaf, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: "Leaf",
			ParentID:    apptesting.MustLookupID(t, a, "GP:Mid"),
			ActorID:     "alice",
		})
		require.NoError(t, err)

		err = a.RemoveEntity(ctx, app.RemoveEntityRequest{
			EntityID: leaf.EntityID,
			ActorID:  "alice",
		})
		require.NoError(t, err)

		results, err := a.GetChildren(ctx, gp.EntityID)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.False(t, results[0].HasChildren, "Mid should report HasChildren=false when its only child is removed")
	})
}

func TestApp_HasLiveChildren(t *testing.T) {
	ctx := context.Background()

	t.Run("live child", func(t *testing.T) {
		a := openTestApp(t)
		parent, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Parent", ActorID: "alice"})
		require.NoError(t, err)
		_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: "Child", ParentID: apptesting.MustLookupID(t, a, "Parent"), ActorID: "alice",
		})
		require.NoError(t, err)

		has, err := a.HasLiveChildren(ctx, parent.EntityID)
		require.NoError(t, err)
		assert.True(t, has)
	})

	t.Run("only child removed", func(t *testing.T) {
		a := openTestApp(t)
		parent, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Parent", ActorID: "alice"})
		require.NoError(t, err)
		child, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: "Child", ParentID: apptesting.MustLookupID(t, a, "Parent"), ActorID: "alice",
		})
		require.NoError(t, err)
		require.NoError(t, a.RemoveEntity(ctx, app.RemoveEntityRequest{EntityID: child.EntityID, ActorID: "alice"}))

		has, err := a.HasLiveChildren(ctx, parent.EntityID)
		require.NoError(t, err)
		assert.False(t, has)
	})

	t.Run("leaf", func(t *testing.T) {
		a := openTestApp(t)
		leaf, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Leaf", ActorID: "alice"})
		require.NoError(t, err)

		has, err := a.HasLiveChildren(ctx, leaf.EntityID)
		require.NoError(t, err)
		assert.False(t, has)
	})
}

func TestLookupEntityByPath_Found(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	created, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage",
		ActorID:     "alice",
	})
	require.NoError(t, err)

	result, err := a.LookupEntityByPath(ctx, "Garage")
	require.NoError(t, err)
	assert.Equal(t, created.EntityID, result.EntityID)
	assert.Equal(t, "Garage", result.DisplayName)
}

func TestLookupEntityByPath_NotFound(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	_, err := a.LookupEntityByPath(ctx, "NoSuchThing")
	assert.ErrorIs(t, err, app.ErrNotFound)
}

func TestRenameEntity_ByID(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	created, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Garage", ActorID: "alice"})
	require.NoError(t, err)

	result, err := a.RenameEntity(ctx, app.RenameEntityRequest{
		EntityID: created.EntityID,
		NewName:  "Workshop",
		ActorID:  "alice",
	})
	require.NoError(t, err)
	assert.Equal(t, "Workshop", result.DisplayName)
	assert.Equal(t, created.EntityID, result.EntityID)
}

func TestRenameEntity_UnknownID_Error(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	_, err := a.RenameEntity(ctx, app.RenameEntityRequest{
		EntityID: "nonexistent",
		NewName:  "Whatever",
		ActorID:  "alice",
	})
	assert.ErrorIs(t, err, app.ErrNotFound)
}

func TestReparentEntity_SourceStatusPreconditions(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, a *app.App, id string)
		wantErr string
	}{
		{name: "ok moves"},
		{
			name: "missing rejected",
			setup: func(t *testing.T, a *app.App, id string) {
				t.Helper()
				_, err := a.MarkLost(t.Context(), []app.ChangeStatusRequest{{EntityID: id, ActorID: "alice"}})
				require.NoError(t, err)
			},
			wantErr: "mark it found first",
		},
		{
			name: "loaned rejected",
			setup: func(t *testing.T, a *app.App, id string) {
				t.Helper()
				_, err := a.MarkLoaned(t.Context(), []app.ChangeStatusRequest{
					{EntityID: id, StatusContext: "Bob", ActorID: "alice"},
				})
				require.NoError(t, err)
			},
			wantErr: "return it first",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := openTestApp(t)
			ctx := t.Context()
			wrench, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Wrench", ActorID: "alice"})
			require.NoError(t, err)
			office, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Office", ActorID: "alice"})
			require.NoError(t, err)
			if tt.setup != nil {
				tt.setup(t, a, wrench.EntityID)
			}

			_, err = a.ReparentEntity(ctx, app.ReparentEntityRequest{
				EntityID: wrench.EntityID, NewParentID: office.EntityID, ActorID: "alice",
			})

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestReparentEntity_BorrowedMoves(t *testing.T) {
	a := openTestApp(t)
	ctx := t.Context()
	borrowed, err := a.BorrowEntities(ctx, []app.BorrowEntityRequest{
		{DisplayName: "Drill", StatusContext: "Alice", ActorID: "alice"},
	})
	require.NoError(t, err)
	office, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Office", ActorID: "alice"})
	require.NoError(t, err)

	moved, err := a.ReparentEntity(ctx, app.ReparentEntityRequest{
		EntityID: borrowed[0].EntityID, NewParentID: office.EntityID, ActorID: "alice",
	})

	require.NoError(t, err)
	assert.Equal(t, "Office:Drill", moved.FullPathDisplay)
}

func TestReparentEntity_InvalidDestinations(t *testing.T) {
	a := openTestApp(t)
	ctx := t.Context()
	garage, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Garage", ActorID: "alice"})
	require.NoError(t, err)
	toolbox, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Toolbox", ParentID: apptesting.MustLookupID(t, a, "Garage"), ActorID: "alice",
	})
	require.NoError(t, err)
	drawer, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Drawer", ParentID: apptesting.MustLookupID(t, a, "Garage:Toolbox"), ActorID: "alice",
	})
	require.NoError(t, err)
	box, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Box of Nails", Discrete: true, ActorID: "alice",
	})
	require.NoError(t, err)

	tests := []struct {
		name        string
		entityID    string
		newParentID string
	}{
		{"into itself", garage.EntityID, garage.EntityID},
		{"into child", garage.EntityID, toolbox.EntityID},
		{"into grandchild", garage.EntityID, drawer.EntityID},
		{"into current parent", toolbox.EntityID, garage.EntityID},
		{"root entity to root", garage.EntityID, ""},
		{"into discrete", toolbox.EntityID, box.EntityID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, moveErr := a.ReparentEntity(ctx, app.ReparentEntityRequest{
				EntityID: tt.entityID, NewParentID: tt.newParentID, ActorID: "alice",
			})
			require.ErrorIs(t, moveErr, app.ErrInvalidDestination)
		})
	}

	got, err := a.GetEntityByID(ctx, garage.EntityID)
	require.NoError(t, err)
	assert.Equal(t, "Garage", got.FullPathDisplay, "rejected moves must not change the hierarchy")
}

func TestReparentEntity_ToRoot(t *testing.T) {
	a := openTestApp(t)
	ctx := t.Context()
	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Garage", ActorID: "alice"})
	require.NoError(t, err)
	toolbox, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Toolbox", ParentID: apptesting.MustLookupID(t, a, "Garage"), ActorID: "alice",
	})
	require.NoError(t, err)

	moved, err := a.ReparentEntity(ctx, app.ReparentEntityRequest{EntityID: toolbox.EntityID, ActorID: "alice"})

	require.NoError(t, err)
	assert.Equal(t, "Toolbox", moved.FullPathDisplay)
	assert.Empty(t, moved.ParentID)
}

// Names are not unique, so the cycle check must not use path prefixes.
func TestReparentEntity_SameNameSiblingIsNotDescendant(t *testing.T) {
	a := openTestApp(t)
	ctx := t.Context()
	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Shelf", ActorID: "alice"})
	require.NoError(t, err)
	box1, err := a.CreateEntity(
		ctx,
		app.CreateEntityRequest{DisplayName: "Box", ParentID: apptesting.MustLookupID(t, a, "Shelf"), ActorID: "alice"},
	)
	require.NoError(t, err)
	box2, err := a.CreateEntity(
		ctx,
		app.CreateEntityRequest{DisplayName: "Box", ParentID: apptesting.MustLookupID(t, a, "Shelf"), ActorID: "alice"},
	)
	require.NoError(t, err)
	// "Shelf:Box" is ambiguous as a path, so place Tray under box2 by ID.
	tray, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Tray", ActorID: "alice"})
	require.NoError(t, err)
	_, err = a.ReparentEntity(ctx, app.ReparentEntityRequest{
		EntityID: tray.EntityID, NewParentID: box2.EntityID, ActorID: "alice",
	})
	require.NoError(t, err)

	moved, err := a.ReparentEntity(ctx, app.ReparentEntityRequest{
		EntityID: box1.EntityID, NewParentID: tray.EntityID, ActorID: "alice",
	})

	require.NoError(t, err)
	assert.Equal(t, "Shelf:Box:Tray:Box", moved.FullPathDisplay)
}

// A live entity under a removed ancestor (#287) is still a descendant.
func TestReparentEntity_DescendantUnderRemovedAncestor(t *testing.T) {
	a := openTestApp(t)
	ctx := t.Context()
	garage, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Garage", ActorID: "alice"})
	require.NoError(t, err)
	box, err := a.CreateEntity(
		ctx,
		app.CreateEntityRequest{
			DisplayName: "Box",
			ParentID:    apptesting.MustLookupID(t, a, "Garage"),
			ActorID:     "alice",
		},
	)
	require.NoError(t, err)
	bit, err := a.CreateEntity(
		ctx,
		app.CreateEntityRequest{
			DisplayName: "Bit",
			ParentID:    apptesting.MustLookupID(t, a, "Garage:Box"),
			ActorID:     "alice",
		},
	)
	require.NoError(t, err)
	apptesting.RemoveLeavingLiveChildren(t, a, box.EntityID) // pre-#287 data: live child under removed parent

	_, err = a.ReparentEntity(ctx, app.ReparentEntityRequest{
		EntityID: garage.EntityID, NewParentID: bit.EntityID, ActorID: "alice",
	})

	require.ErrorIs(t, err, app.ErrInvalidDestination)
}

// A projection already damaged by the pre-ADR-0034 cycle bug must not hang a move.
func TestReparentEntity_ExistingCycleDoesNotHang(t *testing.T) {
	a, s := openTestAppWithStore(t)
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	looped, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Looped", ActorID: "alice"})
	require.NoError(t, err)
	wrench, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Wrench", ActorID: "alice"})
	require.NoError(t, err)
	_, err = s.DB().ExecContext(ctx,
		`UPDATE entities_current SET parent_id = entity_id WHERE entity_id = ?`, looped.EntityID)
	require.NoError(t, err)

	_, err = a.ReparentEntity(ctx, app.ReparentEntityRequest{
		EntityID: wrench.EntityID, NewParentID: looped.EntityID, ActorID: "alice",
	})

	assert.ErrorContains(t, err, "loops")
}

func TestRemoveEntity_LiveChildrenRejected(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	box, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Box", ActorID: "alice"})
	require.NoError(t, err)
	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Nail", ParentID: apptesting.MustLookupID(t, a, "Box"), ActorID: "alice",
	})
	require.NoError(t, err)

	err = a.RemoveEntity(ctx, app.RemoveEntityRequest{EntityID: box.EntityID, ActorID: "alice"})
	require.ErrorIs(t, err, app.ErrHasChildren)
	assert.Contains(t, err.Error(), `cannot remove "Box" (1 inside; remove or move them first)`)

	got, err := a.GetEntityByID(ctx, box.EntityID)
	require.NoError(t, err)
	assert.Equal(t, inventory.EntityStatusOk, got.Status, "a rejected remove must not write an event")
}

func TestRemoveEntity_RemovedChildrenDoNotBlock(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	box, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Box", ActorID: "alice"})
	require.NoError(t, err)
	nail, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Nail", ParentID: apptesting.MustLookupID(t, a, "Box"), ActorID: "alice",
	})
	require.NoError(t, err)

	require.NoError(t, a.RemoveEntity(ctx, app.RemoveEntityRequest{EntityID: nail.EntityID, ActorID: "alice"}))
	require.NoError(t, a.RemoveEntity(ctx, app.RemoveEntityRequest{EntityID: box.EntityID, ActorID: "alice"}))
}

func TestRemoveEntity_AlreadyRemovedNotFound(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	box, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Box", ActorID: "alice"})
	require.NoError(t, err)
	require.NoError(t, a.RemoveEntity(ctx, app.RemoveEntityRequest{EntityID: box.EntityID, ActorID: "alice"}))

	err = a.RemoveEntity(ctx, app.RemoveEntityRequest{EntityID: box.EntityID, ActorID: "alice"})
	require.ErrorIs(t, err, app.ErrNotFound, "web relies on this to detect a concurrent remove")
}

// #288: names are not unique, so the parent is the one the caller picked by ID.
func TestCreateEntity_UnderSecondSameNamedSibling(t *testing.T) {
	a := openTestApp(t)
	ctx := t.Context()
	garage, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Garage", ActorID: "alice"})
	require.NoError(t, err)
	_, err = a.CreateEntity(
		ctx,
		app.CreateEntityRequest{DisplayName: "Bin", ParentID: garage.EntityID, ActorID: "alice"},
	)
	require.NoError(t, err)
	bin2, err := a.CreateEntity(
		ctx,
		app.CreateEntityRequest{DisplayName: "Bin", ParentID: garage.EntityID, ActorID: "alice"},
	)
	require.NoError(t, err)

	got, err := a.CreateEntity(
		ctx,
		app.CreateEntityRequest{DisplayName: "Nail", ParentID: bin2.EntityID, ActorID: "alice"},
	)

	require.NoError(t, err)
	assert.Equal(t, bin2.EntityID, got.ParentID)
}

// #288: a discrete first same-named sibling must not block the eligible second.
func TestCreateEntity_DiscreteFirstSiblingDoesNotBlockSecond(t *testing.T) {
	a := openTestApp(t)
	ctx := t.Context()
	garage, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Garage", ActorID: "alice"})
	require.NoError(t, err)
	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Bin", Discrete: true, ParentID: garage.EntityID, ActorID: "alice",
	})
	require.NoError(t, err)
	bin2, err := a.CreateEntity(
		ctx,
		app.CreateEntityRequest{DisplayName: "Bin", ParentID: garage.EntityID, ActorID: "alice"},
	)
	require.NoError(t, err)

	got, err := a.CreateEntity(
		ctx,
		app.CreateEntityRequest{DisplayName: "Nail", ParentID: bin2.EntityID, ActorID: "alice"},
	)

	require.NoError(t, err)
	assert.Equal(t, bin2.EntityID, got.ParentID)
}

// Review Focus #3: a parent removed after the caller picked it.
func TestCreateEntity_RemovedParent_NotFound(t *testing.T) {
	a := openTestApp(t)
	ctx := t.Context()
	shelf, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Shelf", ActorID: "alice"})
	require.NoError(t, err)
	require.NoError(t, a.RemoveEntity(ctx, app.RemoveEntityRequest{EntityID: shelf.EntityID, ActorID: "alice"}))

	_, err = a.CreateEntity(
		ctx,
		app.CreateEntityRequest{DisplayName: "Nail", ParentID: shelf.EntityID, ActorID: "alice"},
	)

	require.ErrorIs(t, err, app.ErrNotFound)
}

func TestCreateEntity_UnknownParent_NotFound(t *testing.T) {
	a := openTestApp(t)

	_, err := a.CreateEntity(
		t.Context(),
		app.CreateEntityRequest{DisplayName: "Nail", ParentID: "nosuchid00", ActorID: "alice"},
	)

	require.ErrorIs(t, err, app.ErrNotFound)
}
