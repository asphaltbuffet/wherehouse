package web_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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
	ts := newTestServer(t, &fakeApp{entities: entities})
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/entities/abc/actions/toggle-missing", nil)
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), `id="entity-detail"`)
}

func TestHandleToggleMissing_MissingToOk(t *testing.T) {
	entities := []app.EntityResult{
		{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Garage:Hammer",
			Status: inventory.EntityStatusMissing},
	}
	ts := newTestServer(t, &fakeApp{entities: entities})
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/entities/abc/actions/toggle-missing", nil)
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
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
