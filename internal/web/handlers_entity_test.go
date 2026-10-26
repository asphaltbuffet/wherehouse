package web_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/web"
)

func TestHandleEntityDetail_NotFound(t *testing.T) {
	ts := newTestServer(t, &fakeApp{entities: []app.EntityResult{}})
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/entities/nonexistent")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHandleEntityDetail_Found(t *testing.T) {
	history := []app.HistoryResult{
		{EventID: 1, TimestampUTC: "2025-01-01T00:00:00Z", ActorUserID: "alice"},
	}
	entities := []app.EntityResult{
		{EntityID: "xyz", DisplayName: "Hammer", CanonicalName: "garage:toolbox:hammer",
			Status: inventory.EntityStatusOk},
	}
	ts := newTestServer(t, &fakeApp{entities: entities, history: history})
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/entities/xyz")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	bs := string(body)
	assert.Contains(t, bs, "Hammer")
	assert.Contains(t, bs, "2025-01-01T00:00:00Z")
	assert.Contains(t, bs, "alice")
	assert.Contains(t, bs, `class="detail-actions"`)
	assert.Contains(t, bs, `Mark missing`)
	assert.Contains(t, bs, `Add child`)
	assert.NotContains(t, bs, `hx-get="/entities/xyz/edit/status"`)
}

func TestHandleEditNameForm_NotFound(t *testing.T) {
	ts := newTestServer(t, &fakeApp{entities: []app.EntityResult{}})
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/entities/missing/edit/name", nil)
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHandleEditNameForm_Found(t *testing.T) {
	entities := []app.EntityResult{
		{EntityID: "abc", DisplayName: "Old Name", FullPathDisplay: "Shelf:OldName",
			Status: inventory.EntityStatusOk},
	}
	ts := newTestServer(t, &fakeApp{entities: entities})
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/entities/abc/edit/name", nil)
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	bs := string(body)
	assert.Contains(t, bs, "Old Name")
	assert.Contains(t, bs, `hx-post="/entities/abc/edit/name"`)
}

func TestHandleEditName_Success(t *testing.T) {
	entities := []app.EntityResult{
		{EntityID: "abc", DisplayName: "Old Name", FullPathDisplay: "Shelf:OldName",
			Status: inventory.EntityStatusOk},
	}
	fa := &fakeApp{
		entities: entities,
		renameResult: app.EntityResult{
			EntityID:        "abc",
			DisplayName:     "New Name",
			FullPathDisplay: "Shelf:NewName",
			Status:          inventory.EntityStatusOk,
		},
	}
	ts := newTestServer(t, fa)
	defer ts.Close()

	form := strings.NewReader("display_name=New+Name")
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/entities/abc/edit/name", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), `id="entity-detail"`)
}

func TestHandleEditName_AppError(t *testing.T) {
	entities := []app.EntityResult{
		{EntityID: "abc", DisplayName: "Old Name", FullPathDisplay: "Shelf:OldName",
			Status: inventory.EntityStatusOk},
	}
	fa := &fakeApp{
		entities:  entities,
		renameErr: errTest,
	}
	ts := newTestServer(t, fa)
	defer ts.Close()

	form := strings.NewReader("display_name=New+Name")
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/entities/abc/edit/name", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "rename failed")
}

func TestHandleEditName_EmptyName(t *testing.T) {
	entities := []app.EntityResult{
		{EntityID: "abc", DisplayName: "Old Name", FullPathDisplay: "Shelf:OldName",
			Status: inventory.EntityStatusOk},
	}
	ts := newTestServer(t, &fakeApp{entities: entities})
	defer ts.Close()

	form := strings.NewReader("display_name=")
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/entities/abc/edit/name", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestBuildDetailData_Breadcrumbs(t *testing.T) {
	entities := []app.EntityResult{
		{EntityID: "g1", DisplayName: "Garage", FullPathDisplay: "Garage",
			Status: inventory.EntityStatusOk},
		{EntityID: "t1", DisplayName: "Toolbox", FullPathDisplay: "Garage:Toolbox",
			Status: inventory.EntityStatusOk},
		{EntityID: "h1", DisplayName: "Hammer", FullPathDisplay: "Garage:Toolbox:Hammer",
			Status: inventory.EntityStatusOk},
	}
	srv, err := web.New(web.Config{App: &fakeApp{entities: entities}, Bind: "127.0.0.1", Port: 0, Output: io.Discard})
	require.NoError(t, err)

	crumbs := web.BreadcrumbsForEntity(entities, "Garage:Toolbox:Hammer")
	require.Len(t, crumbs, 3)
	assert.Equal(t, "Garage", crumbs[0].Name)
	assert.Equal(t, "g1", crumbs[0].EntityID)
	assert.Equal(t, "Toolbox", crumbs[1].Name)
	assert.Equal(t, "t1", crumbs[1].EntityID)
	assert.Equal(t, "Hammer", crumbs[2].Name)
	assert.Empty(t, crumbs[2].EntityID) // last crumb has no link
	_ = srv
}

func TestHandleToggleMissing_OkToMissing(t *testing.T) {
	entities := []app.EntityResult{
		{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Garage:Hammer",
			Status: inventory.EntityStatusOk},
	}
	fa := &fakeApp{entities: entities}
	ts := newTestServer(t, fa)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/entities/abc/actions/toggle-missing", nil)
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), `id="entity-detail"`)
	assert.Equal(t, 1, fa.lostCalls, "ok → missing must go through MarkLost")
	assert.Zero(t, fa.foundCalls)
}

func TestHandleToggleMissing_MissingToOk(t *testing.T) {
	entities := []app.EntityResult{
		{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Garage:Hammer",
			Status: inventory.EntityStatusMissing},
	}
	fa := &fakeApp{entities: entities}
	ts := newTestServer(t, fa)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/entities/abc/actions/toggle-missing", nil)
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 1, fa.foundCalls, "missing → ok must go through MarkFound")
	assert.Zero(t, fa.lostCalls)
}

func TestHandleToggleMissing_LockedEntity_Forbidden(t *testing.T) {
	// locked entities cannot be marked missing — their location is fixed by assertion.
	entities := []app.EntityResult{
		{EntityID: "abc", DisplayName: "Garage", FullPathDisplay: "Garage",
			Locked: true, Status: inventory.EntityStatusOk},
	}
	ts := newTestServer(t, &fakeApp{entities: entities})
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/entities/abc/actions/toggle-missing", nil)
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestHandleToggleMissing_NotFound(t *testing.T) {
	ts := newTestServer(t, &fakeApp{entities: []app.EntityResult{}})
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/entities/missing/actions/toggle-missing", nil)
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHandleToggleMissing_BorrowedForbidden(t *testing.T) {
	entities := []app.EntityResult{
		{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Garage:Hammer",
			Status: inventory.EntityStatusBorrowed},
	}
	ts := newTestServer(t, &fakeApp{entities: entities})
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/entities/abc/actions/toggle-missing", nil)
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestHandleToggleMissing_AppError(t *testing.T) {
	entities := []app.EntityResult{
		{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Garage:Hammer",
			Status: inventory.EntityStatusOk},
	}
	ts := newTestServer(t, &fakeApp{entities: entities, statusErr: errTest})
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/entities/abc/actions/toggle-missing", nil)
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "status change failed")
}

func postHTMX(t *testing.T, ts *httptest.Server, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, ts.URL+path, nil)
	require.NoError(t, err)
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	return resp
}

func TestHandleReturn_LoanedRerendersInPlace(t *testing.T) {
	fake := &fakeApp{entities: []app.EntityResult{
		{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Hammer",
			Status: inventory.EntityStatusLoaned, StatusContext: "Bob"},
	}}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp := postHTMX(t, ts, "/entities/abc/actions/return")
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "abc", fake.lastReturnReq.EntityID)
	assert.Contains(t, string(body), `id="tree-badge-abc"`)
	assert.Empty(t, resp.Header.Get("Hx-Push-Url"))
	assert.Empty(t, resp.Header.Get("Hx-Trigger-After-Swap"))
}

type detachedEvent struct {
	EntityID          string `json:"entityId"`
	ParentID          string `json:"parentId"`
	ParentHasChildren bool   `json:"parentHasChildren"`
}

// detachedTrigger decodes the tree-node-detached payload from Hx-Trigger-After-Swap.
func detachedTrigger(t *testing.T, resp *http.Response) detachedEvent {
	t.Helper()
	raw := resp.Header.Get("Hx-Trigger-After-Swap")
	require.NotEmpty(t, raw, "Hx-Trigger-After-Swap missing")
	var trigger map[string]detachedEvent
	require.NoError(t, json.Unmarshal([]byte(raw), &trigger))
	ev, ok := trigger["tree-node-detached"]
	require.True(t, ok, "tree-node-detached key missing: %s", raw)
	return ev
}

func TestHandleReturn_BorrowedShowsParentAndDeletesNode(t *testing.T) {
	fake := &fakeApp{entities: []app.EntityResult{
		{EntityID: "g", DisplayName: "Garage", FullPathDisplay: "Garage", Status: inventory.EntityStatusOk},
		{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Garage:Drill", ParentID: "g",
			Status: inventory.EntityStatusBorrowed, StatusContext: "Alice"},
	}}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp := postHTMX(t, ts, "/entities/abc/actions/return")
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "/entities/g", resp.Header.Get("Hx-Push-Url"))
	assert.Contains(t, string(body), "<h2>Garage</h2>")
	assert.Contains(t, string(body), `id="tree-node-abc" hx-swap-oob="delete"`)
	assert.Equal(t, detachedEvent{EntityID: "abc", ParentID: "g", ParentHasChildren: false}, detachedTrigger(t, resp))
}

func TestHandleReturn_BorrowedSiblingRemainsKeepsParentExpandable(t *testing.T) {
	fake := &fakeApp{
		entities: []app.EntityResult{
			{EntityID: "g", DisplayName: "Garage", FullPathDisplay: "Garage", Status: inventory.EntityStatusOk},
			{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Garage:Drill", ParentID: "g",
				Status: inventory.EntityStatusBorrowed, StatusContext: "Alice"},
		},
		hasChildren: map[string]bool{"g": true},
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp := postHTMX(t, ts, "/entities/abc/actions/return")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, detachedEvent{EntityID: "abc", ParentID: "g", ParentHasChildren: true}, detachedTrigger(t, resp))
}

// A failed child count must not turn a committed return into an error page.
func TestHandleReturn_BorrowedCountErrorOmitsTrigger(t *testing.T) {
	fake := &fakeApp{
		entities: []app.EntityResult{
			{EntityID: "g", DisplayName: "Garage", FullPathDisplay: "Garage", Status: inventory.EntityStatusOk},
			{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Garage:Drill", ParentID: "g",
				Status: inventory.EntityStatusBorrowed, StatusContext: "Alice"},
		},
		hasChildrenErr: errTest,
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp := postHTMX(t, ts, "/entities/abc/actions/return")
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "<h2>Garage</h2>")
	assert.Contains(t, string(body), `id="tree-node-abc" hx-swap-oob="delete"`)
	assert.Empty(t, resp.Header.Get("Hx-Trigger-After-Swap"))
}

func TestHandleReturn_BorrowedRootShowsEmptyState(t *testing.T) {
	fake := &fakeApp{entities: []app.EntityResult{
		{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Drill",
			Status: inventory.EntityStatusBorrowed, StatusContext: "Alice"},
	}}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp := postHTMX(t, ts, "/entities/abc/actions/return")
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, "/", resp.Header.Get("Hx-Push-Url"))
	assert.Contains(t, string(body), "Select an item to see details.")
	assert.Contains(t, string(body), `id="tree-node-abc" hx-swap-oob="delete"`)
	assert.Equal(t, detachedEvent{EntityID: "abc", ParentID: ""}, detachedTrigger(t, resp))
	assert.Zero(t, fake.hasChildrenCalls, "root removal must not query child count")
}

func TestHandleReturn_OkForbidden(t *testing.T) {
	ts := newTestServer(t, &fakeApp{entities: []app.EntityResult{
		{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Hammer", Status: inventory.EntityStatusOk},
	}})
	defer ts.Close()

	resp := postHTMX(t, ts, "/entities/abc/actions/return")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// Review Focus #4: a second click after the borrowed entity was removed.
func TestHandleReturn_AlreadyReturnedNotFound(t *testing.T) {
	ts := newTestServer(t, &fakeApp{entities: []app.EntityResult{}})
	defer ts.Close()

	resp := postHTMX(t, ts, "/entities/abc/actions/return")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHandleReturn_HasChildrenShowsMessage(t *testing.T) {
	ts := newTestServer(t, &fakeApp{
		entities: []app.EntityResult{
			{EntityID: "abc", DisplayName: "Case", FullPathDisplay: "Case",
				Status: inventory.EntityStatusBorrowed, StatusContext: "Alice"},
		},
		returnErr: fmt.Errorf("return: %w", app.ErrHasChildren),
	})
	defer ts.Close()

	resp := postHTMX(t, ts, "/entities/abc/actions/return")
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "still contains other entities")
	assert.Empty(t, resp.Header.Get("Hx-Trigger-After-Swap"))
}

// Review Focus #1 (#292): a concurrent Return removed the borrowed entity
// between our detail read and our MarkReturned call.
func TestHandleReturn_ConcurrentBorrowedShowsRemoval(t *testing.T) {
	fake := &fakeApp{
		entities: []app.EntityResult{
			{EntityID: "g", DisplayName: "Garage", FullPathDisplay: "Garage", Status: inventory.EntityStatusOk},
			{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Garage:Drill", ParentID: "g",
				Status: inventory.EntityStatusBorrowed, StatusContext: "Alice"},
		},
		returnErr: fmt.Errorf("get entity %q: %w", "abc", app.ErrNotFound),
		onReturn:  func(f *fakeApp) { f.entities[1].Status = inventory.EntityStatusRemoved },
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp := postHTMX(t, ts, "/entities/abc/actions/return")
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "/entities/g", resp.Header.Get("Hx-Push-Url"))
	assert.Contains(t, string(body), "<h2>Garage</h2>")
	assert.Contains(t, string(body), `id="tree-node-abc" hx-swap-oob="delete"`)
	assert.NotContains(t, string(body), "return failed")
	assert.Equal(t, detachedEvent{EntityID: "abc", ParentID: "g"}, detachedTrigger(t, resp))
}

// Review Focus #2 (#292): a concurrent Return already set the loaned entity
// back to ok, so our MarkReturned fails with a plain "entity is ok" error.
func TestHandleReturn_ConcurrentLoanedShowsReturned(t *testing.T) {
	fake := &fakeApp{
		entities: []app.EntityResult{
			{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Hammer",
				Status: inventory.EntityStatusLoaned, StatusContext: "Bob"},
		},
		returnErr: errTest,
		onReturn:  func(f *fakeApp) { f.entities[0].Status = inventory.EntityStatusOk },
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp := postHTMX(t, ts, "/entities/abc/actions/return")
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), `id="tree-badge-abc"`)
	assert.NotContains(t, string(body), "return failed")
	assert.NotContains(t, string(body), "/actions/return")
	assert.Empty(t, resp.Header.Get("Hx-Push-Url"))
}

// Review Focus #3: a real failure leaves the entity returnable, so the user
// sees the error and can retry.
func TestHandleReturn_GenericFailureShowsError(t *testing.T) {
	ts := newTestServer(t, &fakeApp{
		entities: []app.EntityResult{
			{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Hammer",
				Status: inventory.EntityStatusLoaned, StatusContext: "Bob"},
		},
		returnErr: errTest,
	})
	defer ts.Close()

	resp := postHTMX(t, ts, "/entities/abc/actions/return")
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "return failed")
	assert.Contains(t, string(body), "/entities/abc/actions/return")
}

// Review Focus #4: the post-failure re-read itself fails.
func TestHandleReturn_RereadFailureServerError(t *testing.T) {
	ts := newTestServer(t, &fakeApp{
		entities: []app.EntityResult{
			{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Hammer",
				Status: inventory.EntityStatusLoaned, StatusContext: "Bob"},
		},
		returnErr: errTest,
		onReturn:  func(f *fakeApp) { f.err = errTest },
	})
	defer ts.Close()

	resp := postHTMX(t, ts, "/entities/abc/actions/return")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestEntityDetail_ReturnButton(t *testing.T) {
	tests := []struct {
		name        string
		entity      app.EntityResult
		wantButton  bool
		wantConfirm string
	}{
		{
			name: "borrowed confirms with lender",
			entity: app.EntityResult{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Drill",
				Status: inventory.EntityStatusBorrowed, StatusContext: "Alice"},
			wantButton:  true,
			wantConfirm: `hx-confirm="Return Drill to Alice? It will be removed from the inventory."`,
		},
		{
			name: "loaned has no confirm",
			entity: app.EntityResult{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Drill",
				Status: inventory.EntityStatusLoaned, StatusContext: "Bob"},
			wantButton: true,
		},
		{
			name: "ok has no return",
			entity: app.EntityResult{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Drill",
				Status: inventory.EntityStatusOk},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t, &fakeApp{entities: []app.EntityResult{tt.entity}})
			defer ts.Close()

			req, _ := http.NewRequest(http.MethodGet, ts.URL+"/entities/abc", nil)
			req.Header.Set("Hx-Request", "true")
			resp, err := ts.Client().Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)

			assert.Equal(t, tt.wantButton, strings.Contains(string(body), "/actions/return"))
			if tt.wantConfirm != "" {
				assert.Contains(t, string(body), tt.wantConfirm)
			} else {
				assert.NotContains(t, string(body), "hx-confirm")
			}
		})
	}
}

func TestEntityDetail_LoanButton(t *testing.T) {
	tests := []struct {
		name        string
		entity      app.EntityResult
		wantButton  bool
		wantEnabled bool
	}{
		{
			name: "ok is loanable",
			entity: app.EntityResult{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Drill",
				Status: inventory.EntityStatusOk},
			wantButton: true, wantEnabled: true,
		},
		{
			name: "missing is loanable (ADR 0024)",
			entity: app.EntityResult{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Drill",
				Status: inventory.EntityStatusMissing},
			wantButton: true, wantEnabled: true,
		},
		{
			name: "locked is disabled",
			entity: app.EntityResult{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Drill",
				Status: inventory.EntityStatusOk, Locked: true},
			wantButton: true,
		},
		{
			name: "loaned has no loan button",
			entity: app.EntityResult{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Drill",
				Status: inventory.EntityStatusLoaned, StatusContext: "Bob"},
		},
		{
			name: "borrowed has no loan button",
			entity: app.EntityResult{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Drill",
				Status: inventory.EntityStatusBorrowed, StatusContext: "Alice"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t, &fakeApp{entities: []app.EntityResult{tt.entity}})
			defer ts.Close()

			req, _ := http.NewRequest(http.MethodGet, ts.URL+"/entities/abc", nil)
			req.Header.Set("Hx-Request", "true")
			resp, err := ts.Client().Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)

			assert.Equal(t, tt.wantButton, strings.Contains(string(body), `data-action="loan"`))
			assert.Equal(t, tt.wantEnabled, strings.Contains(string(body), `hx-get="/entities/abc/actions/loan"`))
			assert.Equal(t, tt.wantButton && !tt.wantEnabled,
				strings.Contains(string(body), `title="Locked entities cannot be loaned"`))
		})
	}
}

func TestHandleLoanForm(t *testing.T) {
	tests := []struct {
		name     string
		entities []app.EntityResult
		wantCode int
	}{
		{
			name: "ok renders form",
			entities: []app.EntityResult{{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Drill",
				Status: inventory.EntityStatusOk}},
			wantCode: http.StatusOK,
		},
		{
			name: "locked forbidden",
			entities: []app.EntityResult{{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Drill",
				Status: inventory.EntityStatusOk, Locked: true}},
			wantCode: http.StatusForbidden,
		},
		{
			name: "loaned forbidden",
			entities: []app.EntityResult{{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Drill",
				Status: inventory.EntityStatusLoaned, StatusContext: "Bob"}},
			wantCode: http.StatusForbidden,
		},
		{name: "not found", wantCode: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t, &fakeApp{entities: tt.entities})
			defer ts.Close()

			req, _ := http.NewRequest(http.MethodGet, ts.URL+"/entities/abc/actions/loan", nil)
			req.Header.Set("Hx-Request", "true")
			resp, err := ts.Client().Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)

			assert.Equal(t, tt.wantCode, resp.StatusCode)
			if tt.wantCode == http.StatusOK {
				assert.Contains(t, string(body), `hx-post="/entities/abc/actions/loan"`)
				assert.Contains(t, string(body), `name="recipient"`)
				assert.Contains(t, string(body), "required")
				assert.Contains(t, string(body), `name="note"`)
				assert.Contains(t, string(body), "Loaned to")
			}
		})
	}
}

func postLoan(t *testing.T, ts *httptest.Server, form url.Values) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/entities/abc/actions/loan", strings.NewReader(form.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, string(body)
}

func loanableHammer() []app.EntityResult {
	return []app.EntityResult{
		{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Hammer", Status: inventory.EntityStatusOk},
	}
}

func TestHandleLoan_Success(t *testing.T) {
	fake := &fakeApp{entities: loanableHammer()}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := postLoan(t, ts, url.Values{"recipient": {"Alice"}, "note": {"for the weekend"}})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "abc", fake.lastLoanReq.EntityID)
	assert.Equal(t, "Alice", fake.lastLoanReq.StatusContext)
	assert.Equal(t, "for the weekend", fake.lastLoanReq.Note)
	assert.Contains(t, body, `id="entity-detail"`)
	assert.Contains(t, body, "loaned — Alice")
	assert.Contains(t, body, `id="tree-badge-abc"`)
	assert.NotContains(t, body, `data-action="loan"`)
	assert.NotContains(t, body, "loan failed")
}

func TestHandleLoan_BlankRecipientReopensForm(t *testing.T) {
	fake := &fakeApp{
		entities: loanableHammer(),
		loanErr:  fmt.Errorf("mark loaned: %w", app.ErrRecipientRequired),
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := postLoan(t, ts, url.Values{"recipient": {"   "}, "note": {"keep me"}})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, body, `id="entity-detail"`)
	assert.Contains(t, body, "Loaned to is required.")
	assert.Contains(t, body, `hx-post="/entities/abc/actions/loan"`)
	assert.Contains(t, body, `value="keep me"`)
	assert.NotContains(t, body, `id="tree-badge-abc"`)
}

func TestHandleLoan_ConcurrentLoanShowsFreshState(t *testing.T) {
	fake := &fakeApp{
		entities: loanableHammer(),
		loanErr:  errTest,
		onLoan: func(f *fakeApp) {
			f.entities[0].Status = inventory.EntityStatusLoaned
			f.entities[0].StatusContext = "Bob"
		},
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := postLoan(t, ts, url.Values{"recipient": {"Alice"}})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, body, "loan failed")
	assert.Contains(t, body, "loaned — Bob")
	assert.Contains(t, body, `id="tree-badge-abc"`)
	assert.NotContains(t, body, `data-action="loan"`)
}

func TestHandleLoan_GenericFailureShowsError(t *testing.T) {
	fake := &fakeApp{entities: loanableHammer(), loanErr: errTest}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := postLoan(t, ts, url.Values{"recipient": {"Alice"}})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, body, "loan failed")
	assert.Contains(t, body, `hx-get="/entities/abc/actions/loan"`)
}

func TestHandleLoan_RemovedConcurrentlyNotFound(t *testing.T) {
	fake := &fakeApp{
		entities: loanableHammer(),
		loanErr:  errTest,
		onLoan:   func(f *fakeApp) { f.entities = nil },
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, _ := postLoan(t, ts, url.Values{"recipient": {"Alice"}})

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHandleLoan_Rejected(t *testing.T) {
	tests := []struct {
		name     string
		entities []app.EntityResult
		wantCode int
	}{
		{
			name: "locked forbidden",
			entities: []app.EntityResult{{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Hammer",
				Status: inventory.EntityStatusOk, Locked: true}},
			wantCode: http.StatusForbidden,
		},
		{
			name: "loaned forbidden",
			entities: []app.EntityResult{{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Hammer",
				Status: inventory.EntityStatusLoaned, StatusContext: "Bob"}},
			wantCode: http.StatusForbidden,
		},
		{name: "not found", wantCode: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeApp{entities: tt.entities}
			ts := newTestServer(t, fake)
			defer ts.Close()

			resp, _ := postLoan(t, ts, url.Values{"recipient": {"Alice"}})

			assert.Equal(t, tt.wantCode, resp.StatusCode)
			assert.Zero(t, fake.loanCalls, "no write may be attempted")
		})
	}
}
