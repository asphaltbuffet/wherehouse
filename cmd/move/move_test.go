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
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func seedForMove(t *testing.T, a *app.App) {
	t.Helper()
	ctx := context.Background()
	for _, tc := range []struct {
		name   string
		parent string
		et     inventory.EntityType
	}{
		{"Garage", "", inventory.EntityTypePlace},
		{"Workshop", "", inventory.EntityTypePlace},
		{"Toolbox", "Garage", inventory.EntityTypeContainer},
		{"Wrench", "Garage:Toolbox", inventory.EntityTypeLeaf},
	} {
		_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: tc.name,
			EntityType:  tc.et,
			ParentPath:  tc.parent,
			ActorID:     "test",
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

	entities, err := a.ListEntities(context.Background())
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
