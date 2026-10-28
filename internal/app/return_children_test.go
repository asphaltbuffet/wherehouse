package app_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// seedBorrowedCaseWithBits borrows "Drill Case" (root) and "Drill Case:Bits" from Alice.
func seedBorrowedCaseWithBits(t *testing.T, a *app.App) (string, string) {
	t.Helper()
	ctx := t.Context()
	c, err := a.BorrowEntities(ctx, []app.BorrowEntityRequest{
		{DisplayName: "Drill Case", StatusContext: "Alice", ActorID: "test"},
	})
	require.NoError(t, err)
	b, err := a.BorrowEntities(ctx, []app.BorrowEntityRequest{
		{
			DisplayName:   "Bits",
			ParentID:      apptesting.MustLookupID(t, a, "Drill Case"),
			StatusContext: "Alice",
			ActorID:       "test",
		},
	})
	require.NoError(t, err)
	return c[0].EntityID, b[0].EntityID
}

func TestMarkReturned_BorrowedWithLiveChildrenBlocked(t *testing.T) {
	a := apptesting.OpenApp(t)
	caseID, _ := seedBorrowedCaseWithBits(t, a)

	_, err := a.MarkReturned(t.Context(), []app.ChangeStatusRequest{{EntityID: caseID, ActorID: "test"}})
	require.ErrorIs(t, err, app.ErrHasChildren)

	got, err := a.GetEntityByID(t.Context(), caseID)
	require.NoError(t, err)
	assert.Equal(t, inventory.EntityStatusBorrowed, got.Status)
}

func TestMarkReturned_ChildrenFirstInSameBatch(t *testing.T) {
	a := apptesting.OpenApp(t)
	caseID, bitsID := seedBorrowedCaseWithBits(t, a)

	_, err := a.MarkReturned(t.Context(), []app.ChangeStatusRequest{
		{EntityID: bitsID, ActorID: "test"},
		{EntityID: caseID, ActorID: "test"},
	})
	require.NoError(t, err)

	_, err = a.GetEntityByID(t.Context(), caseID)
	require.ErrorIs(t, err, app.ErrNotFound, "returned borrowed entity is removed")
}

func TestMarkReturned_LoanedWithChildrenAllowed(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	box, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Toolbox", ActorID: "test"})
	require.NoError(t, err)
	_, err = a.CreateEntity(
		ctx,
		app.CreateEntityRequest{
			DisplayName: "Socket",
			ParentID:    apptesting.MustLookupID(t, a, "Toolbox"),
			ActorID:     "test",
		},
	)
	require.NoError(t, err)
	_, err = a.MarkLoaned(ctx, []app.ChangeStatusRequest{
		{EntityID: box.EntityID, StatusContext: "Bob", ActorID: "test"},
	})
	require.NoError(t, err)

	_, err = a.MarkReturned(ctx, []app.ChangeStatusRequest{{EntityID: box.EntityID, ActorID: "test"}})
	require.NoError(t, err)

	got, err := a.GetEntityByID(ctx, box.EntityID)
	require.NoError(t, err)
	assert.Equal(t, inventory.EntityStatusOk, got.Status)
}
