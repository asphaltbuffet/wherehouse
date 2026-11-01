package app_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func TestFindParentCandidates_FiltersDiscreteBeforeLimit(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	// Three discrete "Box" entities rank closer to "box" than "Toolbox" does.
	for _, name := range []string{"Box", "Box A", "Box B"} {
		_, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: name, Discrete: true, ActorID: "test"})
		require.NoError(t, err)
	}
	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Toolbox", ActorID: "test"})
	require.NoError(t, err)

	results, err := a.FindParentCandidates(ctx, app.FindEntitiesRequest{Query: "box", Limit: 1})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Toolbox", results[0].Entity.DisplayName)
}

func TestFindParentCandidates_BorrowedIsEligible(t *testing.T) {
	a := apptesting.OpenApp(t)
	_, err := a.BorrowEntities(t.Context(), []app.BorrowEntityRequest{
		{DisplayName: "Drill Case", StatusContext: "Alice", ActorID: "test"},
	})
	require.NoError(t, err)

	results, err := a.FindParentCandidates(t.Context(), app.FindEntitiesRequest{Query: "case"})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, inventory.EntityStatusBorrowed, results[0].Entity.Status)
}

func TestEntityResult_CanAddChild(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	mk := func(name string, discrete bool) string {
		e, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: name, Discrete: discrete, ActorID: "t"})
		require.NoError(t, err)
		return e.EntityID
	}
	okID := mk("Shelf", false)
	discreteID := mk("Box of Nails", true)
	gone := mk("Old Crate", false)
	require.NoError(t, a.RemoveEntity(ctx, app.RemoveEntityRequest{EntityID: gone, ActorID: "t"}))
	borrowed, err := a.BorrowEntities(ctx, []app.BorrowEntityRequest{
		{DisplayName: "Drill Case", StatusContext: "Alice", ActorID: "t"},
	})
	require.NoError(t, err)

	for _, tt := range []struct {
		name string
		id   string
		want bool
	}{
		{"ok", okID, true},
		{"discrete", discreteID, false},
		{"removed", gone, false},
		{"borrowed", borrowed[0].EntityID, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r, gerr := a.GetEntityByIDAnyStatus(ctx, tt.id)
			require.NoError(t, gerr)
			assert.Equal(t, tt.want, r.CanAddChild)
		})
	}
}

func TestFindMoveDestinations_ExcludesSelfDescendantsAndCurrentParent(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	for _, c := range []struct{ name, parent string }{
		{"Box Shelf", ""},
		{"Box", "Box Shelf"},
		{"Box Lid", "Box Shelf:Box"},
		{"Box Room", ""},
	} {
		_, err := a.CreateEntity(
			ctx,
			app.CreateEntityRequest{
				DisplayName: c.name,
				ParentID:    apptesting.MustLookupID(t, a, c.parent),
				ActorID:     "test",
			},
		)
		require.NoError(t, err)
	}
	box, err := a.LookupEntityByPath(ctx, "Box Shelf:Box")
	require.NoError(t, err)

	results, err := a.FindMoveDestinations(ctx, box.EntityID, app.RelocationMove, app.FindEntitiesRequest{Query: "box"})

	require.NoError(t, err)
	var names []string
	for _, r := range results {
		names = append(names, r.Entity.FullPathDisplay)
	}
	assert.Equal(t, []string{"Box Room"}, names)
}

// Exclusions apply before the limit, so excluded matches never crowd out valid ones.
func TestFindMoveDestinations_FiltersBeforeLimit(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Bin", ActorID: "test"})
	require.NoError(t, err)
	for _, name := range []string{"Bin A", "Bin B"} {
		_, err = a.CreateEntity(
			ctx,
			app.CreateEntityRequest{DisplayName: name, ParentID: apptesting.MustLookupID(t, a, "Bin"), ActorID: "test"},
		)
		require.NoError(t, err)
	}
	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Cabinet", ActorID: "test"})
	require.NoError(t, err)
	bin, err := a.LookupEntityByPath(ctx, "Bin")
	require.NoError(t, err)

	results, err := a.FindMoveDestinations(
		ctx, bin.EntityID, app.RelocationMove, app.FindEntitiesRequest{Query: "b", Limit: 1},
	)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Cabinet", results[0].Entity.DisplayName)
}

func TestFindMoveDestinations_UnknownEntity(t *testing.T) {
	a := apptesting.OpenApp(t)

	_, err := a.FindMoveDestinations(t.Context(), "nope", app.RelocationMove, app.FindEntitiesRequest{Query: "x"})

	require.ErrorIs(t, err, app.ErrNotFound)
}

// A live entity under a removed ancestor (#287) is still a descendant, so the
// picker must not offer it.
func TestFindMoveDestinations_ExcludesDescendantUnderRemovedAncestor(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	garage, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Garage", ActorID: "test"})
	require.NoError(t, err)
	box, err := a.CreateEntity(
		ctx,
		app.CreateEntityRequest{DisplayName: "Box", ParentID: apptesting.MustLookupID(t, a, "Garage"), ActorID: "test"},
	)
	require.NoError(t, err)
	_, err = a.CreateEntity(
		ctx,
		app.CreateEntityRequest{
			DisplayName: "Bit",
			ParentID:    apptesting.MustLookupID(t, a, "Garage:Box"),
			ActorID:     "test",
		},
	)
	require.NoError(t, err)
	apptesting.RemoveLeavingLiveChildren(t, a, box.EntityID) // pre-#287 data: live child under removed parent

	results, err := a.FindMoveDestinations(
		ctx, garage.EntityID, app.RelocationMove, app.FindEntitiesRequest{Query: "bit"},
	)

	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestFindMoveDestinations_DisplacedEntity(t *testing.T) {
	f := newHomeFixture(t)
	ctx := t.Context()
	_, err := f.use(t, f.drill.EntityID, f.deck.EntityID)
	require.NoError(t, err)
	names := func(rel app.Relocation, q string) []string {
		res, ferr := f.a.FindMoveDestinations(ctx, f.drill.EntityID, rel, app.FindEntitiesRequest{Query: q})
		require.NoError(t, ferr)
		out := make([]string, len(res))
		for i, r := range res {
			out[i] = r.Entity.DisplayName
		}
		return out
	}

	assert.Equal(t, []string{"Deck"}, names(app.RelocationMove, "deck"), "move offers the current parent (re-home)")
	assert.Empty(t, names(app.RelocationMove, "garage"), "move never offers the home")
	assert.Empty(t, names(app.RelocationUse, "deck"), "use never offers the current parent")
	assert.Empty(t, names(app.RelocationUse, "garage"), "use never offers the home")
}
