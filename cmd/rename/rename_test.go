package rename_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/rename"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func seedForRename(t *testing.T, a *app.App) {
	t.Helper()
	ctx := t.Context()
	for _, tc := range []struct {
		name   string
		parent string
		et     inventory.EntityType
	}{
		{"Garage", "", inventory.EntityTypePlace},
		{"OldName", "Garage", inventory.EntityTypeContainer},
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

func TestRunRename_HappyPath(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForRename(t, a)

	cmd := rename.NewRenameCmd(a)
	cmd.SetArgs([]string{"Garage:OldName", "--to", "NewName"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	entities, err := a.ListEntities(t.Context())
	require.NoError(t, err)
	var found bool
	for _, e := range entities {
		if e.FullPathDisplay == "Garage:NewName" {
			found = true
		}
	}
	assert.True(t, found, "entity should be at Garage:NewName after rename")
}

func TestRunRename_PropagatesError(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := rename.NewRenameCmd(a)
	cmd.SetArgs([]string{"Garage:Missing", "--to", "X"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	require.Error(t, err)
}
