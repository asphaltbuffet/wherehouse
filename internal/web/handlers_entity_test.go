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

func TestHandleMissingActions(t *testing.T) {
	const (
		lost  = "/entities/abc/actions/lost"
		found = "/entities/abc/actions/found"
	)
	ent := func(status inventory.EntityStatus, locked bool) []app.EntityResult {
		return []app.EntityResult{{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Hammer",
			Status: status, Locked: locked}}
	}
	tests := []struct {
		name      string
		path      string
		entities  []app.EntityResult
		statusErr error
		wantCode  int
		wantLost  int
		wantFound int
		want      []string
		notWant   []string
	}{
		{
			name: "lost on ok", path: lost, entities: ent(inventory.EntityStatusOk, false),
			wantCode: http.StatusOK, wantLost: 1,
			want:    []string{`id="entity-detail"`, `id="tree-badge-abc"`},
			notWant: []string{staleText},
		},
		{
			name: "found on missing", path: found, entities: ent(inventory.EntityStatusMissing, false),
			wantCode: http.StatusOK, wantFound: 1,
			want:    []string{`id="entity-detail"`},
			notWant: []string{staleText},
		},
		{
			// #306 regression: a stale "Mark missing" on an entity already marked
			// missing elsewhere must not mark it found.
			name: "stale lost on missing", path: lost, entities: ent(inventory.EntityStatusMissing, false),
			statusErr: errTest, wantCode: http.StatusOK, wantLost: 1,
			want:    []string{staleText, `hx-post="/entities/abc/actions/found"`},
			notWant: []string{"status change failed"},
		},
		{
			name: "stale found on ok", path: found, entities: ent(inventory.EntityStatusOk, false),
			statusErr: errTest, wantCode: http.StatusOK, wantFound: 1,
			want: []string{staleText, `hx-post="/entities/abc/actions/lost"`},
		},
		{
			// Review Focus #2: locked after the panel rendered.
			name: "stale lost on locked", path: lost, entities: ent(inventory.EntityStatusOk, true),
			statusErr: errTest, wantCode: http.StatusOK, wantLost: 1,
			want:    []string{staleText},
			notWant: []string{`hx-post="/entities/abc/actions/lost"`, "status change failed"},
		},
		{
			name: "genuine failure", path: lost, entities: ent(inventory.EntityStatusOk, false),
			statusErr: errTest, wantCode: http.StatusOK, wantLost: 1,
			want:    []string{"status change failed", `hx-post="/entities/abc/actions/lost"`},
			notWant: []string{staleText},
		},
		{
			name: "removed elsewhere", path: lost, entities: ent(inventory.EntityStatusRemoved, false),
			statusErr: errTest, wantCode: http.StatusOK, wantLost: 1,
			want: []string{staleText, "Last location", `id="tree-node-abc" hx-swap-oob="delete"`},
		},
		{name: "unknown id", path: lost, wantCode: http.StatusNotFound, wantLost: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fa := &fakeApp{entities: tt.entities, statusErr: tt.statusErr}
			ts := newTestServer(t, fa)
			defer ts.Close()

			resp := postHTMX(t, ts, tt.path)
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)

			assert.Equal(t, tt.wantCode, resp.StatusCode)
			assert.Equal(t, tt.wantLost, fa.lostCalls, "MarkLost calls")
			assert.Equal(t, tt.wantFound, fa.foundCalls, "MarkFound calls")
			for _, s := range tt.want {
				assert.Contains(t, string(body), s)
			}
			for _, s := range tt.notWant {
				assert.NotContains(t, string(body), s)
			}
		})
	}
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
	assert.Contains(t, string(body), "Pick something from the tree, or search for it.")
	assert.Contains(t, string(body), `id="tree-node-abc" hx-swap-oob="delete"`)
	assert.Equal(t, detachedEvent{EntityID: "abc", ParentID: ""}, detachedTrigger(t, resp))
	assert.Equal(t, 1, fake.hasChildrenCalls,
		"only the detail panel's discrete check queries; a root removal must not query a parent")
}

// A stale Return on an entity already returned elsewhere reaches the app
// guard (ADR 0035); the intent holds, so the fresh state renders without an
// error (#292, #306).
func TestHandleReturn_StaleOnOkShowsCurrentState(t *testing.T) {
	fake := &fakeApp{
		entities: []app.EntityResult{
			{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Hammer", Status: inventory.EntityStatusOk},
		},
		returnErr: errTest,
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp := postHTMX(t, ts, "/entities/abc/actions/return")
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "abc", fake.lastReturnReq.EntityID, "the app layer decides (ADR 0035)")
	assert.Contains(t, string(body), `id="tree-badge-abc"`)
	assert.NotContains(t, string(body), "return failed")
	assert.NotContains(t, string(body), `hx-post="/entities/abc/actions/return"`)
}

// Review Focus #3: the entity left the inventory elsewhere (returned or
// removed); show the parent as a completed removal, without writing.
func TestHandleReturn_AlreadyRemovedShowsParent(t *testing.T) {
	fake := &fakeApp{entities: []app.EntityResult{
		{EntityID: "g", DisplayName: "Garage", FullPathDisplay: "Garage", Status: inventory.EntityStatusOk},
		{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Garage:Drill", ParentID: "g",
			Status: inventory.EntityStatusRemoved},
	}}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp := postHTMX(t, ts, "/entities/abc/actions/return")
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Empty(t, fake.lastReturnReq.EntityID, "nothing to write: the entity already left the inventory")
	assert.Equal(t, "/entities/g", resp.Header.Get("Hx-Push-Url"))
	assert.Contains(t, string(body), `id="tree-node-abc" hx-swap-oob="delete"`)
}

func TestHandleReturn_UnknownEntityNotFound(t *testing.T) {
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
	drill := func(status inventory.EntityStatus, locked bool) []app.EntityResult {
		return []app.EntityResult{{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Drill",
			Status: status, Locked: locked}}
	}
	formPost := `hx-post="/entities/abc/actions/loan"`
	tests := []struct {
		name     string
		entities []app.EntityResult
		wantCode int
		want     []string
		notWant  []string
	}{
		{
			name: "ok opens form", entities: drill(inventory.EntityStatusOk, false), wantCode: http.StatusOK,
			want: []string{`id="entity-detail"`, formPost, `name="recipient"`, "required", `name="note"`,
				"Loaned to"},
			notWant: []string{staleText},
		},
		{
			name: "locked is stale", entities: drill(inventory.EntityStatusOk, true), wantCode: http.StatusOK,
			want: []string{`id="entity-detail"`, staleText}, notWant: []string{formPost},
		},
		{
			name: "loaned is stale", entities: drill(inventory.EntityStatusLoaned, false), wantCode: http.StatusOK,
			want: []string{`id="entity-detail"`, staleText}, notWant: []string{formPost},
		},
		{
			name: "removed shows removed panel", entities: drill(inventory.EntityStatusRemoved, false),
			wantCode: http.StatusOK,
			want:     []string{staleText, "Last location", `id="tree-node-abc" hx-swap-oob="delete"`},
			notWant:  []string{formPost},
		},
		{name: "unknown id", wantCode: http.StatusNotFound},
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
			for _, s := range tt.want {
				assert.Contains(t, string(body), s)
			}
			for _, s := range tt.notWant {
				assert.NotContains(t, string(body), s)
			}
		})
	}
}

// Review Focus #1: a form GET reached as a full page (htmx history restore)
// renders the whole page with the form open, not a bare fragment.
func TestHandleLoanForm_HistoryRestoreRendersFullPage(t *testing.T) {
	ts := newTestServer(t, &fakeApp{entities: loanableHammer()})
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/entities/abc/actions/loan", nil)
	req.Header.Set("Hx-Request", "true")
	req.Header.Set("Hx-History-Restore-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "<html")
	assert.Contains(t, string(body), `hx-post="/entities/abc/actions/loan"`)
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

func renderDetail(t *testing.T, e app.EntityResult) string {
	t.Helper()
	history := []app.HistoryResult{
		{
			EventID:      412,
			EventType:    inventory.EntityCreatedEvent,
			TimestampUTC: "2026-03-03T10:00:00Z",
			ActorUserID:  "grue",
		},
	}
	ts := newTestServer(t, &fakeApp{entities: []app.EntityResult{e}, history: history})
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/entities/" + e.EntityID)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	return readBody(t, resp)
}

func TestDetail_TapeRowAndNoPathRow(t *testing.T) {
	body := renderDetail(t, app.EntityResult{EntityID: "k3x9q2m7wa", DisplayName: "Torque wrench",
		CanonicalName: "torque wrench", FullPathDisplay: "Garage:Toolbox:Torque wrench",
		Status: inventory.EntityStatusOk})

	assert.Contains(t, body, `<div class="tape-row">`)
	assert.Contains(t, body, `<h2>Torque wrench</h2>`)
	assert.Contains(t, body, `<span class="tape-id mono">k3x9q2m7wa</span>`)
	assert.Contains(t, body, `hx-target="closest .tape-row"`)
	assert.Contains(t, body, `>Edit name</button>`)
	assert.NotContains(t, body, `<dt>Path</dt>`)
	assert.NotContains(t, body, `<dt>Name</dt>`)
}

func TestDetail_Tags(t *testing.T) {
	tests := []struct {
		name    string
		tags    []string
		want    string
		wantNot string
	}{
		{"sorted chips", []string{"auto", "calibrated"},
			`<ul class="tag-list" aria-label="Tags"><li class="tag">auto</li><li class="tag">calibrated</li></ul>`, ""},
		{"nil tags omit row", nil, "", `class="tag-list"`},
		{"empty tags omit row", []string{}, "", `class="tag-list"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := renderDetail(t, app.EntityResult{EntityID: "t1", DisplayName: "Wrench",
				CanonicalName: "wrench", FullPathDisplay: "Wrench",
				Status: inventory.EntityStatusOk, Tags: tc.tags})
			if tc.want != "" {
				assert.Contains(t, body, tc.want)
			}
			if tc.wantNot != "" {
				assert.NotContains(t, body, tc.wantNot)
			}
		})
	}
}

func TestDetail_StatusLine(t *testing.T) {
	tests := []struct {
		name   string
		status inventory.EntityStatus
		ctx    string
		want   string // "" means no status line at all
	}{
		{"ok has no line", inventory.EntityStatusOk, "", ""},
		{"loaned with recipient", inventory.EntityStatusLoaned, "Dana", "Loaned to Dana"},
		{"loaned without recipient", inventory.EntityStatusLoaned, "", "Loaned</p>"},
		{"borrowed with lender", inventory.EntityStatusBorrowed, "Sam", "Borrowed from Sam"},
		{"borrowed without lender", inventory.EntityStatusBorrowed, "", "Borrowed</p>"},
		{"missing", inventory.EntityStatusMissing, "", "Missing</p>"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := renderDetail(t, app.EntityResult{EntityID: "s1", DisplayName: "Wrench",
				CanonicalName: "wrench", FullPathDisplay: "Wrench",
				Status: tc.status, StatusContext: tc.ctx})
			if tc.want == "" {
				assert.NotContains(t, body, `class="status-line"`)
				return
			}
			assert.Contains(t, body, `class="status-line"`)
			assert.Contains(t, body, tc.want)
			assert.NotContains(t, body, " to </p>")
			assert.NotContains(t, body, " from </p>")
		})
	}
}

func TestDetail_PrimaryActionAndLedger(t *testing.T) {
	body := renderDetail(t, app.EntityResult{EntityID: "p1", DisplayName: "Wrench",
		CanonicalName: "wrench", FullPathDisplay: "Wrench",
		Status: inventory.EntityStatusLoaned, StatusContext: "Dana"})

	assert.Contains(t, body, `class="btn-primary detail-action-btn"`) // Return is primary
	assert.Contains(t, body, `<td class="num mono">412</td>`)
	assert.Contains(t, body, `class="detail-actions"`)
}

func TestDetail_ActionsHeadingForScreenReaders(t *testing.T) {
	body := renderDetail(t, app.EntityResult{EntityID: "h1", DisplayName: "Wrench",
		CanonicalName: "wrench", FullPathDisplay: "Wrench", Status: inventory.EntityStatusOk})

	assert.Contains(t, body, `<h3 class="visually-hidden">Actions</h3>`)
}

func TestEditNameForm_RootIsTapeRow(t *testing.T) {
	ts := newTestServer(t, &fakeApp{entities: []app.EntityResult{
		{EntityID: "e1", DisplayName: "Wrench", CanonicalName: "wrench", FullPathDisplay: "Wrench"},
	}})
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/entities/e1/edit/name")
	require.NoError(t, err)
	defer resp.Body.Close()
	body := strings.TrimSpace(readBody(t, resp))

	assert.True(t, strings.HasPrefix(body, `<div class="tape-row">`), body)
	assert.Contains(t, body, `hx-target="#entity-detail"`)
}

func TestInlineForms_TextButtons(t *testing.T) {
	tests := []struct {
		action string
		submit string
	}{
		{"loan", "Loan"},
		{"move", "Move"},
		{"remove", "Remove"},
	}
	for _, tc := range tests {
		t.Run(tc.action, func(t *testing.T) {
			ts := newTestServer(t, &fakeApp{entities: loanableHammer()})
			defer ts.Close()

			resp, err := ts.Client().Get(ts.URL + "/entities/abc/actions/" + tc.action)
			require.NoError(t, err)
			defer resp.Body.Close()
			body := readBody(t, resp)

			assert.Contains(t, body, `form-panel" data-action="`+tc.action+`"`)
			assert.Contains(t, body, `<button type="submit" class="btn-primary">`+tc.submit+`</button>`)
			assert.Contains(t, body, `>Cancel</button>`)
			assert.NotContains(t, body, "✓")
			assert.NotContains(t, body, "✕")
		})
	}
}

func TestTreeBadgeOOB_HasHiddenStatusText(t *testing.T) {
	ts := newTestServer(t, &fakeApp{entities: loanableHammer()})
	defer ts.Close()

	_, body := postLoan(t, ts, url.Values{"recipient": {"Alice"}})

	assert.Contains(
		t,
		body,
		`<span id="tree-badge-abc" class="badge loaned" hx-swap-oob="true"><span class="visually-hidden">loaned</span></span>`,
	)
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
	assert.Contains(t, body, "Loaned to Alice")
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
	assert.Contains(t, body, staleText)
	assert.Contains(t, body, "Loaned to Bob")
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

func TestHandleLoan_RemovedConcurrentlyShowsRemovedPanel(t *testing.T) {
	fake := &fakeApp{
		entities: loanableHammer(),
		loanErr:  errTest,
		onLoan:   func(f *fakeApp) { f.entities[0].Status = inventory.EntityStatusRemoved },
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := postLoan(t, ts, url.Values{"recipient": {"Alice"}})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, body, staleText)
	assert.Contains(t, body, "Last location")
	assert.Contains(t, body, `id="tree-node-abc" hx-swap-oob="delete"`)
}

// The app guard decides (ADR 0035): a stale panel's loan reaches it and the
// refusal renders the fresh state, never a bare 403 (#306). "already loaned"
// is also a double-submitted form (Review Focus #5).
func TestHandleLoan_Rejected(t *testing.T) {
	tests := []struct {
		name     string
		entities []app.EntityResult
	}{
		{
			name: "locked",
			entities: []app.EntityResult{{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Hammer",
				Status: inventory.EntityStatusOk, Locked: true}},
		},
		{
			name: "already loaned",
			entities: []app.EntityResult{{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Hammer",
				Status: inventory.EntityStatusLoaned, StatusContext: "Bob"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeApp{entities: tt.entities, loanErr: errTest}
			ts := newTestServer(t, fake)
			defer ts.Close()

			resp, body := postLoan(t, ts, url.Values{"recipient": {"Alice"}})

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, 1, fake.loanCalls, "the app layer decides")
			assert.Contains(t, body, staleText)
			assert.NotContains(t, body, "loan failed")
			assert.NotContains(t, body, `hx-post="/entities/abc/actions/loan"`)
		})
	}
}

func TestHandleLoan_UnknownEntityNotFound(t *testing.T) {
	fake := &fakeApp{}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, _ := postLoan(t, ts, url.Values{"recipient": {"Alice"}})

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Zero(t, fake.loanCalls)
}

func TestEntityDetail_MoveButton(t *testing.T) {
	tests := []struct {
		name        string
		entity      app.EntityResult
		wantEnabled bool
		wantReason  string
	}{
		{
			name: "ok is movable",
			entity: app.EntityResult{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Drill",
				Status: inventory.EntityStatusOk},
			wantEnabled: true,
		},
		{
			name: "borrowed is movable",
			entity: app.EntityResult{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Drill",
				Status: inventory.EntityStatusBorrowed, StatusContext: "Alice"},
			wantEnabled: true,
		},
		{
			name: "locked is disabled",
			entity: app.EntityResult{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Drill",
				Status: inventory.EntityStatusOk, Locked: true},
			wantReason: "Locked entities cannot be moved",
		},
		{
			name: "missing is disabled",
			entity: app.EntityResult{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Drill",
				Status: inventory.EntityStatusMissing},
			wantReason: "Missing entities cannot be moved — mark found first",
		},
		{
			name: "loaned is disabled",
			entity: app.EntityResult{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Drill",
				Status: inventory.EntityStatusLoaned, StatusContext: "Bob"},
			wantReason: "Loaned entities cannot be moved — return it first",
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

			assert.Contains(t, string(body), `data-action="move"`)
			assert.Equal(t, tt.wantEnabled, strings.Contains(string(body), `hx-get="/entities/abc/actions/move"`))
			if tt.wantReason != "" {
				assert.Contains(t, string(body), `title="`+tt.wantReason+`"`)
			}
		})
	}
}

func TestHandleMoveForm(t *testing.T) {
	formPost := `hx-post="/entities/abc/actions/move"`
	tests := []struct {
		name       string
		entities   []app.EntityResult
		wantCode   int
		wantForm   bool
		wantToRoot bool
		want       []string
	}{
		{
			name: "nested entity offers root",
			entities: []app.EntityResult{
				{EntityID: "g", DisplayName: "Garage", FullPathDisplay: "Garage", Status: inventory.EntityStatusOk},
				{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Garage:Drill", ParentID: "g",
					Status: inventory.EntityStatusOk},
			},
			wantCode: http.StatusOK, wantForm: true, wantToRoot: true,
		},
		{
			name: "root entity hides root choice",
			entities: []app.EntityResult{{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Drill",
				Status: inventory.EntityStatusOk}},
			wantCode: http.StatusOK, wantForm: true,
		},
		{
			name: "missing is stale",
			entities: []app.EntityResult{{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Drill",
				Status: inventory.EntityStatusMissing}},
			wantCode: http.StatusOK, want: []string{`id="entity-detail"`, staleText},
		},
		{
			name: "removed shows removed panel",
			entities: []app.EntityResult{{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Drill",
				Status: inventory.EntityStatusRemoved}},
			wantCode: http.StatusOK,
			want:     []string{staleText, "Last location", `id="tree-node-abc" hx-swap-oob="delete"`},
		},
		{name: "unknown id", wantCode: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t, &fakeApp{entities: tt.entities})
			defer ts.Close()

			req, _ := http.NewRequest(http.MethodGet, ts.URL+"/entities/abc/actions/move", nil)
			req.Header.Set("Hx-Request", "true")
			resp, err := ts.Client().Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)

			assert.Equal(t, tt.wantCode, resp.StatusCode)
			for _, s := range tt.want {
				assert.Contains(t, string(body), s)
			}
			assert.Equal(t, tt.wantForm, strings.Contains(string(body), formPost))
			if !tt.wantForm {
				return
			}
			assert.Contains(t, string(body), `id="entity-detail"`)
			assert.Contains(t, string(body), `name="destination"`)
			assert.Contains(t, string(body), `name="note"`)
			assert.Contains(t, string(body), `hx-vals='{"moving": "abc"}'`)
			assert.NotContains(t, string(body), staleText)
			assert.Equal(t, tt.wantToRoot, strings.Contains(string(body), `data-parent-value="root"`))
		})
	}
}

func postMove(t *testing.T, ts *httptest.Server, form url.Values) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/entities/abc/actions/move", strings.NewReader(form.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, string(body)
}

func movableDrill() []app.EntityResult {
	return []app.EntityResult{
		{EntityID: "g", DisplayName: "Garage", FullPathDisplay: "Garage", Status: inventory.EntityStatusOk},
		{EntityID: "w", DisplayName: "Workshop", FullPathDisplay: "Workshop", Status: inventory.EntityStatusOk},
		{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Garage:Drill", ParentID: "g",
			Status: inventory.EntityStatusOk},
	}
}

func TestHandleMove_Success(t *testing.T) {
	fake := &fakeApp{entities: movableDrill(), hasChildren: map[string]bool{}}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := postMove(t, ts, url.Values{"destination": {"w"}, "note": {"new bench"}})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "abc", fake.lastMoveReq.EntityID)
	assert.Equal(t, "w", fake.lastMoveReq.NewParentID)
	assert.Equal(t, "new bench", fake.lastMoveReq.Note)
	assert.Contains(t, body, `id="entity-detail"`)
	assert.Contains(t, body, `href="/entities/w">Workshop</a>`, "new parent appears in breadcrumbs")
	assert.Contains(t, body, `id="tree-node-abc" hx-swap-oob="delete"`)
	assert.JSONEq(t,
		`{"tree-node-detached":{"entityId":"abc","parentId":"g","parentHasChildren":false},`+
			`"tree-node-attached":{"entityId":"abc","parentId":"w"}}`,
		resp.Header.Get("Hx-Trigger-After-Swap"))
}

func TestHandleMove_ToRoot(t *testing.T) {
	fake := &fakeApp{entities: movableDrill()}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := postMove(t, ts, url.Values{"destination": {"root"}})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Empty(t, fake.lastMoveReq.NewParentID)
	assert.Contains(t, resp.Header.Get("Hx-Trigger-After-Swap"),
		`"tree-node-attached":{"entityId":"abc","parentId":""}`)
	assert.NotContains(t, body, "move failed")
}

func TestHandleMove_BlankDestinationReopensForm(t *testing.T) {
	fake := &fakeApp{entities: movableDrill()}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := postMove(t, ts, url.Values{"destination": {""}, "note": {"keep me"}})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 0, fake.moveCalls)
	assert.Contains(t, body, "Choose a destination.")
	assert.Contains(t, body, `hx-post="/entities/abc/actions/move"`)
	assert.Contains(t, body, `value="keep me"`)
}

// A destination removed (or otherwise invalid) since it was picked re-opens the form.
func TestHandleMove_InvalidDestinationReopensForm(t *testing.T) {
	for _, moveErr := range []error{
		fmt.Errorf("get entity %q: %w", "w", app.ErrNotFound),
		fmt.Errorf("cannot move: %w", app.ErrInvalidDestination),
	} {
		t.Run(moveErr.Error(), func(t *testing.T) {
			fake := &fakeApp{entities: movableDrill(), moveErr: moveErr}
			ts := newTestServer(t, fake)
			defer ts.Close()

			resp, body := postMove(t, ts, url.Values{"destination": {"w"}})

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Contains(t, body, "Can&#39;t move there — choose another destination.")
			assert.Contains(t, body, `hx-post="/entities/abc/actions/move"`)
			assert.Empty(t, resp.Header.Get("Hx-Trigger-After-Swap"))
		})
	}
}

func TestHandleMove_ConcurrentLostShowsFreshState(t *testing.T) {
	fake := &fakeApp{
		entities: movableDrill(),
		moveErr:  errTest,
		onMove:   func(f *fakeApp) { f.entities[2].Status = inventory.EntityStatusMissing },
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := postMove(t, ts, url.Values{"destination": {"w"}})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, body, staleText)
	assert.Contains(t, body, `title="Missing entities cannot be moved — mark found first"`)
	assert.Contains(t, body, `id="tree-badge-abc"`)
}

func TestHandleMove_RemovedConcurrentlyShowsRemovedPanel(t *testing.T) {
	fake := &fakeApp{
		entities: movableDrill(),
		moveErr:  errTest,
		onMove:   func(f *fakeApp) { f.entities[2].Status = inventory.EntityStatusRemoved },
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := postMove(t, ts, url.Values{"destination": {"w"}})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, body, staleText)
	assert.Contains(t, body, "Last location")
	assert.Contains(t, body, `id="tree-node-abc" hx-swap-oob="delete"`)
}

// The app guard decides (ADR 0035); a stale panel's move renders the fresh
// state, never a bare 403 (#306).
func TestHandleMove_Rejected(t *testing.T) {
	fake := &fakeApp{
		entities: []app.EntityResult{
			{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Drill", Status: inventory.EntityStatusLoaned,
				StatusContext: "Bob"},
		},
		moveErr: errTest,
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := postMove(t, ts, url.Values{"destination": {"root"}})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 1, fake.moveCalls, "the app layer decides")
	assert.Contains(t, body, staleText)
	assert.NotContains(t, body, "move failed")
}

func TestEntityDetail_RemoveButton(t *testing.T) {
	drill := func(status inventory.EntityStatus) app.EntityResult {
		return app.EntityResult{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Drill", Status: status}
	}
	ok := drill(inventory.EntityStatusOk)
	borrowed := drill(inventory.EntityStatusBorrowed)
	missing := drill(inventory.EntityStatusMissing)
	loaned := drill(inventory.EntityStatusLoaned)
	locked := ok
	locked.Locked = true

	tests := []struct {
		name        string
		entity      app.EntityResult
		hasChildren bool
		wantButton  bool
		wantEnabled bool
	}{
		{name: "ok leaf is removable", entity: ok, wantButton: true, wantEnabled: true},
		{name: "missing is removable", entity: missing, wantButton: true, wantEnabled: true},
		{name: "loaned is removable", entity: loaned, wantButton: true, wantEnabled: true},
		{name: "locked is removable", entity: locked, wantButton: true, wantEnabled: true},
		{name: "live children disable it", entity: ok, hasChildren: true, wantButton: true},
		{name: "borrowed has no remove button", entity: borrowed},
		// Review Focus #1: Return is the exit; a disabled Remove would be noise.
		{name: "borrowed with children has no remove button", entity: borrowed, hasChildren: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t, &fakeApp{
				entities:    []app.EntityResult{tt.entity},
				hasChildren: map[string]bool{"abc": tt.hasChildren},
			})
			defer ts.Close()

			req, _ := http.NewRequest(http.MethodGet, ts.URL+"/entities/abc", nil)
			req.Header.Set("Hx-Request", "true")
			resp, err := ts.Client().Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)

			assert.Equal(t, tt.wantButton, strings.Contains(string(body), `data-action="remove"`))
			assert.Equal(t, tt.wantEnabled, strings.Contains(string(body), `hx-get="/entities/abc/actions/remove"`))
			assert.Equal(t, tt.wantButton && !tt.wantEnabled,
				strings.Contains(string(body), `title="Contains other entities — remove or move them first"`))
		})
	}
}

// Review Focus #2: Discrete and Remove share one child-count query.
func TestEntityDetail_SingleChildCountQuery(t *testing.T) {
	fake := &fakeApp{
		entities: []app.EntityResult{
			{EntityID: "abc", DisplayName: "Box", FullPathDisplay: "Box", Status: inventory.EntityStatusOk},
		},
		hasChildren: map[string]bool{"abc": true},
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/entities/abc", nil)
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, 1, fake.hasChildrenCalls)
	assert.Contains(t, string(body), `title="Entities with children cannot be marked discrete"`)
	assert.Contains(t, string(body), `title="Contains other entities — remove or move them first"`)
}

// Review Focus #3: a failed count must not break the page; the app guard is the backstop.
func TestEntityDetail_ChildCountErrorKeepsRemoveEnabled(t *testing.T) {
	ts := newTestServer(t, &fakeApp{
		entities: []app.EntityResult{
			{EntityID: "abc", DisplayName: "Box", FullPathDisplay: "Box", Status: inventory.EntityStatusOk},
		},
		hasChildrenErr: errTest,
	})
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/entities/abc", nil)
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), `hx-get="/entities/abc/actions/remove"`)
}

func TestHandleRemoveForm(t *testing.T) {
	hammer := func(status inventory.EntityStatus) []app.EntityResult {
		return []app.EntityResult{{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Hammer", Status: status}}
	}
	formPost := `hx-post="/entities/abc/actions/remove"`
	tests := []struct {
		name        string
		entities    []app.EntityResult
		hasChildren bool
		wantCode    int
		want        []string
		notWant     []string
	}{
		{
			name: "ok opens form", entities: hammer(inventory.EntityStatusOk), wantCode: http.StatusOK,
			want: []string{
				`id="entity-detail"`, formPost,
				// renderAfterRemoval may return detail_empty, which has no #entity-detail wrapper.
				`hx-target="#detail-panel"`, `name="note"`,
				"Remove Hammer from the inventory? This cannot be undone.",
			},
			notWant: []string{"hx-confirm", staleText},
		},
		{
			name: "borrowed is stale", entities: hammer(inventory.EntityStatusBorrowed), wantCode: http.StatusOK,
			want: []string{`id="entity-detail"`, staleText}, notWant: []string{formPost},
		},
		{
			name: "live children is stale", entities: hammer(inventory.EntityStatusOk), hasChildren: true,
			wantCode: http.StatusOK,
			want:     []string{`id="entity-detail"`, staleText}, notWant: []string{formPost},
		},
		{
			name: "removed shows removed panel", entities: hammer(inventory.EntityStatusRemoved),
			wantCode: http.StatusOK,
			want:     []string{staleText, "Last location", `id="tree-node-abc" hx-swap-oob="delete"`},
			notWant:  []string{formPost},
		},
		{name: "unknown id", wantCode: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t, &fakeApp{entities: tt.entities, hasChildren: map[string]bool{"abc": tt.hasChildren}})
			defer ts.Close()

			req, _ := http.NewRequest(http.MethodGet, ts.URL+"/entities/abc/actions/remove", nil)
			req.Header.Set("Hx-Request", "true")
			resp, err := ts.Client().Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)

			assert.Equal(t, tt.wantCode, resp.StatusCode)
			for _, s := range tt.want {
				assert.Contains(t, string(body), s)
			}
			for _, s := range tt.notWant {
				assert.NotContains(t, string(body), s)
			}
		})
	}
}

func postRemove(t *testing.T, ts *httptest.Server, form url.Values) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(
		http.MethodPost, ts.URL+"/entities/abc/actions/remove", strings.NewReader(form.Encode()),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, string(body)
}

func hammerInGarage(status inventory.EntityStatus) []app.EntityResult {
	return []app.EntityResult{
		{EntityID: "g", DisplayName: "Garage", FullPathDisplay: "Garage", Status: inventory.EntityStatusOk},
		{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Garage:Hammer", ParentID: "g", Status: status},
	}
}

func TestHandleRemove_ShowsParentAndDeletesNode(t *testing.T) {
	fake := &fakeApp{entities: hammerInGarage(inventory.EntityStatusOk)}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := postRemove(t, ts, url.Values{"note": {"  broken handle  "}})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "abc", fake.lastRemoveReq.EntityID)
	assert.Equal(t, "broken handle", fake.lastRemoveReq.Note)
	assert.Equal(t, "/entities/g", resp.Header.Get("Hx-Push-Url"))
	assert.Contains(t, body, "<h2>Garage</h2>")
	assert.Contains(t, body, `id="tree-node-abc" hx-swap-oob="delete"`)
	assert.Equal(t, detachedEvent{EntityID: "abc", ParentID: "g", ParentHasChildren: false}, detachedTrigger(t, resp))
}

func TestHandleRemove_RootShowsEmptyState(t *testing.T) {
	ts := newTestServer(t, &fakeApp{entities: []app.EntityResult{
		{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Hammer", Status: inventory.EntityStatusOk},
	}})
	defer ts.Close()

	resp, body := postRemove(t, ts, url.Values{})

	assert.Equal(t, "/", resp.Header.Get("Hx-Push-Url"))
	assert.Contains(t, body, "Pick something from the tree, or search for it.")
	assert.Contains(t, body, `id="tree-node-abc" hx-swap-oob="delete"`)
}

// Review Focus #4: a whitespace-only note is stored as no note.
func TestHandleRemove_BlankNoteStoredEmpty(t *testing.T) {
	fake := &fakeApp{entities: hammerInGarage(inventory.EntityStatusOk)}
	ts := newTestServer(t, fake)
	defer ts.Close()

	postRemove(t, ts, url.Values{"note": {"   "}})

	assert.Equal(t, 1, fake.removeCalls)
	assert.Empty(t, fake.lastRemoveReq.Note)
}

// The app guard rejects removing a borrowed entity (ADR 0022) with an untyped
// error; the handler renders the fresh state, never a bare 403 (#306).
func TestHandleRemove_BorrowedRejectedShowsCurrentState(t *testing.T) {
	fake := &fakeApp{entities: hammerInGarage(inventory.EntityStatusBorrowed), removeErr: errTest}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := postRemove(t, ts, url.Values{})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, 1, fake.removeCalls, "the app layer decides")
	assert.Contains(t, body, staleText)
	assert.NotContains(t, body, "remove failed")
}

// #287 guard tripped at submit (a child was added after the form opened).
func TestHandleRemove_HasChildrenReopensFormWithNote(t *testing.T) {
	fake := &fakeApp{
		entities:  hammerInGarage(inventory.EntityStatusOk),
		removeErr: fmt.Errorf("remove: %w", app.ErrHasChildren),
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := postRemove(t, ts, url.Values{"note": {"keep me"}})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, body, "remove failed: it still contains other entities — remove or move them first")
	assert.Contains(t, body, `hx-post="/entities/abc/actions/remove"`)
	assert.Contains(t, body, `value="keep me"`)
	assert.NotContains(t, body, `hx-swap-oob="delete"`)
	assert.Empty(t, resp.Header.Get("Hx-Push-Url"))
	assert.Empty(t, resp.Header.Get("Hx-Trigger-After-Swap"))
}

// Review Focus #5: double submit / stale tab after the entity is already removed.
func TestHandleRemove_AlreadyRemovedShowsRemoval(t *testing.T) {
	fake := &fakeApp{entities: hammerInGarage(inventory.EntityStatusRemoved)}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := postRemove(t, ts, url.Values{"note": {"late"}})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Zero(t, fake.removeCalls, "nothing to write: the intent already holds")
	assert.Equal(t, "/entities/g", resp.Header.Get("Hx-Push-Url"))
	assert.Contains(t, body, `id="tree-node-abc" hx-swap-oob="delete"`)
}

// A concurrent remove committed between our detail read and our write (#292 pattern).
func TestHandleRemove_ConcurrentRemoveShowsRemoval(t *testing.T) {
	fake := &fakeApp{
		entities:  hammerInGarage(inventory.EntityStatusOk),
		removeErr: fmt.Errorf("remove: %w", app.ErrNotFound),
		onRemove:  func(f *fakeApp) { f.entities[1].Status = inventory.EntityStatusRemoved },
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := postRemove(t, ts, url.Values{})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "/entities/g", resp.Header.Get("Hx-Push-Url"))
	assert.Contains(t, body, `id="tree-node-abc" hx-swap-oob="delete"`)
}

func TestHandleRemove_UnknownEntityNotFound(t *testing.T) {
	ts := newTestServer(t, &fakeApp{})
	defer ts.Close()

	resp, _ := postRemove(t, ts, url.Values{})
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHandleRemove_GenericFailureShowsError(t *testing.T) {
	ts := newTestServer(t, &fakeApp{
		entities:  hammerInGarage(inventory.EntityStatusOk),
		removeErr: errTest,
	})
	defer ts.Close()

	resp, body := postRemove(t, ts, url.Values{})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, body, "remove failed")
	assert.Contains(t, body, "<h2>Hammer</h2>")
	assert.NotContains(t, body, `hx-swap-oob="delete"`)
}

func TestHandleEntityDetail_HistoryRestore(t *testing.T) {
	fake := &fakeApp{
		entities: []app.EntityResult{
			{EntityID: "abc", DisplayName: "Garage", CanonicalName: "garage",
				FullPathDisplay: "Garage", Status: inventory.EntityStatusOk},
		},
		history: []app.HistoryResult{
			{EventID: 1, TimestampUTC: "2025-01-01T00:00:00Z", ActorUserID: "alice"},
		},
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/entities/abc", nil)
	require.NoError(t, err)
	req.Header.Set("Hx-Request", "true")
	req.Header.Set("Hx-History-Restore-Request", "true")

	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	body := string(raw)

	assert.Contains(t, body, "<html")
	assert.Contains(t, body, `id="tree-root-list"`)
	assert.Contains(t, body, `class="app-search"`)
	assert.Contains(t, body, `id="entity-detail"`)
}

func TestHandleEntityDetail_Removed(t *testing.T) {
	fake := &fakeApp{
		entities: []app.EntityResult{
			{EntityID: "g", DisplayName: "Garage", CanonicalName: "garage",
				FullPathDisplay: "Garage", Status: inventory.EntityStatusOk},
			{EntityID: "h", DisplayName: "Hammer", CanonicalName: "hammer", ParentID: "g",
				FullPathDisplay: "Garage:Hammer", Status: inventory.EntityStatusRemoved},
		},
		history: []app.HistoryResult{
			{EventID: 1, TimestampUTC: "2025-01-01T00:00:00Z", ActorUserID: "alice"},
			{EventID: 2, TimestampUTC: "2025-02-01T00:00:00Z", ActorUserID: "bob"},
		},
	}

	tests := []struct {
		name string
		htmx bool
	}{
		{name: "full page", htmx: false},
		{name: "fragment", htmx: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t, fake)
			defer ts.Close()

			req, err := http.NewRequest(http.MethodGet, ts.URL+"/entities/h", nil)
			require.NoError(t, err)
			if tt.htmx {
				req.Header.Set("Hx-Request", "true")
			}

			resp, err := ts.Client().Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, http.StatusOK, resp.StatusCode)
			raw, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			body := string(raw)

			assert.Contains(t, body, "Hammer")
			assert.Contains(t, body, "Removed")
			assert.Contains(t, body, "Garage:Hammer", "last location shown")
			assert.Contains(t, body, "bob", "history shown")
			assert.NotContains(t, body, "/entities/h/edit/name")
			assert.NotContains(t, body, "/entities/h/actions/")
			assert.Equal(t, tt.htmx, !strings.Contains(body, "<html"))
		})
	}
}

func TestHandleEntityDetail_UnknownIDNotFound(t *testing.T) {
	ts := newTestServer(t, &fakeApp{})
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/entities/nope")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
