package use_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/use"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

func seed(t *testing.T, a *app.App) {
	t.Helper()
	for _, tc := range []struct{ name, parent string }{
		{"Garage", ""}, {"Deck", ""}, {"Drill", "Garage"},
	} {
		_, err := a.CreateEntity(t.Context(), app.CreateEntityRequest{
			DisplayName: tc.name, ParentID: apptesting.MustLookupID(t, a, tc.parent), ActorID: "test",
		})
		require.NoError(t, err)
	}
}

func run(t *testing.T, a *app.App, args ...string) (string, error) {
	t.Helper()
	return execute(t, use.NewUseCmd(a), args)
}

func runJSON(t *testing.T, a *app.App, args ...string) (string, error) {
	t.Helper()
	cmd := use.NewUseCmd(a)
	cmd.SetContext(context.WithValue(t.Context(), config.ConfigKey, apptesting.NewTestConfig(t, apptesting.WithJSON())))
	return execute(t, cmd, args)
}

func execute(t *testing.T, cmd *cobra.Command, args []string) (string, error) {
	t.Helper()
	cmd.SetArgs(args)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	return out.String(), err
}

func TestUse_HappyPath(t *testing.T) {
	a := apptesting.OpenApp(t)
	seed(t, a)
	_, err := run(t, a, "Garage:Drill", "--to", "Deck", "--note", "deck project")
	require.NoError(t, err)
	e, err := a.LookupEntityByPath(t.Context(), "Deck:Drill")
	require.NoError(t, err)
	assert.True(t, e.IsDisplaced())
}

func TestUse_JSONIncludesHome(t *testing.T) {
	a := apptesting.OpenApp(t)
	seed(t, a)
	out, err := runJSON(t, a, "Garage:Drill", "--to", "Deck")
	require.NoError(t, err)
	var got app.MoveOutput
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	assert.Equal(t, "Deck:Drill", got.Path)
	assert.Equal(t, "Garage", got.Home)
}

func TestUse_ToHomeRejected(t *testing.T) {
	a := apptesting.OpenApp(t)
	seed(t, a)
	_, err := run(t, a, "Garage:Drill", "--to", "Garage")
	require.ErrorIs(t, err, app.ErrDestinationIsHome)
}

func TestUse_RequiresTo(t *testing.T) {
	a := apptesting.OpenApp(t)
	seed(t, a)
	_, err := run(t, a, "Garage:Drill")
	require.Error(t, err)
}
