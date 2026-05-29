package web_test

import (
	"io"
	"net/http"
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
			EntityType: inventory.EntityTypeLeaf, Status: inventory.EntityStatusOk},
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
	assert.Contains(t, bs, `title="Leaf items cannot have children"`)
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
			EntityType: inventory.EntityTypePlace, Status: inventory.EntityStatusOk},
		{EntityID: "t1", DisplayName: "Toolbox", FullPathDisplay: "Garage:Toolbox",
			EntityType: inventory.EntityTypeContainer, Status: inventory.EntityStatusOk},
		{EntityID: "h1", DisplayName: "Hammer", FullPathDisplay: "Garage:Toolbox:Hammer",
			EntityType: inventory.EntityTypeLeaf, Status: inventory.EntityStatusOk},
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
			EntityType: inventory.EntityTypeLeaf, Status: inventory.EntityStatusOk},
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
			EntityType: inventory.EntityTypeLeaf, Status: inventory.EntityStatusMissing},
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

func TestHandleToggleMissing_PlaceForbidden(t *testing.T) {
	entities := []app.EntityResult{
		{EntityID: "abc", DisplayName: "Garage", FullPathDisplay: "Garage",
			EntityType: inventory.EntityTypePlace, Status: inventory.EntityStatusOk},
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
			EntityType: inventory.EntityTypeLeaf, Status: inventory.EntityStatusBorrowed},
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
			EntityType: inventory.EntityTypeLeaf, Status: inventory.EntityStatusOk},
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
