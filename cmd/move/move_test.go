package move_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/move"
	"github.com/asphaltbuffet/wherehouse/internal/app"
)

type fakeMoveApp struct {
	req  app.ReparentEntityRequest
	resp app.EntityResult
	err  error
}

func (f *fakeMoveApp) ReparentEntity(_ context.Context, req app.ReparentEntityRequest) (app.EntityResult, error) {
	f.req = req
	return f.resp, f.err
}

func TestRunMove_HappyPath(t *testing.T) {
	t.Parallel()
	fake := &fakeMoveApp{
		resp: app.EntityResult{EntityID: "abc", DisplayName: "Wrench", FullPathDisplay: "Workshop:Wrench"},
	}
	cmd := move.NewMoveCmd(fake)
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench", "--to", "Workshop"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())
	assert.Equal(t, "Garage:Toolbox:Wrench", fake.req.EntityPath)
	assert.Equal(t, "Workshop", fake.req.NewParentPath)
}

func TestRunMove_PropagatesError(t *testing.T) {
	t.Parallel()
	fake := &fakeMoveApp{err: errors.New("place entities cannot be moved")}
	cmd := move.NewMoveCmd(fake)
	cmd.SetArgs([]string{"Garage", "--to", "Workshop"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "place entities cannot be moved")
}
