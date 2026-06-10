package web_test

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/web"
)

// errTest is a sentinel error used across handler tests.
var errTest = errors.New("test error")

// fakeApp satisfies web.App for tests.
type fakeApp struct {
	entities     []app.EntityResult
	history      []app.HistoryResult
	err          error
	renameResult app.EntityResult
	renameErr    error
	statusErr    error
	findResults  []app.FindResult
	findErr      error
}

func (f *fakeApp) ListEntities(_ context.Context) ([]app.EntityResult, error) {
	return f.entities, f.err
}

func (f *fakeApp) GetEntityByID(_ context.Context, entityID string) (app.EntityResult, error) {
	if f.err != nil {
		return app.EntityResult{}, f.err
	}
	for _, e := range f.entities {
		if e.EntityID == entityID {
			return e, nil
		}
	}
	return app.EntityResult{}, app.ErrNotFound
}

func (f *fakeApp) GetChildren(_ context.Context, parentID string) ([]app.EntityResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []app.EntityResult
	for _, e := range f.entities {
		if e.EntityID == parentID+"_child" {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeApp) GetHistory(_ context.Context, _ app.GetHistoryRequest) ([]app.HistoryResult, error) {
	return f.history, f.err
}

func (f *fakeApp) CreateEntity(_ context.Context, _ app.CreateEntityRequest) (app.EntityResult, error) {
	return app.EntityResult{}, f.err
}

func (f *fakeApp) RenameEntity(_ context.Context, _ app.RenameEntityRequest) (app.EntityResult, error) {
	return f.renameResult, f.renameErr
}

func (f *fakeApp) ChangeStatus(_ context.Context, _ app.ChangeStatusRequest) (app.EntityResult, error) {
	return app.EntityResult{}, f.statusErr
}

func (f *fakeApp) FindEntities(_ context.Context, _ app.FindEntitiesRequest) ([]app.FindResult, error) {
	return f.findResults, f.findErr
}

func newTestServer(t *testing.T, a web.App) *httptest.Server {
	t.Helper()
	srv, err := web.New(web.Config{
		App:    a,
		Bind:   "127.0.0.1",
		Port:   0,
		Output: io.Discard,
	})
	require.NoError(t, err)
	return httptest.NewServer(srv.Handler())
}
