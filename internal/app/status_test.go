package app_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func TestMarkLost_Missing(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage", ActorID: "alice",
	})
	require.NoError(t, err)

	wrench, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Wrench", ParentPath: "Garage", ActorID: "alice",
	})
	require.NoError(t, err)

	results, err := a.MarkLost(ctx, []app.ChangeStatusRequest{{
		EntityID: wrench.EntityID,
		ActorID:  "alice",
	}})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, inventory.EntityStatusMissing, results[0].Status)
}

func TestMarkLost_LockedEntity_Forbidden(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	garage, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage", Locked: true, ActorID: "alice",
	})
	require.NoError(t, err)

	_, err = a.MarkLost(ctx, []app.ChangeStatusRequest{{
		EntityID: garage.EntityID,
		ActorID:  "alice",
	}})
	assert.ErrorContains(t, err, "locked")
}
