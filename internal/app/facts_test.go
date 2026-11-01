package app_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
)

func TestEntityActions_ReflectsRelationalFacts(t *testing.T) {
	ctx := context.Background()
	a := apptesting.OpenApp(t)

	garage, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Garage", ActorID: "t"})
	require.NoError(t, err)
	deck, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Deck", ActorID: "t"})
	require.NoError(t, err)
	drill, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Drill", ParentID: garage.EntityID, ActorID: "t",
	})
	require.NoError(t, err)
	_, err = a.UseEntity(ctx, app.UseEntityRequest{EntityID: drill.EntityID, NewParentID: deck.EntityID, ActorID: "t"})
	require.NoError(t, err)

	garageSet, err := a.EntityActions(ctx, garage.EntityID)
	require.NoError(t, err)
	rej := garageSet.Rejection(app.ActionRemove)
	require.NotNil(t, rej, "Garage is home to the displaced Drill")
	require.ErrorIs(t, rej, app.ErrIsHome)
	assert.Equal(t, 1, rej.Count)

	deckSet, err := a.EntityActions(ctx, deck.EntityID)
	require.NoError(t, err)
	require.ErrorIs(t, deckSet.Rejection(app.ActionSetDiscrete), app.ErrHasChildren)

	drillSet, err := a.EntityActions(ctx, drill.EntityID)
	require.NoError(t, err)
	assert.Nil(t, drillSet.Rejection(app.ActionReturn), "displaced ok entity can be returned")
	require.ErrorIs(t, drillSet.Rejection(app.ActionLock), app.ErrDisplaced)
}

func TestEntityActions_UnknownEntity(t *testing.T) {
	a := apptesting.OpenApp(t)
	_, err := a.EntityActions(context.Background(), "nope000000")
	require.ErrorIs(t, err, app.ErrNotFound)
}
