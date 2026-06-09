package app_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

func TestFindEntities_ExactMatch(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	for _, name := range []string{"Garage", "Basement", "Kitchen"} {
		_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: name, ActorID: "alice",
		})
		require.NoError(t, err)
	}

	results, err := a.FindEntities(ctx, app.FindEntitiesRequest{Query: "Garage"})
	require.NoError(t, err)
	require.NotEmpty(t, results)
	assert.Equal(t, "Garage", results[0].Entity.DisplayName)
	assert.Equal(t, 0, results[0].Distance)
}
