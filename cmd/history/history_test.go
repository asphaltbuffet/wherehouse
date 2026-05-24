package history_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/history"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

type fakeHistoryApp struct {
	req  app.GetHistoryRequest
	resp []app.HistoryResult
	err  error
}

func (f *fakeHistoryApp) GetHistory(_ context.Context, req app.GetHistoryRequest) ([]app.HistoryResult, error) {
	f.req = req
	return f.resp, f.err
}

func TestRunHistory_HappyPath(t *testing.T) {
	t.Parallel()
	fake := &fakeHistoryApp{
		resp: []app.HistoryResult{
			{
				EventID:      1,
				EventType:    inventory.EntityCreatedEvent,
				TimestampUTC: "2026-05-22T00:00:00Z",
				ActorUserID:  "user@example.com",
			},
		},
	}
	cmd := history.NewHistoryCmd(fake)
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())
	assert.Equal(t, "Garage:Toolbox:Wrench", fake.req.EntityPath)
	assert.Contains(t, stdout.String(), "2026-05-22T00:00:00Z")
}

func TestRunHistory_PropagatesError(t *testing.T) {
	t.Parallel()
	fake := &fakeHistoryApp{err: errors.New("not found")}
	cmd := history.NewHistoryCmd(fake)
	cmd.SetArgs([]string{"Garage:Missing"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
