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
)

func TestRunAdd_HappyPath(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	// Pre-create parents so add can resolve the parent path
	for _, tc := range []struct {
		name   string
		parent string
	}{
		{"Garage", ""},
		{"Toolbox", "Garage"},
	} {
		_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: tc.name,
			ParentPath:  tc.parent,
			ActorID:     "test",
		})
		require.NoError(t, err)
	}

	cmd := add.NewAddCmd(a)
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	require.NoError(t, cmd.Execute())

	entities, err := a.ListEntities(ctx)
	require.NoError(t, err)

	var found bool
	for _, e := range entities {
		if e.FullPathDisplay == "Garage:Toolbox:Wrench" {
			found = true
		}
	}
	require.True(t, found, "Wrench entity should exist after add")
}

func TestRunAdd_LockedFlag(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	cmd := add.NewAddCmd(a)
	cmd.SetArgs([]string{"Garage", "--locked"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	result, err := a.GetEntityByPath(ctx, "Garage")
	require.NoError(t, err)
	assert.True(t, result.Locked)
	assert.False(t, result.Discrete)
}

func TestRunAdd_DiscreteFlag(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	cmd := add.NewAddCmd(a)
	cmd.SetArgs([]string{"Box of Nails", "--discrete"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	result, err := a.GetEntityByPath(ctx, "Box of Nails")
	require.NoError(t, err)
	assert.False(t, result.Locked)
	assert.True(t, result.Discrete)
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
	cmd.SetArgs([]string{"Garage"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	require.NoError(t, cmd.Execute())
	assert.Empty(t, stdout.String())
	assert.Empty(t, stderr.String())
}
