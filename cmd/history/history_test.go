package history_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/history"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
)

func TestRunHistory_HappyPath(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage",
		ActorID:     "test",
	})
	require.NoError(t, err)
	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Wrench",
		ParentPath:  "Garage",
		ActorID:     "test",
	})
	require.NoError(t, err)

	cmd := history.NewHistoryCmd(a)
	cmd.SetArgs([]string{"Garage:Wrench"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, stdout.String(), "entity.created")
}

func TestRunHistory_PropagatesError(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := history.NewHistoryCmd(a)
	cmd.SetArgs([]string{"Garage:Missing"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	require.Error(t, err)
}
