package app_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

func TestApp_GetRootEntities(t *testing.T) {
	t.Run("returns only depth-0 non-removed entities", func(t *testing.T) {
		ctx := context.Background()
		a := openTestApp(t)

		_, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Garage", ActorID: "alice"})
		require.NoError(t, err)

		_, err = a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Shelf", ActorID: "alice"})
		require.NoError(t, err)

		_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: "Toolbox", ParentPath: "Garage", ActorID: "alice",
		})
		require.NoError(t, err)

		results, err := a.GetRootEntities(ctx)
		require.NoError(t, err)

		require.Len(t, results, 2)
		names := []string{results[0].DisplayName, results[1].DisplayName}
		assert.ElementsMatch(t, []string{"Garage", "Shelf"}, names)
	})

	t.Run("HasChildren reflects child presence", func(t *testing.T) {
		ctx := context.Background()
		a := openTestApp(t)

		_, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Garage", ActorID: "alice"})
		require.NoError(t, err)

		_, err = a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Shelf", ActorID: "alice"})
		require.NoError(t, err)

		_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: "Toolbox", ParentPath: "Garage", ActorID: "alice",
		})
		require.NoError(t, err)

		results, err := a.GetRootEntities(ctx)
		require.NoError(t, err)

		byName := map[string]app.EntityResult{}
		for _, r := range results {
			byName[r.DisplayName] = r
		}

		assert.True(t, byName["Garage"].HasChildren)
		assert.False(t, byName["Shelf"].HasChildren)
	})

	t.Run("removed root entity excluded", func(t *testing.T) {
		ctx := context.Background()
		a := openTestApp(t)

		_, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Active", ActorID: "alice"})
		require.NoError(t, err)

		gone, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Gone", ActorID: "alice"})
		require.NoError(t, err)

		err = a.RemoveEntity(ctx, app.RemoveEntityRequest{EntityID: gone.EntityID, ActorID: "alice"})
		require.NoError(t, err)

		results, err := a.GetRootEntities(ctx)
		require.NoError(t, err)

		require.Len(t, results, 1)
		assert.Equal(t, "Active", results[0].DisplayName)
	})
}
