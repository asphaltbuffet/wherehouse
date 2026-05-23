package remove_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/remove"
	"github.com/asphaltbuffet/wherehouse/internal/app"
)

type fakeRemoveApp struct {
	req app.RemoveEntityRequest
	err error
}

func (f *fakeRemoveApp) RemoveEntity(_ context.Context, req app.RemoveEntityRequest) error {
	f.req = req
	return f.err
}

func TestRunRemove_HappyPath(t *testing.T) {
	t.Parallel()
	fake := &fakeRemoveApp{}
	cmd := remove.NewRemoveCmd(fake)
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())
	assert.Equal(t, "Garage:Toolbox:Wrench", fake.req.EntityPath)
}

func TestRunRemove_WithNote(t *testing.T) {
	t.Parallel()
	fake := &fakeRemoveApp{}
	cmd := remove.NewRemoveCmd(fake)
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench", "--note", "broken"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())
	assert.Equal(t, "broken", fake.req.Note)
}

func TestRunRemove_PropagatesError(t *testing.T) {
	t.Parallel()
	fake := &fakeRemoveApp{err: errors.New("not found")}
	cmd := remove.NewRemoveCmd(fake)
	cmd.SetArgs([]string{"Garage:Missing"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
