package lost_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/lost"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func seedForLost(t *testing.T, a *app.App) {
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

func TestRunLost_SinglePath(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForLost(t, a)

	cmd := lost.NewLostCmd(a)
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	entities, err := a.ListEntities(t.Context())
	require.NoError(t, err)
	for _, e := range entities {
		if e.FullPathDisplay == "Garage:Toolbox:Wrench" {
			assert.Equal(t, inventory.EntityStatusMissing, e.Status)
		}
	}
}

func TestRunLost_MultiplePaths(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForLost(t, a)

	cmd := lost.NewLostCmd(a)
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench", "Garage:Toolbox:Hammer"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	entities, err := a.ListEntities(t.Context())
	require.NoError(t, err)
	missing := 0
	for _, e := range entities {
		if e.FullPathDisplay == "Garage:Toolbox:Wrench" || e.FullPathDisplay == "Garage:Toolbox:Hammer" {
			assert.Equal(t, inventory.EntityStatusMissing, e.Status)
			missing++
		}
	}
	assert.Equal(t, 2, missing)
}

func TestRunLost_AtomicOnFailure(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForLost(t, a)

	cmd := lost.NewLostCmd(a)
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
			assert.Equal(t, inventory.EntityStatusOk, e.Status, "Wrench should not have been marked missing")
		}
	}
}

func TestRunLost_LockedEntityFails(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	// Create a locked entity directly
	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage", ActorID: "test",
	})
	require.NoError(t, err)
	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Wrench", ParentPath: "Garage", Locked: true, ActorID: "test",
	})
	require.NoError(t, err)

	cmd := lost.NewLostCmd(a)
	cmd.SetArgs([]string{"Garage:Wrench"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.Error(t, cmd.Execute())
}

func TestRunLost_NotFoundFails(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := lost.NewLostCmd(a)
	cmd.SetArgs([]string{"Garage:NoSuchThing"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.Error(t, cmd.Execute())
}
