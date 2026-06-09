package move_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/move"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

func seedForMove(t *testing.T, a *app.App) {
	t.Helper()
	ctx := t.Context()
	for _, tc := range []struct {
		name   string
		parent string
	}{
		{"Garage", ""},
		{"Workshop", ""},
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

func TestRunMove_HappyPath(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForMove(t, a)

	cmd := move.NewMoveCmd(a)
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench", "--to", "Workshop"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	entities, err := a.ListEntities(t.Context())
	require.NoError(t, err)
	var found bool
	for _, e := range entities {
		if e.FullPathDisplay == "Workshop:Wrench" {
			found = true
		}
	}
	assert.True(t, found, "Wrench should be at Workshop:Wrench after move")
}

func TestRunMove_PropagatesError(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForMove(t, a)

	cmd := move.NewMoveCmd(a)
	// Entity path does not exist — app should return not-found error
	cmd.SetArgs([]string{"Garage:Toolbox:DoesNotExist", "--to", "Workshop"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestRunMove_Quiet_SuppressesSuccess(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForMove(t, a)
	cmd := move.NewMoveCmd(a)
	cmd.SetContext(
		context.WithValue(t.Context(), config.ConfigKey, apptesting.NewTestConfig(t, apptesting.WithQuiet())),
	)
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench", "--to", "Workshop"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	require.NoError(t, cmd.Execute())
	assert.Empty(t, stdout.String())
	assert.Empty(t, stderr.String())
}
