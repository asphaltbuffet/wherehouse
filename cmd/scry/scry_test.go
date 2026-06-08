package scry_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/scry"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func TestRunScry_NoArg_ListsAll(t *testing.T) {
	a := apptesting.OpenApp(t)
	_, err := a.CreateEntity(t.Context(), app.CreateEntityRequest{
		DisplayName: "Garage",
		EntityType:  inventory.EntityTypePlace,
		ActorID:     "test",
	})
	require.NoError(t, err)

	cmd := scry.NewScryCmd(a)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, stdout.String(), "Garage")
}

func TestRunScry_WithArg_CallsFindEntities(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage",
		EntityType:  inventory.EntityTypePlace,
		ActorID:     "test",
	})
	require.NoError(t, err)
	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Toolbox",
		EntityType:  inventory.EntityTypeContainer,
		ParentPath:  "Garage",
		ActorID:     "test",
	})
	require.NoError(t, err)

	cmd := scry.NewScryCmd(a)
	cmd.SetArgs([]string{"toolbox"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, stdout.String(), "Garage:Toolbox")
}

func TestRunScry_EmptyDB_NoOutput(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := scry.NewScryCmd(a)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())
	assert.Empty(t, stdout.String())
}

func TestRunScry_WithArg_Verbose_ShowsDistance(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage",
		EntityType:  inventory.EntityTypePlace,
		ActorID:     "test",
	})
	require.NoError(t, err)

	cmd := scry.NewScryCmd(a)
	cmd.SetArgs([]string{"-v", "garage"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, stdout.String(), "dist:")
}

func TestRunScry_NoArg_Verbose_NoDistance(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage",
		EntityType:  inventory.EntityTypePlace,
		ActorID:     "test",
	})
	require.NoError(t, err)

	cmd := scry.NewScryCmd(a)
	cmd.SetArgs([]string{"-v"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())
	assert.NotContains(t, stdout.String(), "dist:")
}
