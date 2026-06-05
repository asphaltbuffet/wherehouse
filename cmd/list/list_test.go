package listcmd_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	listcmd "github.com/asphaltbuffet/wherehouse/cmd/list"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// seedThree creates Garage (place) → Garage:Toolbox (container) → Garage:Toolbox:Wrench (leaf).
func seedThree(t *testing.T, a *app.App) {
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

func TestRunList_ReturnsAll(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedThree(t, a)

	cmd := listcmd.NewListCmd(a)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, stdout.String(), "Garage")
	assert.Contains(t, stdout.String(), "Garage:Toolbox")
}

func TestRunList_FilterByType(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedThree(t, a)

	cmd := listcmd.NewListCmd(a)
	cmd.SetArgs([]string{"--type", "container"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())
	assert.NotContains(t, stdout.String(), "place")
	assert.Contains(t, stdout.String(), "Garage:Toolbox")
}

func TestRunList_FilterByUnderPath(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedThree(t, a)

	cmd := listcmd.NewListCmd(a)
	cmd.SetArgs([]string{"--under", "Garage:Toolbox"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())
	assert.NotContains(t, stdout.String(), "Garage\n")
	assert.NotContains(t, stdout.String(), "Garage:Toolbox\n")
	assert.Contains(t, stdout.String(), "Garage:Toolbox:Wrench")
}

func TestRunList_EmptyDB_ReturnsNoOutput(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := listcmd.NewListCmd(a)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())
	assert.Empty(t, stdout.String())
}
