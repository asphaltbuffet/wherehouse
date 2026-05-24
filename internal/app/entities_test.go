package app_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
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

func TestCreateEntity_RootPlace(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	result, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage",
		EntityType:  inventory.EntityTypePlace,
		ActorID:     "alice",
	})
	require.NoError(t, err)
	assert.Equal(t, "Garage", result.DisplayName)
	assert.Equal(t, "Garage", result.FullPathDisplay)
	assert.Equal(t, inventory.EntityTypePlace, result.EntityType)
}

func TestCreateEntity_NestedPath(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage",
		EntityType:  inventory.EntityTypePlace,
		ActorID:     "alice",
	})
	require.NoError(t, err)

	result, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Toolbox",
		EntityType:  inventory.EntityTypeContainer,
		ParentPath:  "Garage",
		ActorID:     "alice",
	})
	require.NoError(t, err)
	assert.Equal(t, "Garage:Toolbox", result.FullPathDisplay)
}

func TestCreateEntity_PlaceInNonPlace_Error(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage",
		EntityType:  inventory.EntityTypePlace,
		ActorID:     "alice",
	})
	require.NoError(t, err)

	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Wrench",
		EntityType:  inventory.EntityTypeLeaf,
		ParentPath:  "Garage",
		ActorID:     "alice",
	})
	require.NoError(t, err)

	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Zone",
		EntityType:  inventory.EntityTypePlace,
		ParentPath:  "Garage:Wrench",
		ActorID:     "alice",
	})
	assert.Error(t, err)
}

func TestGetEntity_ByPath(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage",
		EntityType:  inventory.EntityTypePlace,
		ActorID:     "alice",
	})
	require.NoError(t, err)

	result, err := a.GetEntityByPath(ctx, "Garage")
	require.NoError(t, err)
	assert.Equal(t, "Garage", result.DisplayName)
}

func TestGetEntityByPath_Disambiguation(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	// Create two places with the same name at different paths.
	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Shelf", EntityType: inventory.EntityTypePlace, ActorID: "alice",
	})
	require.NoError(t, err)

	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage", EntityType: inventory.EntityTypePlace, ActorID: "alice",
	})
	require.NoError(t, err)

	garageID := "Garage"
	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Shelf", EntityType: inventory.EntityTypePlace, ParentPath: garageID, ActorID: "alice",
	})
	require.NoError(t, err)

	// "Shelf" (root-level) and "Garage:Shelf" both exist.
	result, err := a.GetEntityByPath(ctx, "Garage:Shelf")
	require.NoError(t, err)
	assert.Equal(t, "Garage:Shelf", result.FullPathDisplay)

	result2, err := a.GetEntityByPath(ctx, "Shelf")
	require.NoError(t, err)
	assert.Equal(t, "Shelf", result2.FullPathDisplay)
}

func TestRemoveEntity_ThenMutate_Error(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage", EntityType: inventory.EntityTypePlace, ActorID: "alice",
	})
	require.NoError(t, err)

	err = a.RemoveEntity(ctx, app.RemoveEntityRequest{
		EntityPath: "Garage", ActorID: "alice",
	})
	require.NoError(t, err)

	// Attempting to rename a removed entity should fail.
	_, err = a.RenameEntity(ctx, app.RenameEntityRequest{
		EntityPath: "Garage", NewName: "Workshop", ActorID: "alice",
	})
	assert.Error(t, err)
}

func TestListEntities(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	for _, name := range []string{"Garage", "Basement", "Kitchen"} {
		_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: name,
			EntityType:  inventory.EntityTypePlace,
			ActorID:     "alice",
		})
		require.NoError(t, err)
	}

	results, err := a.ListEntities(ctx)
	require.NoError(t, err)
	assert.Len(t, results, 3)
}
