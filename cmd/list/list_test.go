package list_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/list"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

type fakeListApp struct {
	resp []app.EntityResult
	err  error
}

func (f *fakeListApp) ListEntities(_ context.Context) ([]app.EntityResult, error) {
	return f.resp, f.err
}

func TestRunList_ReturnsAll(t *testing.T) {
	t.Parallel()
	fake := &fakeListApp{
		resp: []app.EntityResult{
			{EntityID: "a", FullPathDisplay: "Garage", EntityType: inventory.EntityTypePlace, Status: inventory.EntityStatusOk},
			{EntityID: "b", FullPathDisplay: "Garage:Toolbox", EntityType: inventory.EntityTypeContainer, Status: inventory.EntityStatusOk},
		},
	}
	cmd := list.NewListCmd(fake)
	cmd.SetArgs([]string{})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, stdout.String(), "Garage")
	assert.Contains(t, stdout.String(), "Garage:Toolbox")
}

func TestRunList_FilterByType(t *testing.T) {
	t.Parallel()
	fake := &fakeListApp{
		resp: []app.EntityResult{
			{EntityID: "a", FullPathDisplay: "Garage", EntityType: inventory.EntityTypePlace, Status: inventory.EntityStatusOk},
			{EntityID: "b", FullPathDisplay: "Garage:Toolbox", EntityType: inventory.EntityTypeContainer, Status: inventory.EntityStatusOk},
		},
	}
	cmd := list.NewListCmd(fake)
	cmd.SetArgs([]string{"--type", "container"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())
	assert.NotContains(t, stdout.String(), "Garage\n")
	assert.Contains(t, stdout.String(), "Garage:Toolbox")
}

func TestRunList_FilterByUnderPath(t *testing.T) {
	t.Parallel()
	fake := &fakeListApp{
		resp: []app.EntityResult{
			{EntityID: "a", FullPathDisplay: "Garage", EntityType: inventory.EntityTypePlace, Status: inventory.EntityStatusOk},
			{EntityID: "b", FullPathDisplay: "Garage:Toolbox", EntityType: inventory.EntityTypeContainer, Status: inventory.EntityStatusOk},
			{EntityID: "c", FullPathDisplay: "Garage:Toolbox:Wrench", EntityType: inventory.EntityTypeLeaf, Status: inventory.EntityStatusOk},
		},
	}
	cmd := list.NewListCmd(fake)
	cmd.SetArgs([]string{"--under", "Garage:Toolbox"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())
	assert.NotContains(t, stdout.String(), "Garage\n")
	assert.NotContains(t, stdout.String(), "Garage:Toolbox\n")
	assert.Contains(t, stdout.String(), "Garage:Toolbox:Wrench")
}

func TestRunList_PropagatesError(t *testing.T) {
	t.Parallel()
	fake := &fakeListApp{err: errors.New("db error")}
	cmd := list.NewListCmd(fake)
	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}