package info_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	infocmd "github.com/asphaltbuffet/wherehouse/cmd/info"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

func runInfo(t *testing.T, a *app.App, args ...string) string {
	t.Helper()
	cmd := infocmd.NewInfoCmd(a)
	cmd.SetArgs(args)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())
	return out.String()
}

func TestInfoCmd_ShowsUnnamed(t *testing.T) {
	a := apptesting.OpenApp(t)
	out := runInfo(t, a)
	assert.Contains(t, out, "Name: (unnamed)")
}

func TestInfoCmd_ShowsName(t *testing.T) {
	a := apptesting.OpenApp(t)
	require.NoError(t, a.SetWherehouseName(t.Context(), "My Garage"))
	out := runInfo(t, a)
	assert.Contains(t, out, "Name: My Garage")
}

func TestInfoCmd_ShowsAllStatuses(t *testing.T) {
	a := apptesting.OpenApp(t)
	out := runInfo(t, a)
	assert.Contains(t, out, "OK:")
	assert.Contains(t, out, "MISSING:")
	assert.Contains(t, out, "BORROWED:")
	assert.Contains(t, out, "LOANED:")
	assert.Contains(t, out, "REMOVED:")
}

func TestInfoCmd_ShowsDatabasePath(t *testing.T) {
	a := apptesting.OpenApp(t)
	info, err := a.GetInfo(t.Context())
	require.NoError(t, err)
	out := runInfo(t, a)
	assert.Contains(t, out, "Database: "+info.DatabasePath)
}

func TestInfoCmd_CountsEntities(t *testing.T) {
	a := apptesting.OpenApp(t)
	_, err := a.CreateEntity(t.Context(), app.CreateEntityRequest{DisplayName: "Shelf", ActorID: "alice"})
	require.NoError(t, err)
	out := runInfo(t, a)
	assert.Contains(t, out, "OK:")
	assert.Contains(t, out, "1")
}

func TestInfoCmd_JSON(t *testing.T) {
	a := apptesting.OpenApp(t)
	require.NoError(t, a.SetWherehouseName(t.Context(), "Test House"))

	cmd := infocmd.NewInfoCmd(a)
	cmd.SetContext(context.WithValue(t.Context(), config.ConfigKey, apptesting.NewTestConfig(t, apptesting.WithJSON())))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	var result map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))
	assert.Equal(t, "Test House", result["name"])
	assert.NotEmpty(t, result["database"])
	entities, ok := result["entities"].(map[string]any)
	require.True(t, ok, "entities must be a map")
	assert.Contains(t, entities, "ok")
	assert.Contains(t, entities, "missing")
	assert.Contains(t, entities, "borrowed")
	assert.Contains(t, entities, "loaned")
	assert.Contains(t, entities, "removed")
}
