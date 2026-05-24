package rename_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/rename"
	"github.com/asphaltbuffet/wherehouse/internal/app"
)

type fakeRenameApp struct {
	req  app.RenameEntityRequest
	resp app.EntityResult
	err  error
}

func (f *fakeRenameApp) RenameEntity(_ context.Context, req app.RenameEntityRequest) (app.EntityResult, error) {
	f.req = req
	return f.resp, f.err
}

func TestRunRename_HappyPath(t *testing.T) {
	t.Parallel()
	fake := &fakeRenameApp{
		resp: app.EntityResult{
			EntityID:        "abc",
			DisplayName:     "NewName",
			FullPathDisplay: "Garage:NewName",
		},
	}
	cmd := rename.NewRenameCmd(fake)
	cmd.SetArgs([]string{"Garage:OldName", "--to", "NewName"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	require.NoError(t, cmd.Execute())
	assert.Equal(t, "Garage:OldName", fake.req.EntityPath)
	assert.Equal(t, "NewName", fake.req.NewName)
}

func TestRunRename_PropagatesError(t *testing.T) {
	t.Parallel()
	fake := &fakeRenameApp{err: errors.New("not found")}
	cmd := rename.NewRenameCmd(fake)
	cmd.SetArgs([]string{"Garage:Missing", "--to", "X"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
