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

func TestIndex_TreeMarksAndPadlock(t *testing.T) {
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
	// padlock is an svg, not an emoji
	assert.Contains(t, body, `class="padlock"`)
	assert.NotContains(t, body, "🔒")
}

func TestIndex_EmptyStateCopy(t *testing.T) {
	ts := newTestServer(t, &fakeApp{entities: []app.EntityResult{}})
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()
	body := readBody(t, resp)

	assert.Contains(t, body, `class="empty-state"`)
	assert.Contains(t, body, `<span class="empty-tape" aria-hidden="true"></span>`)
	assert.Contains(t, body, "Pick something from the tree, or search for it.")
	assert.NotContains(t, body, "📦")
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
