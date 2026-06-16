package borrow_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/borrow"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func seedShelf(t *testing.T, a *app.App) {
	t.Helper()
	_, err := a.CreateEntity(t.Context(), app.CreateEntityRequest{
		DisplayName: "Shelf", ActorID: "test",
	})
	require.NoError(t, err)
}

func TestRunBorrow_SinglePath(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedShelf(t, a)

	cmd := borrow.NewBorrowCmd(a)
	cmd.SetArgs([]string{"Shelf:Alice's Drill", "--from", "Alice"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	entities, err := a.ListEntities(t.Context())
	require.NoError(t, err)
	found := false
	for _, e := range entities {
		if e.FullPathDisplay == "Shelf:Alice's Drill" {
			assert.Equal(t, inventory.EntityStatusBorrowed, e.Status)
			assert.Equal(t, "Alice", e.StatusContext)
			found = true
		}
	}
	assert.True(t, found, "borrowed entity not found")
}

func TestRunBorrow_MultiplePaths(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedShelf(t, a)

	cmd := borrow.NewBorrowCmd(a)
	cmd.SetArgs([]string{"Shelf:Bob's Ladder", "Shelf:Bob's Saw", "--from", "Bob"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	entities, err := a.ListEntities(t.Context())
	require.NoError(t, err)
	borrowed := 0
	for _, e := range entities {
		if e.FullPathDisplay == "Shelf:Bob's Ladder" || e.FullPathDisplay == "Shelf:Bob's Saw" {
			assert.Equal(t, inventory.EntityStatusBorrowed, e.Status)
			assert.Equal(t, "Bob", e.StatusContext)
			borrowed++
		}
	}
	assert.Equal(t, 2, borrowed)
}

func TestRunBorrow_AtomicOnFailure(t *testing.T) {
	a := apptesting.OpenApp(t)
	// No shelf — second path's parent won't exist either, first should also be rolled back
	// Use a discrete entity as parent to force failure on second arg
	_, err := a.CreateEntity(t.Context(), app.CreateEntityRequest{
		DisplayName: "Box", Discrete: true, ActorID: "test",
	})
	require.NoError(t, err)

	cmd := borrow.NewBorrowCmd(a)
	// Second path uses discrete parent — should fail, rolling back first
	cmd.SetArgs([]string{"Box:Item1", "Box:Item2", "--from", "Alice"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err = cmd.Execute()
	require.Error(t, err)

	entities, err := a.ListEntities(t.Context())
	require.NoError(t, err)
	for _, e := range entities {
		assert.NotEqual(t, "Box:Item1", e.FullPathDisplay, "Item1 should not have been created")
	}
}

func TestRunBorrow_FromRequired(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := borrow.NewBorrowCmd(a)
	cmd.SetArgs([]string{"Shelf:Drill"}) // missing --from
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.Error(t, cmd.Execute())
}

func TestRunBorrow_BlocksLost(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedShelf(t, a)

	_, err := a.BorrowEntities(t.Context(), []app.BorrowEntityRequest{
		{DisplayName: "Drill", ParentPath: "Shelf", StatusContext: "Alice", ActorID: "test"},
	})
	require.NoError(t, err)

	drill, err := a.LookupEntityByPath(t.Context(), "Shelf:Drill")
	require.NoError(t, err)
	_, err = a.MarkLost(t.Context(), []app.ChangeStatusRequest{
		{EntityID: drill.EntityID, Status: inventory.EntityStatusMissing, ActorID: "test"},
	})
	assert.ErrorContains(t, err, "borrowed")
}

func TestRunBorrow_BlocksFound(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedShelf(t, a)

	_, err := a.BorrowEntities(t.Context(), []app.BorrowEntityRequest{
		{DisplayName: "Drill", ParentPath: "Shelf", StatusContext: "Alice", ActorID: "test"},
	})
	require.NoError(t, err)

	drill, err := a.LookupEntityByPath(t.Context(), "Shelf:Drill")
	require.NoError(t, err)
	_, err = a.MarkFound(t.Context(), []app.ChangeStatusRequest{
		{EntityID: drill.EntityID, Status: inventory.EntityStatusOk, ActorID: "test"},
	})
	assert.ErrorContains(t, err, "borrowed")
}

func TestRunBorrow_BlocksLoan(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedShelf(t, a)

	_, err := a.BorrowEntities(t.Context(), []app.BorrowEntityRequest{
		{DisplayName: "Drill", ParentPath: "Shelf", StatusContext: "Alice", ActorID: "test"},
	})
	require.NoError(t, err)

	drill, err := a.LookupEntityByPath(t.Context(), "Shelf:Drill")
	require.NoError(t, err)
	_, err = a.MarkLoaned(t.Context(), []app.ChangeStatusRequest{
		{EntityID: drill.EntityID, Status: inventory.EntityStatusLoaned, ActorID: "test"},
	})
	assert.ErrorContains(t, err, "borrowed")
}

func TestRunBorrow_BlocksRemove(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedShelf(t, a)

	_, err := a.BorrowEntities(t.Context(), []app.BorrowEntityRequest{
		{DisplayName: "Drill", ParentPath: "Shelf", StatusContext: "Alice", ActorID: "test"},
	})
	require.NoError(t, err)

	drill, err := a.LookupEntityByPath(t.Context(), "Shelf:Drill")
	require.NoError(t, err)
	err = a.RemoveEntity(t.Context(), app.RemoveEntityRequest{
		EntityID: drill.EntityID, ActorID: "test",
	})
	assert.ErrorContains(t, err, "borrowed")
}

func TestRunBorrow_ReturnSetsRemoved(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedShelf(t, a)

	_, err := a.BorrowEntities(t.Context(), []app.BorrowEntityRequest{
		{DisplayName: "Drill", ParentPath: "Shelf", StatusContext: "Alice", ActorID: "test"},
	})
	require.NoError(t, err)

	drill, err := a.LookupEntityByPath(t.Context(), "Shelf:Drill")
	require.NoError(t, err)
	_, err = a.MarkReturned(t.Context(), []app.ChangeStatusRequest{
		{EntityID: drill.EntityID, ActorID: "test"},
	})
	require.NoError(t, err)

	entities, err := a.ListEntities(t.Context())
	require.NoError(t, err)
	for _, e := range entities {
		if e.FullPathDisplay == "Shelf:Drill" {
			assert.Equal(t, inventory.EntityStatusRemoved, e.Status)
		}
	}
}
