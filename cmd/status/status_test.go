package status_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/status"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func seedForStatus(t *testing.T, a *app.App) {
	t.Helper()
	ctx := t.Context()
	for _, tc := range []struct {
		name   string
		parent string
	}{
		{"Garage", ""},
		{"Toolbox", "Garage"},
		{"Wrench", "Garage:Toolbox"},
	} {
		_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: tc.name, ParentPath: tc.parent,
			ActorID: "test",
		})
		require.NoError(t, err)
	}
}

func TestRunStatus_ShowsCurrentStatus(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForStatus(t, a)

	var stdout bytes.Buffer
	cmd := status.NewStatusCmd(a)
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	assert.Contains(t, stdout.String(), "ok")
}

func TestRunStatus_ShowsRemovedEntity(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForStatus(t, a)
	wrench, err := a.LookupEntityByPath(t.Context(), "Garage:Toolbox:Wrench")
	require.NoError(t, err)
	err = a.RemoveEntity(t.Context(), app.RemoveEntityRequest{
		EntityID: wrench.EntityID, ActorID: "test",
	})
	require.NoError(t, err)

	var stdout bytes.Buffer
	cmd := status.NewStatusCmd(a)
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	assert.Contains(t, stdout.String(), "removed")
}

func TestRunStatus_MultipleMatches_RankedByEventID(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForStatus(t, a)

	// Remove the wrench, then re-add it — two entities ever at this path
	wrench, err := a.LookupEntityByPath(t.Context(), "Garage:Toolbox:Wrench")
	require.NoError(t, err)
	err = a.RemoveEntity(t.Context(), app.RemoveEntityRequest{
		EntityID: wrench.EntityID, ActorID: "test",
	})
	require.NoError(t, err)
	_, err = a.CreateEntity(t.Context(), app.CreateEntityRequest{
		DisplayName: "Wrench", ParentPath: "Garage:Toolbox", ActorID: "test",
	})
	require.NoError(t, err)

	var stdout bytes.Buffer
	cmd := status.NewStatusCmd(a)
	cmd.SetContext(context.WithValue(t.Context(), config.ConfigKey, apptesting.NewTestConfig(t, apptesting.WithJSON())))
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	var results []app.StatusOutput
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &results))
	require.Len(t, results, 2)
	assert.Equal(t, inventory.EntityStatusOk, results[0].Status)
	assert.Equal(t, inventory.EntityStatusRemoved, results[1].Status)
}

func TestRunStatus_NotFound_ReturnsError(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := status.NewStatusCmd(a)
	cmd.SetArgs([]string{"Garage:NoSuchThing"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.Error(t, cmd.Execute())
}

func TestRunStatus_JSON(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForStatus(t, a)

	var stdout bytes.Buffer
	cmd := status.NewStatusCmd(a)
	cmd.SetContext(context.WithValue(t.Context(), config.ConfigKey, apptesting.NewTestConfig(t, apptesting.WithJSON())))
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	var results []app.StatusOutput
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &results))
	require.Len(t, results, 1)
	assert.Equal(t, "Garage:Toolbox:Wrench", results[0].Path)
	assert.Equal(t, inventory.EntityStatusOk, results[0].Status)
}
