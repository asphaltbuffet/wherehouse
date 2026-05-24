package add_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/add"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

type fakeAddApp struct {
	gotReq   app.CreateEntityRequest
	response app.EntityResult
	err      error
}

func (f *fakeAddApp) CreateEntity(_ context.Context, req app.CreateEntityRequest) (app.EntityResult, error) {
	f.gotReq = req
	return f.response, f.err
}

func TestRunAdd_HappyPath(t *testing.T) {
	t.Parallel()

	fake := &fakeAddApp{
		response: app.EntityResult{
			EntityID:        "abc1234567",
			DisplayName:     "Wrench",
			FullPathDisplay: "Garage:Toolbox:Wrench",
			EntityType:      inventory.EntityTypeLeaf,
		},
	}

	cmd := add.NewAddCmd(fake)
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench", "--type", "leaf"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	require.NoError(t, cmd.Execute())
	assert.Equal(t, "Wrench", fake.gotReq.DisplayName)
	assert.Equal(t, "Garage:Toolbox", fake.gotReq.ParentPath)
	assert.Equal(t, inventory.EntityTypeLeaf, fake.gotReq.EntityType)
}

func TestRunAdd_PropagatesAppError(t *testing.T) {
	t.Parallel()

	fake := &fakeAddApp{err: errors.New("boom")}
	cmd := add.NewAddCmd(fake)
	cmd.SetArgs([]string{"Garage:Wrench"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}
