package web_test

import (
	"context"
	"errors"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/logging"
	"github.com/asphaltbuffet/wherehouse/internal/web"
)

// errTest is a sentinel error used across handler tests.
var errTest = errors.New("test error")

// staleText mirrors web.staleMessage (unexported) for body assertions.
const staleText = "Already changed elsewhere; showing current state."

// unavailableText mirrors web.unavailableMessage (unexported).
const unavailableText = "That action isn't available; showing current state."

// alert is msg as the detail panel's inline error renders it, so a blocked
// reason is told apart from the same text in a disabled button's tooltip.
func alert(msg string) string {
	return `role="alert">` + html.EscapeString(msg) + `</div>`
}

// recordingLogger satisfies logging.Logger and keeps the messages of Error calls.
type recordingLogger struct {
	mu     sync.Mutex
	errors []string
}

func (l *recordingLogger) Debug(string, ...any) {}
func (l *recordingLogger) Info(string, ...any)  {}
func (l *recordingLogger) Warn(string, ...any)  {}

func (l *recordingLogger) Error(msg string, _ ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errors = append(l.errors, msg)
}

func (l *recordingLogger) With(...any) logging.Logger { return l }

// fakeApp satisfies web.App for tests.
type fakeApp struct {
	entities       []app.EntityResult
	history        []app.HistoryResult
	err            error
	renameResult   app.EntityResult
	renameErr      error
	statusErr      error
	lostCalls      int
	foundCalls     int
	lastMissingReq app.ChangeStatusRequest // most recent MarkLost/MarkFound request
	findResults    []app.FindResult
	findErr        error
	lastActor      string // ActorID from the most recent mutating call

	lastCreateReq app.CreateEntityRequest // most recent CreateEntity request
	createResult  app.EntityResult

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

	homedHere    map[string]int // entityID → displaced entities homed here; missing key ⇒ 0
	actionsErr   error
	actionsCalls int

	children    map[string][]app.EntityResult // parentID → GetChildren result; nil map ⇒ legacy "_child" lookup
	childrenErr error

	parentResults   []app.FindResult
	rootLegal       bool  // answer of RootIsLegalDestination
	rootLegalErr    error // error of RootIsLegalDestination
	parentCalls     int
	lastParentQuery string

	borrowResults []app.EntityResult
	borrowErr     error
	borrowCalls   int
	lastBorrowReq app.BorrowEntityRequest

	attrErr     error
	attrCalls   int
	lastAttrReq app.SetAttributesRequest

	moveErr     error
	moveCalls   int
	lastMoveReq app.ReparentEntityRequest
	onMove      func(*fakeApp) // runs inside ReparentEntity; simulates a concurrent writer

	useErr     error
	useCalls   int
	lastUseReq app.UseEntityRequest

	lastMovingID   string         // entityID from the most recent FindMoveDestinations call
	lastRelocation app.Relocation // relocation from the most recent FindMoveDestinations call

	removeErr     error
	removeCalls   int
	lastRemoveReq app.RemoveEntityRequest
	onRemove      func(*fakeApp) // runs inside RemoveEntity; simulates a concurrent writer

	tagErr     error
	tagCalls   int
	lastTagReq app.TagEntityRequest
	onTag      func(*fakeApp) // runs inside TagEntity; simulates a concurrent writer

	allTags    []string // AllTags result
	allTagsErr error

	name    string // GetWherehouseName result
	nameErr error
}

func (f *fakeApp) RemoveEntity(_ context.Context, req app.RemoveEntityRequest) error {
	f.removeCalls++
	f.lastRemoveReq = req
	f.lastActor = req.ActorID
	if f.onRemove != nil {
		f.onRemove(f)
	}
	if f.removeErr != nil {
		return f.removeErr
	}
	for i := range f.entities {
		if f.entities[i].EntityID == req.EntityID && f.entities[i].Status != inventory.EntityStatusRemoved {
			f.entities[i].Status = inventory.EntityStatusRemoved
			return nil
		}
	}
	return app.ErrNotFound
}

// ReparentEntity records the request and, on success, re-parents the entity in
// f.entities so the handler's fresh re-read shows the new path.
func (f *fakeApp) ReparentEntity(_ context.Context, req app.ReparentEntityRequest) (app.RelocationResult, error) {
	f.moveCalls++
	f.lastMoveReq = req
	f.lastActor = req.ActorID
	if f.onMove != nil {
		f.onMove(f)
	}
	if f.moveErr != nil {
		return app.RelocationResult{}, f.moveErr
	}
	return f.relocate(req.EntityID, req.NewParentID)
}

// relocate re-parents entityID in f.entities, reporting the parent it had at
// write time (after any onMove writer), as the real transaction does.
func (f *fakeApp) relocate(entityID, newParentID string) (app.RelocationResult, error) {
	for i := range f.entities {
		if f.entities[i].EntityID != entityID {
			continue
		}
		from := f.entities[i].ParentID
		f.entities[i].ParentID = newParentID
		f.entities[i].FullPathDisplay = f.entities[i].DisplayName
		for _, p := range f.entities {
			if p.EntityID == newParentID {
				f.entities[i].FullPathDisplay = p.FullPathDisplay + ":" + f.entities[i].DisplayName
			}
		}
		return app.RelocationResult{Entity: f.entities[i], FromParentID: from}, nil
	}
	return app.RelocationResult{}, app.ErrNotFound
}

func (f *fakeApp) RootIsLegalDestination(_ context.Context, _ string, _ app.Relocation) (bool, error) {
	return f.rootLegal, f.rootLegalErr
}

func (f *fakeApp) FindMoveDestinations(
	_ context.Context, entityID string, relocation app.Relocation, req app.FindEntitiesRequest,
) ([]app.FindResult, error) {
	f.parentCalls++
	f.lastParentQuery = req.Query
	f.lastMovingID = entityID
	f.lastRelocation = relocation
	return f.parentResults, f.findErr
}

// SetAttributes records the request and, on success, applies it to f.entities
// so the handler's fresh re-read shows the new state.
func (f *fakeApp) SetAttributes(_ context.Context, reqs []app.SetAttributesRequest) ([]app.EntityResult, error) {
	f.attrCalls++
	f.lastAttrReq = reqs[0]
	f.lastActor = reqs[0].ActorID
	if f.attrErr != nil {
		return nil, f.attrErr
	}
	for i := range f.entities {
		if f.entities[i].EntityID != reqs[0].EntityID {
			continue
		}
		if reqs[0].Locked != nil {
			f.entities[i].Locked = *reqs[0].Locked
		}
		if reqs[0].Discrete != nil {
			f.entities[i].Discrete = *reqs[0].Discrete
			f.entities[i].CanAddChild = !*reqs[0].Discrete
		}
		return []app.EntityResult{f.entities[i]}, nil
	}
	return nil, app.ErrNotFound
}

func (f *fakeApp) ListEntities(_ context.Context) ([]app.EntityResult, error) {
	return f.entities, f.err
}

func (f *fakeApp) GetWherehouseName(_ context.Context) (string, error) {
	return f.name, f.nameErr
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
	if f.childrenErr != nil {
		return nil, f.childrenErr
	}
	if f.children != nil {
		return f.children[parentID], nil
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
	return f.createResult, f.err
}

func (f *fakeApp) RenameEntity(_ context.Context, req app.RenameEntityRequest) (app.EntityResult, error) {
	f.lastActor = req.ActorID
	return f.renameResult, f.renameErr
}

func (f *fakeApp) MarkLost(_ context.Context, reqs []app.ChangeStatusRequest) ([]app.EntityResult, error) {
	f.lostCalls++
	if len(reqs) > 0 {
		f.lastActor = reqs[0].ActorID
		f.lastMissingReq = reqs[0]
	}
	return nil, f.statusErr
}

func (f *fakeApp) MarkFound(_ context.Context, reqs []app.ChangeStatusRequest) ([]app.EntityResult, error) {
	f.foundCalls++
	if len(reqs) > 0 {
		f.lastActor = reqs[0].ActorID
		f.lastMissingReq = reqs[0]
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

func (f *fakeApp) EntityActions(_ context.Context, entityID string) (app.ActionSet, error) {
	f.actionsCalls++
	if f.actionsErr != nil {
		return nil, f.actionsErr
	}
	for _, e := range f.entities {
		if e.EntityID != entityID || e.Status == inventory.EntityStatusRemoved {
			continue
		}
		children := 0
		if f.hasChildren[entityID] {
			children = 1
		}
		return app.ActionsFor(app.EntityFacts{
			Path:               e.FullPathDisplay,
			Status:             e.Status,
			Locked:             e.Locked,
			Discrete:           e.Discrete,
			Displaced:          e.IsDisplaced(),
			LiveChildren:       children,
			DisplacedHomedHere: f.homedHere[entityID],
		}), nil
	}
	return nil, app.ErrNotFound
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

// readBody reads the full response body as a string.
func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(b)
}

// newActorHandler builds a server that trusts header (may be "") for the actor.
func newActorHandler(t *testing.T, a web.App, header string) http.Handler {
	t.Helper()
	srv, err := web.New(web.Config{App: a, Output: io.Discard, ActorHeader: header})
	require.NoError(t, err)
	return srv.Handler()
}

// UseEntity records the request and, on success, relocates the entity like
// ReparentEntity while keeping its home. A zero HomeID defaults to the current
// parent so older fixtures that never set HomeID read as "at home" before a
// use; a root-homed entity used in a test must set HomeID explicitly.
func (f *fakeApp) UseEntity(_ context.Context, req app.UseEntityRequest) (app.RelocationResult, error) {
	f.useCalls++
	f.lastUseReq = req
	f.lastActor = req.ActorID
	if f.useErr != nil {
		return app.RelocationResult{}, f.useErr
	}
	for i := range f.entities {
		if f.entities[i].EntityID == req.EntityID && f.entities[i].HomeID == "" {
			f.entities[i].HomeID = f.entities[i].ParentID
		}
	}
	return f.relocate(req.EntityID, req.NewParentID)
}

func (f *fakeApp) TagEntity(_ context.Context, req app.TagEntityRequest) error {
	f.tagCalls++
	f.lastActor = req.ActorID
	f.lastTagReq = req
	if f.onTag != nil {
		f.onTag(f)
	}
	if f.tagErr != nil {
		return f.tagErr
	}
	for i := range f.entities {
		if f.entities[i].EntityID != req.EntityID {
			continue
		}
		tags := slices.DeleteFunc(slices.Clone(f.entities[i].Tags), func(t string) bool {
			return slices.Contains(req.Remove, t)
		})
		for _, t := range req.Add {
			if c := inventory.CanonicalizeString(t); !slices.Contains(tags, c) {
				tags = append(tags, c)
			}
		}
		slices.Sort(tags)
		f.entities[i].Tags = tags
	}
	return nil
}

func (f *fakeApp) AllTags(context.Context) ([]string, error) {
	return slices.Clone(f.allTags), f.allTagsErr
}
