package app_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
)

func TestBorrowEntities_RejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		req     app.BorrowEntityRequest
		wantErr error
	}{
		{
			name:    "empty lender",
			req:     app.BorrowEntityRequest{DisplayName: "Drill", ActorID: "test"},
			wantErr: app.ErrLenderRequired,
		},
		{
			name:    "whitespace lender",
			req:     app.BorrowEntityRequest{DisplayName: "Drill", StatusContext: "   ", ActorID: "test"},
			wantErr: app.ErrLenderRequired,
		},
		{
			name:    "empty name",
			req:     app.BorrowEntityRequest{StatusContext: "Alice", ActorID: "test"},
			wantErr: entitypath.ErrEmptySegment,
		},
		{
			name:    "name with separator",
			req:     app.BorrowEntityRequest{DisplayName: "Alice:Drill", StatusContext: "Alice", ActorID: "test"},
			wantErr: entitypath.ErrSegmentContainsSeparator,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := apptesting.OpenApp(t)

			_, err := a.BorrowEntities(t.Context(), []app.BorrowEntityRequest{tt.req})
			require.ErrorIs(t, err, tt.wantErr)

			entities, err := a.ListEntities(t.Context())
			require.NoError(t, err)
			assert.Empty(t, entities, "nothing may be created on validation failure")
		})
	}
}

func TestBorrowEntities_TrimsLender(t *testing.T) {
	a := apptesting.OpenApp(t)

	results, err := a.BorrowEntities(t.Context(), []app.BorrowEntityRequest{
		{DisplayName: "Drill", StatusContext: "  Alice  ", ActorID: "test"},
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Alice", results[0].StatusContext)
}

func TestBorrowEntities_DiscreteParent_InvalidDestination(t *testing.T) {
	a := apptesting.OpenApp(t)
	_, err := a.CreateEntity(t.Context(), app.CreateEntityRequest{
		DisplayName: "Box", Discrete: true, ActorID: "test",
	})
	require.NoError(t, err)

	_, err = a.BorrowEntities(t.Context(), []app.BorrowEntityRequest{
		{DisplayName: "Drill", ParentID: apptesting.MustLookupID(t, a, "Box"), StatusContext: "Alice", ActorID: "test"},
	})

	require.ErrorIs(t, err, app.ErrInvalidDestination)
	assert.ErrorContains(t, err, "discrete")
}

// #288: borrow lands under the picked same-named sibling.
func TestBorrowEntities_UnderSecondSameNamedSibling(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Bin", ActorID: "test"})
	require.NoError(t, err)
	bin2, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Bin", ActorID: "test"})
	require.NoError(t, err)

	got, err := a.BorrowEntities(ctx, []app.BorrowEntityRequest{
		{DisplayName: "Drill", ParentID: bin2.EntityID, StatusContext: "Alice", ActorID: "test"},
	})

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, bin2.EntityID, got[0].ParentID)
}

func TestBorrowEntities_RemovedParent_NotFound(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	shelf, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Shelf", ActorID: "test"})
	require.NoError(t, err)
	require.NoError(t, a.RemoveEntity(ctx, app.RemoveEntityRequest{EntityID: shelf.EntityID, ActorID: "test"}))

	_, err = a.BorrowEntities(ctx, []app.BorrowEntityRequest{
		{DisplayName: "Drill", ParentID: shelf.EntityID, StatusContext: "Alice", ActorID: "test"},
	})

	require.ErrorIs(t, err, app.ErrNotFound)
}
