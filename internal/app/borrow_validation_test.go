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
