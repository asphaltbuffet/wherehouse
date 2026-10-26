package web_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
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
	lostCalls    int
	foundCalls   int
	findResults  []app.FindResult
	findErr      error
	lastActor    string // ActorID from the most recent mutating call

	lastCreateReq app.CreateEntityRequest // most recent CreateEntity request

	returnErr     error
	lastReturnReq app.ChangeStatusRequest
	onReturn      func(*fakeApp) // runs inside MarkReturned; simulates a concurrent writer

	loanErr     error
	loanCalls   int
	lastLoanReq app.ChangeStatusRequest
	onLoan      func(*fakeApp) // runs inside MarkLoaned; simulates a concurrent writer

	hasChildren      map[string]bool // parentID → HasLiveChildren result; missing key ⇒ false
	hasChildrenErr   error
	hasChildrenCalls int

	parentResults   []app.FindResult
	parentCalls     int
	lastParentQuery string

	borrowResults []app.EntityResult
	borrowErr     error
	borrowCalls   int
	lastBorrowReq app.BorrowEntityRequest
}

func (f *fakeApp) ListEntities(_ context.Context) ([]app.EntityResult, error) {
	return f.entities, f.err
}

func (f *fakeApp) GetEntityByID(_ context.Context, entityID string) (app.EntityResult, error) {
	if f.err != nil {
		return app.EntityResult{}, f.err
	}
	for _, e := range f.entities {
		if e.EntityID == entityID && e.Status != inventory.EntityStatusRemoved {
			return e, nil
		}
	}
	return app.EntityResult{}, app.ErrNotFound
}

func (f *fakeApp) GetEntityByIDAnyStatus(_ context.Context, entityID string) (app.EntityResult, error) {
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

func (f *fakeApp) CreateEntity(_ context.Context, req app.CreateEntityRequest) (app.EntityResult, error) {
	f.lastActor = req.ActorID
	f.lastCreateReq = req
	return app.EntityResult{}, f.err
}

func (f *fakeApp) RenameEntity(_ context.Context, req app.RenameEntityRequest) (app.EntityResult, error) {
	f.lastActor = req.ActorID
	return f.renameResult, f.renameErr
}

func (f *fakeApp) MarkLost(_ context.Context, reqs []app.ChangeStatusRequest) ([]app.EntityResult, error) {
	f.lostCalls++
	if len(reqs) > 0 {
		f.lastActor = reqs[0].ActorID
	}
	return nil, f.statusErr
}

func (f *fakeApp) MarkFound(_ context.Context, reqs []app.ChangeStatusRequest) ([]app.EntityResult, error) {
	f.foundCalls++
	if len(reqs) > 0 {
		f.lastActor = reqs[0].ActorID
	}
	return nil, f.statusErr
}

func (f *fakeApp) FindEntities(_ context.Context, _ app.FindEntitiesRequest) ([]app.FindResult, error) {
	return f.findResults, f.findErr
}

func (f *fakeApp) MarkReturned(_ context.Context, reqs []app.ChangeStatusRequest) ([]app.EntityResult, error) {
	if len(reqs) > 0 {
		f.lastActor = reqs[0].ActorID
		f.lastReturnReq = reqs[0]
	}
	if f.onReturn != nil {
		f.onReturn(f)
	}
	return nil, f.returnErr
}

func (f *fakeApp) MarkLoaned(_ context.Context, reqs []app.ChangeStatusRequest) ([]app.EntityResult, error) {
	f.loanCalls++
	if len(reqs) > 0 {
		f.lastActor = reqs[0].ActorID
		f.lastLoanReq = reqs[0]
	}
	if f.onLoan != nil {
		f.onLoan(f)
	}
	if f.loanErr != nil {
		return nil, f.loanErr
	}
	for _, req := range reqs {
		for i := range f.entities {
			if f.entities[i].EntityID == req.EntityID {
				f.entities[i].Status = inventory.EntityStatusLoaned
				f.entities[i].StatusContext = req.StatusContext
			}
		}
	}
	return nil, nil
}

func (f *fakeApp) HasLiveChildren(_ context.Context, parentID string) (bool, error) {
	f.hasChildrenCalls++
	if f.hasChildrenErr != nil {
		return false, f.hasChildrenErr
	}
	return f.hasChildren[parentID], nil
}

func (f *fakeApp) FindParentCandidates(_ context.Context, req app.FindEntitiesRequest) ([]app.FindResult, error) {
	f.parentCalls++
	f.lastParentQuery = req.Query
	return f.parentResults, f.findErr
}

func (f *fakeApp) BorrowEntities(_ context.Context, reqs []app.BorrowEntityRequest) ([]app.EntityResult, error) {
	f.borrowCalls++
	if len(reqs) > 0 {
		f.lastActor = reqs[0].ActorID
		f.lastBorrowReq = reqs[0]
	}
	if f.borrowErr != nil {
		return nil, f.borrowErr
	}
	f.entities = append(f.entities, f.borrowResults...)
	return f.borrowResults, nil
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

// newActorHandler builds a server that trusts header (may be "") for the actor.
func newActorHandler(t *testing.T, a web.App, header string) http.Handler {
	t.Helper()
	srv, err := web.New(web.Config{App: a, Output: io.Discard, ActorHeader: header})
	require.NoError(t, err)
	return srv.Handler()
}
