package scry_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/scry"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

type fakeScryApp struct {
	listResp []app.EntityResult
	listErr  error
	findResp []app.FindResult
	findErr  error
	findReq  app.FindEntitiesRequest
}

func (f *fakeScryApp) ListEntities(_ context.Context) ([]app.EntityResult, error) {
	return f.listResp, f.listErr
}

func (f *fakeScryApp) FindEntities(_ context.Context, req app.FindEntitiesRequest) ([]app.FindResult, error) {
	f.findReq = req
	return f.findResp, f.findErr
}

func TestRunScry_NoArg_ListsAll(t *testing.T) {
	t.Parallel()
	fake := &fakeScryApp{
		listResp: []app.EntityResult{
			{
				EntityID:        "a",
				FullPathDisplay: "Garage",
				EntityType:      inventory.EntityTypePlace,
				Status:          inventory.EntityStatusOk,
			},
		},
	}
	cmd := scry.NewScryCmd(fake)
	cmd.SetArgs([]string{})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, stdout.String(), "Garage")
}

func TestRunScry_WithArg_CallsFindEntities(t *testing.T) {
	t.Parallel()
	fake := &fakeScryApp{
		findResp: []app.FindResult{
			{
				Entity: app.EntityResult{
					EntityID:        "b",
					FullPathDisplay: "Garage:Toolbox",
					EntityType:      inventory.EntityTypeContainer,
					Status:          inventory.EntityStatusOk,
				},
				Distance: 0,
			},
		},
	}
	cmd := scry.NewScryCmd(fake)
	cmd.SetArgs([]string{"toolbox"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())
	assert.Equal(t, "toolbox", fake.findReq.Query)
	assert.Contains(t, stdout.String(), "Garage:Toolbox")
}

func TestRunScry_PropagatesListError(t *testing.T) {
	t.Parallel()
	fake := &fakeScryApp{listErr: errors.New("db down")}
	cmd := scry.NewScryCmd(fake)
	cmd.SetArgs([]string{})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db down")
}
