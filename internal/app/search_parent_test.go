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
