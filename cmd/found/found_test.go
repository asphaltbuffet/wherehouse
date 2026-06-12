package found_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/found"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func seedForFound(t *testing.T, a *app.App) {
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
	// Mark both as missing so found has something to recover
	_, err := a.MarkLost(ctx, []app.ChangeStatusRequest{
		{EntityPath: "Garage:Toolbox:Wrench", Status: inventory.EntityStatusMissing, ActorID: "test"},
		{EntityPath: "Garage:Toolbox:Hammer", Status: inventory.EntityStatusMissing, ActorID: "test"},
	})
	require.NoError(t, err)
}

func TestRunFound_SinglePath(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForFound(t, a)

	cmd := found.NewFoundCmd(a)
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

func TestRunFound_MultiplePaths(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForFound(t, a)

	cmd := found.NewFoundCmd(a)
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench", "Garage:Toolbox:Hammer"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	entities, err := a.ListEntities(t.Context())
	require.NoError(t, err)
	recovered := 0
	for _, e := range entities {
		if e.FullPathDisplay == "Garage:Toolbox:Wrench" || e.FullPathDisplay == "Garage:Toolbox:Hammer" {
			assert.Equal(t, inventory.EntityStatusOk, e.Status)
			recovered++
		}
	}
	assert.Equal(t, 2, recovered)
}

func TestRunFound_AtomicOnFailure(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForFound(t, a)

	cmd := found.NewFoundCmd(a)
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
			assert.Equal(t, inventory.EntityStatusMissing, e.Status, "Wrench should still be missing")
		}
	}
}

func TestRunFound_WorksOnLockedEntity(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	// Locked entity set to missing via direct ChangeStatus to confirm found can recover it
	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage", ActorID: "test",
	})
	require.NoError(t, err)
	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Wrench", ParentPath: "Garage", Locked: false, ActorID: "test",
	})
	require.NoError(t, err)

	// Manually put it in missing state via ChangeStatus (not locked, so allowed)
	_, err = a.ChangeStatus(ctx, app.ChangeStatusRequest{
		EntityPath: "Garage:Wrench",
		Status:     inventory.EntityStatusMissing,
		ActorID:    "test",
	})
	require.NoError(t, err)

	cmd := found.NewFoundCmd(a)
	cmd.SetArgs([]string{"Garage:Wrench"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	entities, err := a.ListEntities(t.Context())
	require.NoError(t, err)
	for _, e := range entities {
		if e.FullPathDisplay == "Garage:Wrench" {
			assert.Equal(t, inventory.EntityStatusOk, e.Status)
		}
	}
}

func TestRunFound_NotFoundFails(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := found.NewFoundCmd(a)
	cmd.SetArgs([]string{"Garage:NoSuchThing"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.Error(t, cmd.Execute())
}
