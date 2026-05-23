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
