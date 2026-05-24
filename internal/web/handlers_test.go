package web_test

import (
	"context"
	"errors"
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

// fakeApp satisfies web.App for tests.
type fakeApp struct {
	entities     []app.EntityResult
	history      []app.HistoryResult
	err          error
	renameResult app.EntityResult
	renameErr    error
	statusErr    error
}

func (f *fakeApp) ListEntities(_ context.Context) ([]app.EntityResult, error) {
	return f.entities, f.err
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

func (f *fakeApp) ChangeStatus(_ context.Context, _ app.ChangeStatusRequest) error {
	return f.statusErr
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

func TestHandleHealthz(t *testing.T) {
	ts := newTestServer(t, &fakeApp{})
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

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
		renameErr: errors.New("database locked"),
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

func TestHandleEditStatusForm_Editable(t *testing.T) {
	entities := []app.EntityResult{
		{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Garage:Hammer",
			Status: inventory.EntityStatusOk},
	}
	ts := newTestServer(t, &fakeApp{entities: entities})
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/entities/abc/edit/status", nil)
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	bs := string(body)
	assert.Contains(t, bs, `hx-post="/entities/abc/edit/status"`)
	assert.Contains(t, bs, `value="ok"`)
	assert.Contains(t, bs, `selected`)
}

func TestHandleEditStatus_Success(t *testing.T) {
	entities := []app.EntityResult{
		{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Garage:Hammer",
			Status: inventory.EntityStatusOk},
	}
	ts := newTestServer(t, &fakeApp{entities: entities})
	defer ts.Close()

	form := strings.NewReader("status=missing&status_context=")
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/entities/abc/edit/status", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), `id="entity-detail"`)
}

func TestHandleEditStatus_AppError(t *testing.T) {
	entities := []app.EntityResult{
		{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Garage:Hammer",
			Status: inventory.EntityStatusOk},
	}
	fa := &fakeApp{
		entities:  entities,
		statusErr: errors.New("write conflict"),
	}
	ts := newTestServer(t, fa)
	defer ts.Close()

	form := strings.NewReader("status=missing&status_context=")
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/entities/abc/edit/status", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(body), "status change failed")
}

func TestHandleEditStatusForm_NonEditable(t *testing.T) {
	entities := []app.EntityResult{
		{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Garage:Hammer",
			Status: inventory.EntityStatusBorrowed},
	}
	ts := newTestServer(t, &fakeApp{entities: entities})
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/entities/abc/edit/status", nil)
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
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

func TestHandleEditStatus_NonEditable(t *testing.T) {
	entities := []app.EntityResult{
		{EntityID: "abc", DisplayName: "Hammer", FullPathDisplay: "Garage:Hammer",
			Status: inventory.EntityStatusLoaned},
	}
	ts := newTestServer(t, &fakeApp{entities: entities})
	defer ts.Close()

	form := strings.NewReader("status=ok&status_context=")
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/entities/abc/edit/status", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}
