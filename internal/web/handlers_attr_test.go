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
)

func postAttr(t *testing.T, ts *httptest.Server, entityID, body string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/entities/"+entityID+"/actions/attr", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, string(b)
}

func getDetail(t *testing.T, ts *httptest.Server, entityID string) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/entities/"+entityID, nil)
	require.NoError(t, err)
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(b)
}

func attrEntity(status inventory.EntityStatus, locked, discrete bool) app.EntityResult {
	return app.EntityResult{EntityID: "abc", DisplayName: "Shelf", FullPathDisplay: "Shelf",
		Status: status, Locked: locked, Discrete: discrete}
}

func TestDetail_AttrButtons(t *testing.T) {
	cases := []struct {
		name        string
		entity      app.EntityResult
		hasChildren bool
		childErr    error
		want        []string // substrings that must appear
	}{
		{"unlocked ok", attrEntity(inventory.EntityStatusOk, false, false), false, nil,
			[]string{`name="locked" value="true"`, ">Lock<", `name="discrete" value="true"`, ">Mark discrete<"}},
		{"locked", attrEntity(inventory.EntityStatusOk, true, false), false, nil,
			[]string{`name="locked" value="false"`, ">Unlock<"}},
		{"discrete", attrEntity(inventory.EntityStatusOk, false, true), false, nil,
			[]string{`name="discrete" value="false"`, ">Clear discrete<"}},
		{"missing cannot lock", attrEntity(inventory.EntityStatusMissing, false, false), false, nil,
			[]string{`title="Missing or loaned entities cannot be locked"`}},
		{"loaned cannot lock", attrEntity(inventory.EntityStatusLoaned, false, false), false, nil,
			[]string{`title="Missing or loaned entities cannot be locked"`}},
		{"borrowed can lock", attrEntity(inventory.EntityStatusBorrowed, false, false), false, nil,
			[]string{`name="locked" value="true"`}},
		{"children cannot be discrete", attrEntity(inventory.EntityStatusOk, false, false), true, nil,
			[]string{`title="Entities with children cannot be marked discrete"`}},
		{
			"child count error leaves discrete enabled",
			attrEntity(inventory.EntityStatusOk, false, false), false, errTest,
			[]string{`name="discrete" value="true"`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeApp{
				entities:       []app.EntityResult{tc.entity},
				hasChildren:    map[string]bool{"abc": tc.hasChildren},
				hasChildrenErr: tc.childErr,
			}
			ts := newTestServer(t, fake)
			defer ts.Close()

			body := getDetail(t, ts, "abc")
			for _, w := range tc.want {
				assert.Contains(t, body, w)
			}
		})
	}
}

func TestHandleAttr_Lock(t *testing.T) {
	fake := &fakeApp{entities: []app.EntityResult{attrEntity(inventory.EntityStatusOk, false, false)}}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := postAttr(t, ts, "abc", "locked=true")

	require.Equal(t, http.StatusOK, resp.StatusCode, body)
	require.NotNil(t, fake.lastAttrReq.Locked)
	assert.True(t, *fake.lastAttrReq.Locked)
	assert.Nil(t, fake.lastAttrReq.Discrete)
	assert.Equal(t, "abc", fake.lastAttrReq.EntityID)
	assert.Contains(t, body, ">Unlock<", "detail re-rendered with new state")
	assert.Contains(t, body,
		`id="tree-lock-abc" class="tree-slot" hx-swap-oob="true"><svg class="padlock"`)
}

func TestHandleAttr_DiscreteRemovesTreeAddButton(t *testing.T) {
	fake := &fakeApp{entities: []app.EntityResult{attrEntity(inventory.EntityStatusOk, false, false)}}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := postAttr(t, ts, "abc", "discrete=true")

	require.Equal(t, http.StatusOK, resp.StatusCode, body)
	assert.Contains(t, body, `id="tree-add-abc" class="tree-slot" hx-swap-oob="true"></span>`)
}

func TestHandleAttr_BadRequest(t *testing.T) {
	cases := []struct{ name, body string }{
		{"no field", ""},
		{"invalid value", "locked=maybe"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeApp{entities: []app.EntityResult{attrEntity(inventory.EntityStatusOk, false, false)}}
			ts := newTestServer(t, fake)
			defer ts.Close()

			resp, _ := postAttr(t, ts, "abc", tc.body)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			assert.Zero(t, fake.attrCalls)
		})
	}
}

func TestHandleAttr_NotFound(t *testing.T) {
	ts := newTestServer(t, &fakeApp{})
	defer ts.Close()

	resp, _ := postAttr(t, ts, "nope", "locked=true")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// A stale panel posts the old target; the app reports no change and the fresh
// state is shown with an inline error — never flipped back.
func TestHandleAttr_StaleClickShowsCurrentState(t *testing.T) {
	fake := &fakeApp{
		entities: []app.EntityResult{attrEntity(inventory.EntityStatusOk, true, false)},
		attrErr:  fmt.Errorf("cannot lock %q: already locked (%w)", "Shelf", app.ErrNoChange),
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := postAttr(t, ts, "abc", "locked=true")

	require.Equal(t, http.StatusOK, resp.StatusCode, body)
	assert.Contains(t, body, "Already changed elsewhere; showing current state.")
	assert.Contains(t, body, ">Unlock<")
}

func TestHandleAttr_OtherErrorShowsGenericMessage(t *testing.T) {
	fake := &fakeApp{
		entities: []app.EntityResult{attrEntity(inventory.EntityStatusOk, false, false)},
		attrErr:  errTest,
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := postAttr(t, ts, "abc", "locked=true")

	require.Equal(t, http.StatusOK, resp.StatusCode, body)
	assert.Contains(t, body, "attribute change failed")
}

// #306: a panel left open after the entity was removed elsewhere shows the
// removed view instead of a silent 404, and drops the stale tree node. The
// entity is a root, so the detached trigger names no parent.
func TestHandleAttr_RemovedEntityShowsRemovedPanel(t *testing.T) {
	fake := &fakeApp{
		entities: []app.EntityResult{attrEntity(inventory.EntityStatusRemoved, false, false)},
		attrErr:  fmt.Errorf("set attributes: %w", app.ErrNotFound),
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := postAttr(t, ts, "abc", "locked=true")

	require.Equal(t, http.StatusOK, resp.StatusCode, body)
	assert.Contains(t, body, staleText)
	assert.Contains(t, body, "Last location")
	assert.NotContains(t, body, "/entities/abc/actions/")
	assert.Contains(t, body, `id="tree-node-abc" hx-swap-oob="delete"`)
	assert.Equal(t, detachedEvent{EntityID: "abc"}, detachedTrigger(t, resp))
}
