package loan_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/loan"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func seedForLoan(t *testing.T, a *app.App) {
	t.Helper()
	ctx := t.Context()
	for _, tc := range []struct {
		name   string
		parent string
	}{
		{"Garage", ""},
		{"Toolbox", "Garage"},
		{"Wrench", "Garage:Toolbox"},
		{"Hammer", "Garage:Toolbox"},
	} {
		_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: tc.name, ParentPath: tc.parent,
			ActorID: "test",
		})
		require.NoError(t, err)
	}
}

func TestRunLoan_ToRequired(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForLoan(t, a)

	cmd := loan.NewLoanCmd(a)
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench"}) // missing --to
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.Error(t, cmd.Execute())

	// Entity must remain ok — the missing recipient should block the loan.
	entities, err := a.ListEntities(t.Context())
	require.NoError(t, err)
	for _, e := range entities {
		if e.FullPathDisplay == "Garage:Toolbox:Wrench" {
			assert.Equal(t, inventory.EntityStatusOk, e.Status)
		}
	}
}

func TestRunLoan_WithTo(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForLoan(t, a)

	cmd := loan.NewLoanCmd(a)
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench", "--to", "Alice"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	entities, err := a.ListEntities(t.Context())
	require.NoError(t, err)
	for _, e := range entities {
		if e.FullPathDisplay == "Garage:Toolbox:Wrench" {
			assert.Equal(t, inventory.EntityStatusLoaned, e.Status)
			assert.Equal(t, "Alice", e.StatusContext)
		}
	}
}

func TestRunLoan_MultiplePaths(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForLoan(t, a)

	cmd := loan.NewLoanCmd(a)
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench", "Garage:Toolbox:Hammer", "--to", "Bob"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	entities, err := a.ListEntities(t.Context())
	require.NoError(t, err)
	loaned := 0
	for _, e := range entities {
		if e.FullPathDisplay == "Garage:Toolbox:Wrench" || e.FullPathDisplay == "Garage:Toolbox:Hammer" {
			assert.Equal(t, inventory.EntityStatusLoaned, e.Status)
			assert.Equal(t, "Bob", e.StatusContext)
			loaned++
		}
	}
	assert.Equal(t, 2, loaned)
}

func TestRunLoan_AtomicOnFailure(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForLoan(t, a)

	cmd := loan.NewLoanCmd(a)
	// Second path does not exist — entire batch should be rolled back
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench", "Garage:Toolbox:NoSuchThing"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	require.Error(t, err)

	entities, err := a.ListEntities(t.Context())
	require.NoError(t, err)
	for _, e := range entities {
		if e.FullPathDisplay == "Garage:Toolbox:Wrench" {
			assert.Equal(t, inventory.EntityStatusOk, e.Status, "Wrench should not have been marked loaned")
		}
	}
}

func TestRunLoan_LockedEntityFails(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage", ActorID: "test",
	})
	require.NoError(t, err)
	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Wrench", ParentPath: "Garage", Locked: true, ActorID: "test",
	})
	require.NoError(t, err)

	cmd := loan.NewLoanCmd(a)
	cmd.SetArgs([]string{"Garage:Wrench", "--to", "Alice"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.Error(t, cmd.Execute())
}

func TestRunLoan_NotFoundFails(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := loan.NewLoanCmd(a)
	cmd.SetArgs([]string{"Garage:NoSuchThing"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.Error(t, cmd.Execute())
}
