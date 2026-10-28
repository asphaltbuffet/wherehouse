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

func TestEntityResult_IsEligibleParent(t *testing.T) {
	tests := []struct {
		name string
		e    app.EntityResult
		want bool
	}{
		{"ok", app.EntityResult{Status: inventory.EntityStatusOk}, true},
		{"missing", app.EntityResult{Status: inventory.EntityStatusMissing}, true},
		{"loaned", app.EntityResult{Status: inventory.EntityStatusLoaned}, true},
		{"borrowed", app.EntityResult{Status: inventory.EntityStatusBorrowed}, true},
		{"removed", app.EntityResult{Status: inventory.EntityStatusRemoved}, false},
		{"discrete", app.EntityResult{Status: inventory.EntityStatusOk, Discrete: true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.e.IsEligibleParent())
		})
	}
}

func TestEntityResult_CanMove(t *testing.T) {
	tests := []struct {
		name string
		e    app.EntityResult
		want bool
	}{
		{"ok", app.EntityResult{Status: inventory.EntityStatusOk}, true},
		{"borrowed", app.EntityResult{Status: inventory.EntityStatusBorrowed}, true},
		{"missing", app.EntityResult{Status: inventory.EntityStatusMissing}, false},
		{"loaned", app.EntityResult{Status: inventory.EntityStatusLoaned}, false},
		{"removed", app.EntityResult{Status: inventory.EntityStatusRemoved}, false},
		{"locked ok", app.EntityResult{Status: inventory.EntityStatusOk, Locked: true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.e.CanMove())
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
			app.CreateEntityRequest{DisplayName: c.name, ParentID: apptesting.MustLookupID(t, a, c.parent), ActorID: "test"},
		)
		require.NoError(t, err)
	}
	box, err := a.LookupEntityByPath(ctx, "Box Shelf:Box")
	require.NoError(t, err)

	results, err := a.FindMoveDestinations(ctx, box.EntityID, app.FindEntitiesRequest{Query: "box"})

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
		_, err = a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: name, ParentID: apptesting.MustLookupID(t, a, "Bin"), ActorID: "test"})
		require.NoError(t, err)
	}
	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Cabinet", ActorID: "test"})
	require.NoError(t, err)
	bin, err := a.LookupEntityByPath(ctx, "Bin")
	require.NoError(t, err)

	results, err := a.FindMoveDestinations(ctx, bin.EntityID, app.FindEntitiesRequest{Query: "b", Limit: 1})

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Cabinet", results[0].Entity.DisplayName)
}

func TestFindMoveDestinations_UnknownEntity(t *testing.T) {
	a := apptesting.OpenApp(t)

	_, err := a.FindMoveDestinations(t.Context(), "nope", app.FindEntitiesRequest{Query: "x"})

	require.ErrorIs(t, err, app.ErrNotFound)
}

// A live entity under a removed ancestor (#287) is still a descendant, so the
// picker must not offer it.
func TestFindMoveDestinations_ExcludesDescendantUnderRemovedAncestor(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	garage, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Garage", ActorID: "test"})
	require.NoError(t, err)
	box, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Box", ParentID: apptesting.MustLookupID(t, a, "Garage"), ActorID: "test"})
	require.NoError(t, err)
	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Bit", ParentID: apptesting.MustLookupID(t, a, "Garage:Box"), ActorID: "test"})
	require.NoError(t, err)
	apptesting.RemoveLeavingLiveChildren(t, a, box.EntityID) // pre-#287 data: live child under removed parent

	results, err := a.FindMoveDestinations(ctx, garage.EntityID, app.FindEntitiesRequest{Query: "bit"})

	require.NoError(t, err)
	assert.Empty(t, results)
}
