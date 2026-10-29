package app_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
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
	require.ErrorIs(t, err, app.ErrInvalidDestination)
	assert.Contains(t, err.Error(), "use 'return' instead")
}

func TestHomeGuard_ReturnBorrowedRejectedWhileHomeOfDisplaced(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	borrowed, err := a.BorrowEntities(ctx, []app.BorrowEntityRequest{
		{DisplayName: "Case", StatusContext: "Alice", ActorID: "t"},
	})
	require.NoError(t, err)
	caseID := borrowed[0].EntityID
	bit, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Bit", ParentID: caseID, ActorID: "t"})
	require.NoError(t, err)
	deck, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Deck", ActorID: "t"})
	require.NoError(t, err)
	_, err = a.UseEntity(ctx, app.UseEntityRequest{EntityID: bit.EntityID, NewParentID: deck.EntityID, ActorID: "t"})
	require.NoError(t, err)

	_, err = a.MarkReturned(ctx, []app.ChangeStatusRequest{{EntityID: caseID, ActorID: "t"}})
	require.ErrorIs(t, err, app.ErrIsHome)
	assert.Contains(t, err.Error(),
		`cannot return "Case": it is home to 1 displaced entities — return them first`)
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

func (f homeFixture) ret(t *testing.T, id string) ([]app.EntityResult, error) {
	t.Helper()
	return f.a.MarkReturned(t.Context(), []app.ChangeStatusRequest{{EntityID: id, ActorID: "t"}})
}

func TestReturn_DisplacedOkGoesHome(t *testing.T) {
	f := newHomeFixture(t)
	_, err := f.use(t, f.drill.EntityID, f.deck.EntityID)
	require.NoError(t, err)
	got, err := f.ret(t, f.drill.EntityID)
	require.NoError(t, err)
	assert.Equal(t, "Garage:Drill", got[0].FullPathDisplay)
	assert.Equal(t, inventory.EntityStatusOk, got[0].Status)
	assert.False(t, got[0].IsDisplaced())
	bit, err := f.a.GetEntityByID(t.Context(), f.bit.EntityID)
	require.NoError(t, err)
	assert.Equal(t, "Garage:Drill:Bit", bit.FullPathDisplay)
}

func TestReturn_OkAtHomeRejected(t *testing.T) {
	f := newHomeFixture(t)
	_, err := f.ret(t, f.drill.EntityID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already at home")
}

func TestReturn_MissingRejected(t *testing.T) {
	f := newHomeFixture(t)
	_, err := f.use(t, f.drill.EntityID, f.deck.EntityID)
	require.NoError(t, err)
	_, err = f.a.MarkLost(t.Context(), []app.ChangeStatusRequest{{EntityID: f.drill.EntityID, ActorID: "t"}})
	require.NoError(t, err)
	_, err = f.ret(t, f.drill.EntityID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mark it found first")

	found, err := f.a.MarkFound(t.Context(), []app.ChangeStatusRequest{{EntityID: f.drill.EntityID, ActorID: "t"}})
	require.NoError(t, err)
	assert.True(t, found[0].IsDisplaced(), "found does not relocate")
}

func TestReturn_LoanedAndDisplacedRestoresBoth(t *testing.T) {
	f := newHomeFixture(t)
	ctx := t.Context()
	_, err := f.use(t, f.drill.EntityID, f.deck.EntityID)
	require.NoError(t, err)
	_, err = f.a.MarkLoaned(ctx, []app.ChangeStatusRequest{
		{EntityID: f.drill.EntityID, StatusContext: "Sam", ActorID: "t"},
	})
	require.NoError(t, err)

	got, err := f.ret(t, f.drill.EntityID)
	require.NoError(t, err)
	assert.Equal(t, inventory.EntityStatusOk, got[0].Status)
	assert.Equal(t, "Garage:Drill", got[0].FullPathDisplay)

	hist, err := f.a.GetHistory(ctx, app.GetHistoryRequest{EntityID: f.drill.EntityID})
	require.NoError(t, err)
	// History is newest-first: returned_home landed after status_changed.
	require.GreaterOrEqual(t, len(hist), 2)
	assert.Equal(t, inventory.EntityStatusChangedEvent, hist[1].EventType)
	assert.Equal(t, inventory.EntityReturnedHomeEvent, hist[0].EventType)
}

func TestReturn_LoanedAtHomeStatusOnly(t *testing.T) {
	f := newHomeFixture(t)
	ctx := t.Context()
	_, err := f.a.MarkLoaned(ctx, []app.ChangeStatusRequest{
		{EntityID: f.drill.EntityID, StatusContext: "Sam", ActorID: "t"},
	})
	require.NoError(t, err)
	got, err := f.ret(t, f.drill.EntityID)
	require.NoError(t, err)
	assert.Equal(t, "Garage:Drill", got[0].FullPathDisplay)
	hist, err := f.a.GetHistory(ctx, app.GetHistoryRequest{EntityID: f.drill.EntityID})
	require.NoError(t, err)
	require.NotEmpty(t, hist)
	assert.Equal(t, inventory.EntityStatusChangedEvent, hist[0].EventType)
}

func TestReturn_BorrowedDisplacedRemovedWithoutRelocation(t *testing.T) {
	f := newHomeFixture(t)
	ctx := t.Context()
	b, err := f.a.BorrowEntities(ctx, []app.BorrowEntityRequest{
		{DisplayName: "Ladder", ParentID: f.garage.EntityID, StatusContext: "Neighbor"},
	})
	require.NoError(t, err)
	_, err = f.use(t, b[0].EntityID, f.deck.EntityID)
	require.NoError(t, err)
	got, err := f.ret(t, b[0].EntityID)
	require.NoError(t, err)
	assert.Equal(t, inventory.EntityStatusRemoved, got[0].Status)
	assert.Equal(t, "Deck:Ladder", got[0].FullPathDisplay, "no trip home before leaving")
}

func TestReturn_RootHome(t *testing.T) {
	f := newHomeFixture(t)
	_, err := f.use(t, f.deck.EntityID, f.garage.EntityID)
	require.NoError(t, err)
	got, err := f.ret(t, f.deck.EntityID)
	require.NoError(t, err)
	assert.Equal(t, "Deck", got[0].FullPathDisplay)
	assert.Empty(t, got[0].ParentID)
}

func TestReturn_BatchReturnsEverythingAtDeck(t *testing.T) {
	f := newHomeFixture(t)
	ctx := t.Context()
	saw, err := f.a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Saw", ParentID: f.garage.EntityID, ActorID: "t",
	})
	require.NoError(t, err)
	_, err = f.use(t, f.drill.EntityID, f.deck.EntityID)
	require.NoError(t, err)
	_, err = f.use(t, saw.EntityID, f.deck.EntityID)
	require.NoError(t, err)
	got, err := f.a.MarkReturned(ctx, []app.ChangeStatusRequest{
		{EntityID: f.drill.EntityID, ActorID: "t"}, {EntityID: saw.EntityID, ActorID: "t"},
	})
	require.NoError(t, err)
	assert.Equal(t, "Garage:Drill", got[0].FullPathDisplay)
	assert.Equal(t, "Garage:Saw", got[1].FullPathDisplay)
}

func TestReturn_LoanedAndDisplacedIsAtomicWhenHomeInvalid(t *testing.T) {
	ctx := t.Context()
	s, err := store.Open(store.Config{Path: filepath.Join(t.TempDir(), "test.db"), AutoMigrate: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	a := app.New(s)

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
	_, err = a.MarkLoaned(ctx, []app.ChangeStatusRequest{
		{EntityID: drill.EntityID, StatusContext: "Sam", ActorID: "t"},
	})
	require.NoError(t, err)

	// Make the home an invalid destination by bypassing app guards.
	raw, err := json.Marshal(eventbus.EntityDiscreteSetPayload{EntityID: garage.EntityID})
	require.NoError(t, err)
	_, err = eventbus.New(s).Dispatch(ctx, inventory.EntityDiscreteSetEvent, "t", raw, nil)
	require.NoError(t, err)

	before, err := a.GetHistory(ctx, app.GetHistoryRequest{EntityID: drill.EntityID})
	require.NoError(t, err)

	_, err = a.MarkReturned(ctx, []app.ChangeStatusRequest{{EntityID: drill.EntityID, ActorID: "t"}})
	require.ErrorIs(t, err, app.ErrInvalidDestination)

	got, err := a.GetEntityByID(ctx, drill.EntityID)
	require.NoError(t, err)
	assert.Equal(t, inventory.EntityStatusLoaned, got.Status, "status_changed must roll back")
	assert.Equal(t, "Deck:Drill", got.FullPathDisplay)
	after, err := a.GetHistory(ctx, app.GetHistoryRequest{EntityID: drill.EntityID})
	require.NoError(t, err)
	assert.Len(t, after, len(before), "neither event lands")
}

func TestHomeGuard_RemoveRejectedWhileHomeOfDisplaced(t *testing.T) {
	f := newHomeFixture(t)
	ctx := t.Context()
	shelf, err := f.a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Shelf", ParentID: f.garage.EntityID, ActorID: "t",
	})
	require.NoError(t, err)
	_, err = f.a.ReparentEntity(ctx, app.ReparentEntityRequest{
		EntityID: f.drill.EntityID, NewParentID: shelf.EntityID, ActorID: "t",
	})
	require.NoError(t, err)
	_, err = f.use(t, f.drill.EntityID, f.deck.EntityID)
	require.NoError(t, err)

	err = f.a.RemoveEntity(ctx, app.RemoveEntityRequest{EntityID: shelf.EntityID, ActorID: "t"})
	require.ErrorIs(t, err, app.ErrIsHome)
	assert.Contains(t, err.Error(), "return them first")
}

func TestHomeGuard_DiscreteRejectedWhileHomeOfDisplaced(t *testing.T) {
	f := newHomeFixture(t)
	ctx := t.Context()
	_, err := f.use(t, f.drill.EntityID, f.deck.EntityID)
	require.NoError(t, err)
	// Garage's only child (Drill) is away, so only the home guard can reject this.
	discrete := true
	_, err = f.a.SetAttributes(ctx, []app.SetAttributesRequest{{
		EntityID: f.garage.EntityID, Discrete: &discrete, ActorID: "t",
	}})
	require.ErrorIs(t, err, app.ErrIsHome)
}

func TestHomeGuard_LockRejectedWhileDisplaced(t *testing.T) {
	f := newHomeFixture(t)
	_, err := f.use(t, f.drill.EntityID, f.deck.EntityID)
	require.NoError(t, err)
	locked := true
	_, err = f.a.SetAttributes(t.Context(), []app.SetAttributesRequest{{
		EntityID: f.drill.EntityID, Locked: &locked, ActorID: "t",
	}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "entity is displaced — return it first")
}

func TestHomeGuard_DirectCycleRejected(t *testing.T) {
	f := newHomeFixture(t)
	ctx := t.Context()
	shelf, err := f.a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Shelf", ParentID: f.garage.EntityID, ActorID: "t",
	})
	require.NoError(t, err)
	_, err = f.a.ReparentEntity(ctx, app.ReparentEntityRequest{
		EntityID: f.drill.EntityID, NewParentID: shelf.EntityID, ActorID: "t",
	})
	require.NoError(t, err)
	_, err = f.use(t, f.drill.EntityID, f.deck.EntityID)
	require.NoError(t, err)

	_, err = f.a.ReparentEntity(ctx, app.ReparentEntityRequest{
		EntityID: shelf.EntityID, NewParentID: f.drill.EntityID, ActorID: "t",
	})
	require.ErrorIs(t, err, app.ErrInvalidDestination)
	assert.Contains(t, err.Error(), "home would end up inside it")
}

func TestHomeGuard_IndirectCycleRejected(t *testing.T) {
	f := newHomeFixture(t)
	ctx := t.Context()
	shelf, err := f.a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Shelf", ParentID: f.garage.EntityID, ActorID: "t",
	})
	require.NoError(t, err)
	_, err = f.a.ReparentEntity(ctx, app.ReparentEntityRequest{
		EntityID: f.drill.EntityID, NewParentID: shelf.EntityID, ActorID: "t",
	})
	require.NoError(t, err)
	_, err = f.use(t, f.drill.EntityID, f.deck.EntityID) // drill home = Shelf, inside Garage
	require.NoError(t, err)

	// Garage contains the drill's home (Shelf); moving it into the drill would nest the home in the drill.
	_, err = f.a.ReparentEntity(ctx, app.ReparentEntityRequest{
		EntityID: f.garage.EntityID, NewParentID: f.drill.EntityID, ActorID: "t",
	})
	require.ErrorIs(t, err, app.ErrInvalidDestination)
	assert.Contains(t, err.Error(), "home would end up inside it")

	// Same through use, into a descendant of the drill.
	_, err = f.use(t, f.garage.EntityID, f.bit.EntityID)
	require.ErrorIs(t, err, app.ErrInvalidDestination)
	assert.Contains(t, err.Error(), "home would end up inside it")

	// Moving Deck (which holds the drill) somewhere harmless is fine.
	yard, err := f.a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Yard", ActorID: "t"})
	require.NoError(t, err)
	_, err = f.a.ReparentEntity(ctx, app.ReparentEntityRequest{
		EntityID: f.deck.EntityID, NewParentID: yard.EntityID, ActorID: "t",
	})
	require.NoError(t, err)
}

func TestHomeGuard_ReturnedHomeAllowsRemove(t *testing.T) {
	f := newHomeFixture(t)
	ctx := t.Context()
	_, err := f.use(t, f.bit.EntityID, f.deck.EntityID) // bit home = Drill
	require.NoError(t, err)
	_, err = f.ret(t, f.bit.EntityID)
	require.NoError(t, err)
	err = f.a.RemoveEntity(ctx, app.RemoveEntityRequest{EntityID: f.bit.EntityID, ActorID: "t"})
	require.NoError(t, err)
	// Drill now has no live children and is home to nobody displaced.
	err = f.a.RemoveEntity(ctx, app.RemoveEntityRequest{EntityID: f.drill.EntityID, ActorID: "t"})
	require.NoError(t, err)
}
