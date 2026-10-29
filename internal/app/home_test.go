package app_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

type homeFixture struct {
	a                        *app.App
	garage, deck, drill, bit app.EntityResult
}

func newHomeFixture(t *testing.T) homeFixture {
	t.Helper()
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	mk := func(name, parent string) app.EntityResult {
		e, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: name, ParentID: parent, ActorID: "t"})
		require.NoError(t, err)
		return e
	}
	f := homeFixture{a: a}
	f.garage = mk("Garage", "")
	f.deck = mk("Deck", "")
	f.drill = mk("Drill", f.garage.EntityID)
	f.bit = mk("Bit", f.drill.EntityID)
	return f
}

func (f homeFixture) use(t *testing.T, id, dest string) (app.EntityResult, error) {
	t.Helper()
	return f.a.UseEntity(t.Context(), app.UseEntityRequest{EntityID: id, NewParentID: dest, ActorID: "t"})
}

func TestUseEntity_DisplacesAndKeepsHome(t *testing.T) {
	f := newHomeFixture(t)
	got, err := f.use(t, f.drill.EntityID, f.deck.EntityID)
	require.NoError(t, err)
	assert.Equal(t, "Deck:Drill", got.FullPathDisplay)
	assert.Equal(t, f.garage.EntityID, got.HomeID)
	assert.Equal(t, "Garage", got.HomePathDisplay)
	assert.True(t, got.IsDisplaced())
}

func TestUseEntity_AgainWhileDisplacedKeepsHome(t *testing.T) {
	f := newHomeFixture(t)
	kitchen, err := f.a.CreateEntity(t.Context(), app.CreateEntityRequest{DisplayName: "Kitchen", ActorID: "t"})
	require.NoError(t, err)
	_, err = f.use(t, f.drill.EntityID, f.deck.EntityID)
	require.NoError(t, err)
	got, err := f.use(t, f.drill.EntityID, kitchen.EntityID)
	require.NoError(t, err)
	assert.Equal(t, f.garage.EntityID, got.HomeID)
}

func TestUseEntity_Rejections(t *testing.T) {
	f := newHomeFixture(t)
	_, err := f.use(t, f.drill.EntityID, f.garage.EntityID)
	require.ErrorIs(t, err, app.ErrDestinationIsHome, "already at home: destination is home")
	require.ErrorIs(t, err, app.ErrInvalidDestination)

	_, err = f.use(t, f.drill.EntityID, f.deck.EntityID)
	require.NoError(t, err)
	_, err = f.use(t, f.drill.EntityID, f.garage.EntityID)
	require.ErrorIs(t, err, app.ErrDestinationIsHome)
	assert.Contains(t, err.Error(), "use 'return' instead")

	_, err = f.use(t, f.drill.EntityID, f.bit.EntityID)
	require.ErrorIs(t, err, app.ErrInvalidDestination, "into own contents")
}

func TestUseEntity_RootHome(t *testing.T) {
	f := newHomeFixture(t)
	got, err := f.use(t, f.deck.EntityID, f.garage.EntityID)
	require.NoError(t, err)
	assert.True(t, got.IsDisplaced())
	assert.Equal(t, "(root)", got.HomeLabel())

	_, err = f.use(t, f.deck.EntityID, "")
	require.ErrorIs(t, err, app.ErrDestinationIsHome, "root is its home")
}

func TestUseEntity_LockedAndStatusGuards(t *testing.T) {
	f := newHomeFixture(t)
	ctx := t.Context()
	locked := true
	_, err := f.a.SetAttributes(ctx, []app.SetAttributesRequest{
		{EntityID: f.drill.EntityID, Locked: &locked, ActorID: "t"},
	})
	require.NoError(t, err)
	_, err = f.use(t, f.drill.EntityID, f.deck.EntityID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "locked")
}

func TestReparent_RehomesInPlaceWhenDisplaced(t *testing.T) {
	f := newHomeFixture(t)
	_, err := f.use(t, f.drill.EntityID, f.deck.EntityID)
	require.NoError(t, err)
	got, err := f.a.ReparentEntity(t.Context(), app.ReparentEntityRequest{
		EntityID: f.drill.EntityID, NewParentID: f.deck.EntityID, ActorID: "t",
	})
	require.NoError(t, err)
	assert.Equal(t, f.deck.EntityID, got.HomeID)
	assert.False(t, got.IsDisplaced())

	hist, err := f.a.GetHistory(t.Context(), app.GetHistoryRequest{EntityID: f.drill.EntityID})
	require.NoError(t, err)
	require.NotEmpty(t, hist)
	var last inventory.EventType
	var lastID int64
	for _, h := range hist {
		if h.EventID > lastID {
			lastID, last = h.EventID, h.EventType
		}
	}
	assert.Equal(t, inventory.EntityRehomedEvent, last)
}

func TestReparent_ToHomeWhenDisplacedRejected(t *testing.T) {
	f := newHomeFixture(t)
	_, err := f.use(t, f.drill.EntityID, f.deck.EntityID)
	require.NoError(t, err)
	_, err = f.a.ReparentEntity(t.Context(), app.ReparentEntityRequest{
		EntityID: f.drill.EntityID, NewParentID: f.garage.EntityID, ActorID: "t",
	})
	require.ErrorIs(t, err, app.ErrDestinationIsHome)
}

func TestReparent_ElsewhereClearsDisplacement(t *testing.T) {
	f := newHomeFixture(t)
	kitchen, err := f.a.CreateEntity(t.Context(), app.CreateEntityRequest{DisplayName: "Kitchen", ActorID: "t"})
	require.NoError(t, err)
	_, err = f.use(t, f.drill.EntityID, f.deck.EntityID)
	require.NoError(t, err)
	got, err := f.a.ReparentEntity(t.Context(), app.ReparentEntityRequest{
		EntityID: f.drill.EntityID, NewParentID: kitchen.EntityID, ActorID: "t",
	})
	require.NoError(t, err)
	assert.Equal(t, kitchen.EntityID, got.HomeID)
	assert.False(t, got.IsDisplaced())
}

func TestUseEntity_DescendantsNotDisplaced(t *testing.T) {
	f := newHomeFixture(t)
	_, err := f.use(t, f.drill.EntityID, f.deck.EntityID)
	require.NoError(t, err)
	bit, err := f.a.GetEntityByID(t.Context(), f.bit.EntityID)
	require.NoError(t, err)
	assert.Equal(t, "Deck:Drill:Bit", bit.FullPathDisplay)
	assert.False(t, bit.IsDisplaced())
}
