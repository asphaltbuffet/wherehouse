package remove_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/remove"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func seedForRemove(t *testing.T, a *app.App) {
	t.Helper()
	ctx := t.Context()
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

func TestRunRemove_HappyPath(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForRemove(t, a)

	cmd := remove.NewRemoveCmd(a)
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	entities, err := a.ListEntities(t.Context())
	require.NoError(t, err)
	for _, e := range entities {
		if e.FullPathDisplay == "Garage:Toolbox:Wrench" {
			assert.Equal(t, inventory.EntityStatusRemoved, e.Status, "Wrench should be removed")
		}
	}
}

func TestRunRemove_WithNote(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForRemove(t, a)

	cmd := remove.NewRemoveCmd(a)
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench", "--note", "broken"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	// Verify entity is removed (note stored on event, not on projection)
	entities, err := a.ListEntities(t.Context())
	require.NoError(t, err)
	for _, e := range entities {
		if e.FullPathDisplay == "Garage:Toolbox:Wrench" {
			assert.Equal(t, inventory.EntityStatusRemoved, e.Status)
		}
	}
}

func TestRunRemove_PropagatesError(t *testing.T) {
	a := apptesting.OpenApp(t)
	// No entities seeded — path not found
	cmd := remove.NewRemoveCmd(a)
	cmd.SetArgs([]string{"Garage:Missing"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	require.Error(t, err)
}
