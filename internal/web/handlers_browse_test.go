package web_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func TestHandleIndex(t *testing.T) {
	fake := &fakeApp{entities: []app.EntityResult{
		{EntityID: "abc", DisplayName: "Garage", CanonicalName: "garage",
			FullPathDisplay: "Garage", Status: inventory.EntityStatusOk},
	}}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "Garage")
}

func TestIndex_TreeMarksAndAnchor(t *testing.T) {
	entities := []app.EntityResult{
		{EntityID: "g1", DisplayName: "Garage", CanonicalName: "garage", FullPathDisplay: "Garage",
			Status: inventory.EntityStatusOk, Locked: true},
		{EntityID: "w1", DisplayName: "Wrench", CanonicalName: "wrench", FullPathDisplay: "Wrench",
			Status: inventory.EntityStatusLoaned},
	}
	ts := newTestServer(t, &fakeApp{entities: entities})
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()
	body := readBody(t, resp)

	// ok → empty mark, loaned → hidden text
	assert.Contains(t, body, `<span id="tree-badge-g1" class="badge "></span>`)
	assert.Contains(t, body,
		`<span id="tree-badge-w1" class="badge loaned"><span class="visually-hidden">loaned</span></span>`)
	// anchor is an svg, not an emoji, and is announced as part of the row name
	assert.Contains(t, body, `class="anchor"`)
	assert.NotContains(t, body, "⚓")
	assert.Contains(t, body, `<span class="visually-hidden">locked</span>`)
	assert.Contains(t, body, `aria-labelledby="tree-label-g1 tree-lock-g1"`)
}

func TestIndex_HomeListsRoots(t *testing.T) {
	fake := &fakeApp{entities: []app.EntityResult{
		{EntityID: "g", DisplayName: "Garage", FullPathDisplay: "Garage", Status: inventory.EntityStatusOk,
			HasChildren: true},
		{EntityID: "s", DisplayName: "Shed", FullPathDisplay: "Shed", Status: inventory.EntityStatusOk},
		{EntityID: "t", DisplayName: "Toolbox", FullPathDisplay: "Garage:Toolbox", ParentID: "g", HomeID: "g",
			Status: inventory.EntityStatusOk},
	}}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()
	body := readBody(t, resp)

	assert.Contains(t, body, `<section id="home-detail"`)
	assert.Contains(t, body, `<h2>Inventory</h2>`)
	assert.Contains(t, body, `id="children-list" class="children-list" data-parent-id=""`)
	assert.Contains(t, body, `<a class="child-row" href="/entities/g"`)
	assert.Contains(t, body, `<a class="child-row" href="/entities/s"`)
	assert.NotContains(t, body, `<a class="child-row" href="/entities/t"`, "non-roots stay off the home list")
	assert.Contains(t, body, `data-action="add-child" data-parent-id="">Add root</button>`)
	assert.NotContains(t, body, "Pick something from the tree")
}

func TestIndex_HomeEmptyInventory(t *testing.T) {
	ts := newTestServer(t, &fakeApp{entities: []app.EntityResult{}})
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Contains(t, readBody(t, resp), `data-parent-id="">Nothing in the inventory yet.</p>`)
}

func TestIndex_FragmentIsHomeOnly(t *testing.T) {
	ts := newTestServer(t, &fakeApp{entities: []app.EntityResult{
		{EntityID: "g", DisplayName: "Garage", FullPathDisplay: "Garage", Status: inventory.EntityStatusOk},
	}})
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/", nil)
	require.NoError(t, err)
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body := readBody(t, resp)

	assert.Contains(t, body, `<section id="home-detail"`)
	assert.NotContains(t, body, "<!doctype")
	assert.NotContains(t, body, `id="tree-root-list"`)
}

func TestIndex_HistoryRestoreGetsFullPage(t *testing.T) {
	ts := newTestServer(t, &fakeApp{entities: []app.EntityResult{}})
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/", nil)
	require.NoError(t, err)
	req.Header.Set("Hx-Request", "true")
	req.Header.Set("Hx-History-Restore-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Contains(t, readBody(t, resp), "<!doctype")
}

func TestSearch_EmptyStatesHaveNoEmoji(t *testing.T) {
	ts := newTestServer(t, &fakeApp{entities: []app.EntityResult{}})
	defer ts.Close()

	for _, q := range []string{"", "zzz"} {
		resp, err := ts.Client().Get(ts.URL + "/search?q=" + url.QueryEscape(q))
		require.NoError(t, err)
		body := readBody(t, resp)
		resp.Body.Close()

		assert.Contains(t, body, `class="empty-state"`)
		assert.Contains(t, body, `class="empty-tape"`)
		assert.NotContains(t, body, "🔍")
	}
}

func TestHandleTreeChildren_UnknownParentReturns404(t *testing.T) {
	ts := newTestServer(t, &fakeApp{})
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/tree/ghost/children")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHandleSearch_EmptyQuery(t *testing.T) {
	ts := newTestServer(t, &fakeApp{})
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/search?q=")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "empty-state")
}

func TestHandleSearch_NoResults(t *testing.T) {
	ts := newTestServer(t, &fakeApp{findResults: []app.FindResult{}})
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/search?q=notfound")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "empty-state")
}

func TestHandleSearch_WithResults(t *testing.T) {
	results := []app.FindResult{
		{Entity: app.EntityResult{
			EntityID:        "id1",
			DisplayName:     "Hammer",
			FullPathDisplay: "Garage:Toolbox:Hammer",
			Status:          inventory.EntityStatusOk,
		}, Distance: 0},
	}
	ts := newTestServer(t, &fakeApp{findResults: results})
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/search?q=hammer")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "Hammer")
	assert.Contains(t, string(body), "Garage:Toolbox:Hammer")
}

func TestHandleSearch_HTMXFragment(t *testing.T) {
	results := []app.FindResult{
		{Entity: app.EntityResult{
			EntityID:        "id1",
			DisplayName:     "Hammer",
			FullPathDisplay: "Garage:Toolbox:Hammer",
			Status:          inventory.EntityStatusOk,
		}},
	}
	ts := newTestServer(t, &fakeApp{findResults: results})
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/search?q=hammer", nil)
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	// HTMX fragment should NOT include <html> wrapper
	assert.NotContains(t, string(body), "<html")
	assert.Contains(t, string(body), "Hammer")
}

func TestHandleSearch_AppError(t *testing.T) {
	ts := newTestServer(t, &fakeApp{findErr: errTest})
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/search?q=something")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func getParentOptions(t *testing.T, ts *httptest.Server, q string) (*http.Response, string) {
	t.Helper()
	resp, err := ts.Client().Get(ts.URL + "/entities/parent-options?q=" + url.QueryEscape(q))
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, string(body)
}

func TestHandleParentOptions_RendersCandidates(t *testing.T) {
	fake := &fakeApp{parentResults: []app.FindResult{
		{Entity: app.EntityResult{EntityID: "t1", DisplayName: "Toolbox", FullPathDisplay: "Garage:Toolbox"}},
	}}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := getParentOptions(t, ts, "  tool  ")

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "tool", fake.lastParentQuery)
	assert.Contains(t, body, `data-action="pick-parent"`)
	assert.Contains(t, body, `data-parent-id="t1"`)
	assert.Contains(t, body, `data-parent-path="Garage:Toolbox"`)
}

func TestHandleParentOptions_NoMatches(t *testing.T) {
	ts := newTestServer(t, &fakeApp{})
	defer ts.Close()

	_, body := getParentOptions(t, ts, "zzz")
	assert.Contains(t, body, "No matching locations")
}

func TestHandleParentOptions_EmptyQuerySkipsSearch(t *testing.T) {
	fake := &fakeApp{}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := getParentOptions(t, ts, "   ")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Zero(t, fake.parentCalls)
	assert.NotContains(t, body, "No matching locations")
}

func TestHandleParentOptions_QueryTooLong(t *testing.T) {
	ts := newTestServer(t, &fakeApp{})
	defer ts.Close()

	resp, _ := getParentOptions(t, ts, strings.Repeat("a", 101))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestHandleTreeNode(t *testing.T) {
	ts := newTestServer(t, &fakeApp{entities: []app.EntityResult{
		{EntityID: "abc", DisplayName: "Drill", FullPathDisplay: "Drill", Status: inventory.EntityStatusBorrowed},
	}})
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/entities/abc/tree-node")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), `id="tree-node-abc"`)
	// Named by its label and lock spans, so out-of-band swaps of either rename it.
	assert.Contains(t, string(body), `aria-labelledby="tree-label-abc tree-lock-abc"`)
	assert.NotContains(t, string(body), `aria-label="Drill"`)

	resp2, err := ts.Client().Get(ts.URL + "/entities/nope/tree-node")
	require.NoError(t, err)
	defer resp2.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp2.StatusCode)
}

func TestHandleParentOptions_MovingUsesMoveDestinations(t *testing.T) {
	fake := &fakeApp{parentResults: []app.FindResult{
		{Entity: app.EntityResult{EntityID: "w", DisplayName: "Workshop", FullPathDisplay: "Workshop"}},
	}}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/entities/parent-options?q=work&moving=abc")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "abc", fake.lastMovingID)
	assert.Equal(t, "work", fake.lastParentQuery)
	assert.Contains(t, string(body), `data-parent-id="w"`)
}

// A picker whose entity was removed while the form was open must replace its
// stale options: htmx would not swap a 404, leaving them clickable (#315).
func TestHandleParentOptions_MovingGoneReplacesOptions(t *testing.T) {
	fake := &fakeApp{findErr: app.ErrNotFound}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/entities/parent-options?q=work&moving=abc")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "This item no longer exists")
	assert.NotContains(t, string(body), `data-action="pick-parent"`)
}

func TestHandleParentOptions_WithoutMovingUsesParentCandidates(t *testing.T) {
	fake := &fakeApp{}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/entities/parent-options?q=work")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Empty(t, fake.lastMovingID)
	assert.Equal(t, 1, fake.parentCalls)
}

// A moved entity may have children; its re-inserted tree node needs a chevron.
func TestHandleTreeNode_ShowsChevronWhenEntityHasChildren(t *testing.T) {
	fake := &fakeApp{
		entities: []app.EntityResult{{EntityID: "abc", DisplayName: "Toolbox", FullPathDisplay: "Toolbox",
			Status: inventory.EntityStatusOk}},
		hasChildren: map[string]bool{"abc": true},
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/entities/abc/tree-node")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), `aria-expanded="false"`)
	assert.NotContains(t, string(body), "chevron-hidden")
}

func TestHandleSearch_HistoryRestore(t *testing.T) {
	results := []app.FindResult{
		{Entity: app.EntityResult{
			EntityID:        "id1",
			DisplayName:     "Hammer",
			FullPathDisplay: "Garage:Toolbox:Hammer",
			Status:          inventory.EntityStatusOk,
		}},
	}

	tests := []struct {
		name         string
		query        string
		restoreValue string
		wantFullPage bool
	}{
		{name: "restore with results", query: "hammer", restoreValue: "true", wantFullPage: true},
		{name: "restore with empty query", query: "", restoreValue: "true", wantFullPage: true},
		{name: "restore header not true", query: "hammer", restoreValue: "false", wantFullPage: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t, &fakeApp{findResults: results})
			defer ts.Close()

			req, err := http.NewRequest(http.MethodGet, ts.URL+"/search?q="+tt.query, nil)
			require.NoError(t, err)
			req.Header.Set("Hx-Request", "true")
			req.Header.Set("Hx-History-Restore-Request", tt.restoreValue)

			resp, err := ts.Client().Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			require.Equal(t, http.StatusOK, resp.StatusCode)
			raw, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			body := string(raw)

			if tt.wantFullPage {
				assert.Contains(t, body, "<html")
				assert.Contains(t, body, `id="tree-root-list"`)
				assert.Contains(t, body, `class="app-search"`)
			} else {
				assert.NotContains(t, body, "<html")
			}
		})
	}
}

func TestFullPages_ShareShell(t *testing.T) {
	fake := &fakeApp{entities: []app.EntityResult{
		{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Hammer", Status: inventory.EntityStatusOk},
	}}
	for _, page := range []string{"/", "/search?q=ham", "/entities/abc"} {
		t.Run(page, func(t *testing.T) {
			ts := newTestServer(t, fake)
			defer ts.Close()
			resp, err := ts.Client().Get(ts.URL + page)
			require.NoError(t, err)
			defer resp.Body.Close()
			body := readBody(t, resp)

			assert.Equal(t, 1, strings.Count(body, `id="tree-root-list"`))
			assert.Equal(t, 1, strings.Count(body, `<dialog id="add-modal"`))
			assert.Equal(t, 1, strings.Count(body, `class="app-footer"`))
			assert.Contains(t, body, `id="tree-node-abc"`)
			assert.Contains(t, body, `<button id="tree-toggle" class="btn-ghost tree-toggle" type="button" `+
				`aria-controls="tree-panel" aria-expanded="false">Inventory</button>`)
			assert.Contains(t, body, `<div id="tree-backdrop" class="tree-backdrop"></div>`)
			assert.Contains(t, body, `<span class="app-brand-name">Wherehouse</span>`)
		})
	}
}

func TestHandleParentOptions_PassesRelocation(t *testing.T) {
	for _, tt := range []struct {
		query string
		want  app.Relocation
	}{
		{"moving=abc", app.RelocationMove},
		{"moving=abc&verb=move", app.RelocationMove},
		{"moving=abc&verb=use", app.RelocationUse},
		{"moving=abc&verb=bogus", app.RelocationMove},
	} {
		t.Run(tt.query, func(t *testing.T) {
			fake := &fakeApp{}
			ts := newTestServer(t, fake)
			defer ts.Close()

			resp, err := ts.Client().Get(ts.URL + "/entities/parent-options?q=x&" + tt.query)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "abc", fake.lastMovingID)
			assert.Equal(t, tt.want, fake.lastRelocation)
		})
	}
}

// The picker served over a real app must omit a destination the write would
// reject for a home cycle (#386).
func TestHandleParentOptions_RealAppOmitsHomeCycleDestination(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	mk := func(name, parent string) string {
		e, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: name, ParentID: parent, ActorID: "t"})
		require.NoError(t, err)
		return e.EntityID
	}
	garage := mk("Garage", "")
	shelf := mk("Shelf", garage)
	drill := mk("Drill", shelf)
	deck := mk("Deck", "")
	_, err := a.UseEntity(ctx, app.UseEntityRequest{EntityID: drill, NewParentID: deck, ActorID: "t"})
	require.NoError(t, err)

	ts := newTestServer(t, a)
	defer ts.Close()
	resp, err := ts.Client().Get(ts.URL + "/entities/parent-options?q=d&moving=" + garage)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), `data-parent-id="`+deck+`"`)
	assert.NotContains(t, string(body), `data-parent-id="`+drill+`"`,
		"moving Garage into Drill puts Drill's home inside Drill")
}
