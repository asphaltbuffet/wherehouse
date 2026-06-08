package add_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/add"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func TestRunAdd_HappyPath(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	// Pre-create parents so add can resolve the parent path
	for _, tc := range []struct {
		name   string
		parent string
		et     inventory.EntityType
	}{
		{"Garage", "", inventory.EntityTypePlace},
		{"Toolbox", "Garage", inventory.EntityTypeContainer},
	} {
		_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: tc.name,
			EntityType:  tc.et,
			ParentPath:  tc.parent,
			ActorID:     "test",
		})
		require.NoError(t, err)
	}

	cmd := add.NewAddCmd(a)
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench", "--type", "leaf"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	require.NoError(t, cmd.Execute())

	entities, err := a.ListEntities(ctx)
	require.NoError(t, err)

	var wrench *inventory.EntityType
	for _, e := range entities {
		if e.FullPathDisplay == "Garage:Toolbox:Wrench" {
			et := e.EntityType
			wrench = &et
		}
	}
	require.NotNil(t, wrench, "Wrench entity should exist after add")
	assert.Equal(t, inventory.EntityTypeLeaf, *wrench)
}

func TestRunAdd_PropagatesAppError(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := add.NewAddCmd(a)
	// Path with only a separator is invalid
	cmd.SetArgs([]string{":"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
}

func TestRunAdd_Quiet_SuppressesSuccess(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := add.NewAddCmd(a)
	cmd.SetContext(
		context.WithValue(t.Context(), config.ConfigKey, apptesting.NewTestConfig(t, apptesting.WithQuiet())),
	)
	cmd.SetArgs([]string{"Garage", "--type", "place"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	require.NoError(t, cmd.Execute())
	assert.Empty(t, stdout.String())
	assert.Empty(t, stderr.String())
}
