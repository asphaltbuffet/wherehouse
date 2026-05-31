package status_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/status"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func seedForStatus(t *testing.T, a *app.App) {
	t.Helper()
	ctx := context.Background()
	for _, tc := range []struct {
		name   string
		parent string
		et     inventory.EntityType
	}{
		{"Garage", "", inventory.EntityTypePlace},
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

func TestRunStatus_HappyPath(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForStatus(t, a)

	cmd := status.NewStatusCmd(a)
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench", "--set", "borrowed", "--note", "loaned to Bob"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	entities, err := a.ListEntities(context.Background())
	require.NoError(t, err)
	for _, e := range entities {
		if e.FullPathDisplay == "Garage:Toolbox:Wrench" {
			assert.Equal(t, inventory.EntityStatusBorrowed, e.Status)
		}
	}
}

func TestRunStatus_InvalidStatus(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := status.NewStatusCmd(a)
	cmd.SetArgs([]string{"Garage:Wrench", "--set", "invalid-status"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestRunStatus_PropagatesError(t *testing.T) {
	a := apptesting.OpenApp(t)
	// No entities — path not found
	cmd := status.NewStatusCmd(a)
	cmd.SetArgs([]string{"Garage:Missing", "--set", "ok"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	require.Error(t, err)
}
