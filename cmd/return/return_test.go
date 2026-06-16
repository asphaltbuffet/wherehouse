package returncmd_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	returncmd "github.com/asphaltbuffet/wherehouse/cmd/return"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func seedForReturn(t *testing.T, a *app.App) {
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
	wrench, err := a.LookupEntityByPath(ctx, "Garage:Toolbox:Wrench")
	require.NoError(t, err)
	hammer, err := a.LookupEntityByPath(ctx, "Garage:Toolbox:Hammer")
	require.NoError(t, err)
	_, err = a.MarkLoaned(ctx, []app.ChangeStatusRequest{
		{EntityID: wrench.EntityID, Status: inventory.EntityStatusLoaned, ActorID: "test"},
		{EntityID: hammer.EntityID, Status: inventory.EntityStatusLoaned, ActorID: "test"},
	})
	require.NoError(t, err)
}

func TestRunReturn_SinglePath(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForReturn(t, a)

	cmd := returncmd.NewReturnCmd(a)
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	entities, err := a.ListEntities(t.Context())
	require.NoError(t, err)
	for _, e := range entities {
		if e.FullPathDisplay == "Garage:Toolbox:Wrench" {
			assert.Equal(t, inventory.EntityStatusOk, e.Status)
		}
	}
}

func TestRunReturn_MultiplePaths(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForReturn(t, a)

	cmd := returncmd.NewReturnCmd(a)
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench", "Garage:Toolbox:Hammer"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	entities, err := a.ListEntities(t.Context())
	require.NoError(t, err)
	returned := 0
	for _, e := range entities {
		if e.FullPathDisplay == "Garage:Toolbox:Wrench" || e.FullPathDisplay == "Garage:Toolbox:Hammer" {
			assert.Equal(t, inventory.EntityStatusOk, e.Status)
			returned++
		}
	}
	assert.Equal(t, 2, returned)
}

func TestRunReturn_AtomicOnFailure(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForReturn(t, a)

	cmd := returncmd.NewReturnCmd(a)
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench", "Garage:Toolbox:NoSuchThing"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	require.Error(t, err)

	entities, err := a.ListEntities(t.Context())
	require.NoError(t, err)
	for _, e := range entities {
		if e.FullPathDisplay == "Garage:Toolbox:Wrench" {
			assert.Equal(t, inventory.EntityStatusLoaned, e.Status, "Wrench should still be loaned")
		}
	}
}

func TestRunReturn_NotFoundFails(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := returncmd.NewReturnCmd(a)
	cmd.SetArgs([]string{"Garage:NoSuchThing"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.Error(t, cmd.Execute())
}
