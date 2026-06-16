package app_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
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
		ParentPath:  "Garage",
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
		ParentPath:  "Box of Nails",
		ActorID:     "alice",
	})
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
		ParentPath:  "Garage",
		ActorID:     "alice",
	})
	require.NoError(t, err)

	// Top Drawer is locked — cannot be directly moved, but moves with parent.
	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Top Drawer",
		Locked:      true,
		ParentPath:  "Garage:File Cabinet",
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
		DisplayName: "Shelf", ParentPath: "Garage", ActorID: "alice",
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
			ParentPath:  "Parent",
			ActorID:     "alice",
		})
		require.NoError(t, err)

		child2, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: "Child2",
			ParentPath:  "Parent",
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
			ParentPath:  "Parent",
			ActorID:     "alice",
		})
		require.NoError(t, err)

		removedEntity, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: "Removed",
			ParentPath:  "Parent",
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
			ParentPath:  "GP",
			ActorID:     "alice",
		})
		require.NoError(t, err)

		_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: "Leaf",
			ParentPath:  "GP:Mid",
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
			ParentPath:  "GP",
			ActorID:     "alice",
		})
		require.NoError(t, err)

		leaf, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: "Leaf",
			ParentPath:  "GP:Mid",
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
