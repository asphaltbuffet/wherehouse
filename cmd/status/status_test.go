package status_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/status"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

type fakeStatusApp struct {
	req app.ChangeStatusRequest
	err error
}

func (f *fakeStatusApp) ChangeStatus(_ context.Context, req app.ChangeStatusRequest) error {
	f.req = req
	return f.err
}

func TestRunStatus_HappyPath(t *testing.T) {
	t.Parallel()
	fake := &fakeStatusApp{}
	cmd := status.NewStatusCmd(fake)
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench", "--set", "borrowed", "--note", "loaned to Bob"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())
	assert.Equal(t, "Garage:Toolbox:Wrench", fake.req.EntityPath)
	assert.Equal(t, inventory.EntityStatusBorrowed, fake.req.Status)
	assert.Equal(t, "loaned to Bob", fake.req.StatusContext)
}

func TestRunStatus_InvalidStatus(t *testing.T) {
	t.Parallel()
	fake := &fakeStatusApp{}
	cmd := status.NewStatusCmd(fake)
	cmd.SetArgs([]string{"Garage:Wrench", "--set", "invalid-status"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestRunStatus_PropagatesError(t *testing.T) {
	t.Parallel()
	fake := &fakeStatusApp{err: errors.New("entity not found")}
	cmd := status.NewStatusCmd(fake)
	cmd.SetArgs([]string{"Garage:Missing", "--set", "ok"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "entity not found")
}
