package web_test

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func TestHandleIndex(t *testing.T) {
	fake := &fakeApp{entities: []app.EntityResult{
		{EntityID: "abc", DisplayName: "Garage", CanonicalName: "garage",
			EntityType: inventory.EntityTypePlace, Status: inventory.EntityStatusOk},
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
			EntityType:      inventory.EntityTypeLeaf,
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
			EntityType:      inventory.EntityTypeLeaf,
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
