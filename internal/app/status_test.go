package app_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func TestChangeStatus_Missing(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage", EntityType: inventory.EntityTypePlace, ActorID: "alice",
	})
	require.NoError(t, err)

	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Wrench", EntityType: inventory.EntityTypeLeaf, ParentPath: "Garage", ActorID: "alice",
	})
	require.NoError(t, err)

	result, err := a.ChangeStatus(ctx, app.ChangeStatusRequest{
		EntityPath:    "Garage:Wrench",
		Status:        inventory.EntityStatusMissing,
		StatusContext: "lost at job site",
		ActorID:       "alice",
	})
	require.NoError(t, err)
	assert.Equal(t, inventory.EntityStatusMissing, result.Status)
	assert.Equal(t, "lost at job site", result.StatusContext)
}
