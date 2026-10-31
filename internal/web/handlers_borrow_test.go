package web_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/web"
)

func TestHandleBorrowForm_DefaultParent(t *testing.T) {
	entities := []app.EntityResult{
		{EntityID: "g", DisplayName: "Garage", FullPathDisplay: "Garage", Status: inventory.EntityStatusOk},
		{EntityID: "nails", DisplayName: "Box of Nails", FullPathDisplay: "Garage:Box of Nails",
			ParentID: "g", HomeID: "g", Discrete: true, Status: inventory.EntityStatusOk},
		{EntityID: "old", DisplayName: "Old Shelf", FullPathDisplay: "Garage:Old Shelf",
			ParentID: "g", HomeID: "g", Status: inventory.EntityStatusRemoved},
		{EntityID: "rootbox", DisplayName: "Crate", FullPathDisplay: "Crate",
			Discrete: true, Status: inventory.EntityStatusOk},
		{EntityID: "case", DisplayName: "Drill Case", FullPathDisplay: "Drill Case",
			Status: inventory.EntityStatusBorrowed, StatusContext: "Alice"},
	}
	tests := []struct {
		name      string
		parent    string // value of ?parent=; "" sends none
		wantID    string
		wantLabel string
	}{
		{"eligible entity", "g", "g", "Garage"},
		{"borrowed entity is eligible", "case", "case", "Drill Case"},
		{"discrete falls back to parent", "nails", "g", "Garage"},
		{"removed falls back to parent", "old", "g", "Garage"},
		{"discrete root falls back to root", "rootbox", "", "(root)"},
		{"no param", "", "", "(root)"},
		{"unknown id", "nope", "", "(root)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t, &fakeApp{entities: entities})
			defer ts.Close()

			target := ts.URL + "/entities/borrow"
			if tt.parent != "" {
				target += "?parent=" + url.QueryEscape(tt.parent)
			}
			req, err := http.NewRequest(http.MethodGet, target, nil)
			require.NoError(t, err)
			req.Header.Set("Hx-Request", "true")
			resp, err := ts.Client().Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)

			require.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Contains(t, string(body), `name="parent_id" value="`+tt.wantID+`"`)
			assert.Contains(t, string(body), `<span id="borrow_parent_label" data-parent-label>`+tt.wantLabel+`</span>`)
		})
	}
}

func TestBorrowActions(t *testing.T) {
	fake := &fakeApp{entities: []app.EntityResult{
		{EntityID: "g", DisplayName: "Garage", FullPathDisplay: "Garage", Status: inventory.EntityStatusOk},
		{EntityID: "n", DisplayName: "Nails", FullPathDisplay: "Nails", Discrete: true,
			Status: inventory.EntityStatusOk},
	}}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()
	home := readBody(t, resp)
	_, afterOpen, _ := strings.Cut(home, "<header")
	header, _, _ := strings.Cut(afterOpen, "</header>")

	assert.NotContains(t, header, "/entities/borrow", "header has no Borrow button")
	assert.NotContains(t, header, "Add root", "header has no Add root button")
	assert.NotContains(t, header, "acting as")
	assert.Contains(t, home,
		`hx-get="/entities/borrow" hx-target="#add-modal-body" hx-swap="innerHTML">Borrow</button>`)

	assert.Contains(t, getDetail(t, ts, "g"),
		`hx-get="/entities/borrow?parent=g" hx-target="#add-modal-body" hx-swap="innerHTML">Borrow into here</button>`)
	assert.NotContains(t, getDetail(t, ts, "n"), "Borrow into here", "discrete entities cannot receive a borrow")
}

func postBorrow(t *testing.T, ts *httptest.Server, form url.Values) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/entities/borrow", strings.NewReader(form.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Hx-Request", "true")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, string(body)
}

func TestHandleBorrow_CreatesUnderParent(t *testing.T) {
	fake := &fakeApp{
		entities: []app.EntityResult{
			{EntityID: "g", DisplayName: "Garage", FullPathDisplay: "Garage", Status: inventory.EntityStatusOk},
		},
		borrowResults: []app.EntityResult{
			{EntityID: "n", DisplayName: "Drill", FullPathDisplay: "Garage:Drill", ParentID: "g", HomeID: "g",
				Status: inventory.EntityStatusBorrowed, StatusContext: "Alice"},
		},
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := postBorrow(t, ts, url.Values{
		"display_name": {" Drill "}, "from": {"Alice"}, "parent_id": {"g"}, "note": {"weekend"},
	})

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "Drill", fake.lastBorrowReq.DisplayName)
	assert.Equal(t, "g", fake.lastBorrowReq.ParentID)
	assert.Equal(t, "Alice", fake.lastBorrowReq.StatusContext)
	assert.Equal(t, "weekend", fake.lastBorrowReq.Note)
	assert.Equal(t, "/entities/n", resp.Header.Get("Hx-Push-Url"))
	assert.Empty(t, resp.Header.Get("Hx-Retarget"))

	var trigger map[string]map[string]string
	require.NoError(t, json.Unmarshal([]byte(resp.Header.Get("Hx-Trigger")), &trigger))
	assert.Equal(t, map[string]string{"entityId": "n", "parentId": "g"}, trigger["entity-created"])
	assert.Contains(t, body, "<h2>Drill</h2>")
}

func TestHandleBorrow_Root(t *testing.T) {
	fake := &fakeApp{borrowResults: []app.EntityResult{
		{EntityID: "n", DisplayName: "Ladder", FullPathDisplay: "Ladder", Status: inventory.EntityStatusBorrowed},
	}}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, _ := postBorrow(t, ts, url.Values{"display_name": {"Ladder"}, "from": {"Bob"}, "parent_id": {""}})

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Empty(t, fake.lastBorrowReq.ParentID)
	assert.Contains(t, resp.Header.Get("Hx-Trigger"), `"parentId":""`)
}

// Review Focus #3: parent removed between opening the form and submitting it.
func TestHandleBorrow_StaleParent(t *testing.T) {
	fake := &fakeApp{borrowErr: fmt.Errorf("get entity %q: %w", "gone", app.ErrNotFound)}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := postBorrow(t, ts, url.Values{"display_name": {"Drill"}, "from": {"Alice"}, "parent_id": {"gone"}})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "#add-modal-body", resp.Header.Get("Hx-Retarget"))
	assert.Equal(t, "innerHTML", resp.Header.Get("Hx-Reswap"))
	assert.Contains(t, body, "no longer exists")
	assert.Contains(t, body, `name="parent_id" value=""`)
	assert.Equal(t, "gone", fake.lastBorrowReq.ParentID, "ID passed straight through, no pre-fetch")
}

func TestHandleBorrow_DiscreteParent(t *testing.T) {
	fake := &fakeApp{
		entities: []app.EntityResult{
			{
				EntityID:        "box",
				DisplayName:     "Box",
				FullPathDisplay: "Garage:Box",
				Discrete:        true,
				Status:          inventory.EntityStatusOk,
			},
		},
		borrowErr: fmt.Errorf("%q is discrete: %w", "Garage:Box", app.ErrInvalidDestination),
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := postBorrow(t, ts, url.Values{"display_name": {"Drill"}, "from": {"Alice"}, "parent_id": {"box"}})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, body, "can&#39;t hold items")
	assert.Contains(
		t,
		body,
		`name="parent_id" value=""`,
		"ineligible parent is cleared so a resubmit can't repeat the error",
	)
	assert.Contains(t, body, `data-parent-label>(root)</span>`)
}

// A failed label lookup clears the parent like a stale one, but is logged.
func TestHandleBorrow_ParentLabelLookupFailureIsLogged(t *testing.T) {
	fake := &fakeApp{
		err:       errTest, // GetEntityByID fails with a non-NotFound error
		borrowErr: fmt.Errorf("borrow: %w", app.ErrLenderRequired),
	}
	logger := &recordingLogger{}
	srv, err := web.New(web.Config{App: fake, Bind: "127.0.0.1", Output: io.Discard, Logger: logger})
	require.NoError(t, err)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	_, body := postBorrow(t, ts, url.Values{"display_name": {"Drill"}, "from": {" "}, "parent_id": {"g"}})

	assert.Contains(t, body, `name="parent_id" value=""`)
	assert.Contains(t, logger.errors, "borrow form parent label")
}

// Expected user mistakes are not logged as errors; unexpected failures are.
func TestHandleBorrow_ErrorLogLevel(t *testing.T) {
	tests := []struct {
		name      string
		borrowErr error
		wantError bool
	}{
		{"blank lender", fmt.Errorf("borrow: %w", app.ErrLenderRequired), false},
		{"invalid name", fmt.Errorf("borrow: %w", entitypath.ErrSegmentContainsSeparator), false},
		{"unexpected", errTest, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &recordingLogger{}
			srv, err := web.New(web.Config{
				App:    &fakeApp{borrowErr: tt.borrowErr},
				Bind:   "127.0.0.1",
				Output: io.Discard,
				Logger: logger,
			})
			require.NoError(t, err)
			ts := httptest.NewServer(srv.Handler())
			defer ts.Close()

			postBorrow(t, ts, url.Values{"display_name": {"Drill"}, "from": {"Alice"}})

			assert.Equal(t, tt.wantError, slices.Contains(logger.errors, "borrow entity"))
		})
	}
}

// Review Focus #4: an unrelated error keeps the chosen parent and its label.
func TestHandleBorrow_ErrorKeepsParentLabel(t *testing.T) {
	fake := &fakeApp{
		entities: []app.EntityResult{
			{EntityID: "g", DisplayName: "Garage", FullPathDisplay: "Garage", Status: inventory.EntityStatusOk},
		},
		borrowErr: fmt.Errorf("borrow: %w", app.ErrLenderRequired),
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	_, body := postBorrow(t, ts, url.Values{"display_name": {"Drill"}, "from": {" "}, "parent_id": {"g"}})

	assert.Contains(t, body, "Borrowed from is required")
	assert.Contains(t, body, `name="parent_id" value="g"`)
	assert.Contains(t, body, `data-parent-label>Garage</span>`)
}

// Review Focus #1 and #2: app validation errors become inline form errors.
func TestHandleBorrow_AppValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantMsg string
	}{
		{"blank lender", app.ErrLenderRequired, "Borrowed from is required"},
		{"empty name", entitypath.ErrEmptySegment, "Name is required"},
		{"separator", entitypath.ErrSegmentContainsSeparator, "Name cannot contain"},
		{"invalid", entitypath.ErrInvalidSegment, "Name contains invalid characters"},
		{"other", errTest, "borrow failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeApp{borrowErr: fmt.Errorf("borrow: %w", tt.err)}
			ts := newTestServer(t, fake)
			defer ts.Close()

			resp, body := postBorrow(t, ts, url.Values{"display_name": {"Drill"}, "from": {"   "}})

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "#add-modal-body", resp.Header.Get("Hx-Retarget"))
			assert.Contains(t, body, tt.wantMsg)
			assert.Contains(t, body, `value="Drill"`, "entered name is echoed back")
			assert.Empty(t, resp.Header.Get("Hx-Trigger"))
		})
	}
}

// The tree must follow where the entity was actually created, not the submitted
// parent_id: path resolution can pick a different same-named sibling.
func TestHandleBorrow_TriggerUsesCreatedParent(t *testing.T) {
	fake := &fakeApp{
		entities: []app.EntityResult{
			{EntityID: "binA", DisplayName: "Bin", FullPathDisplay: "Bin", Status: inventory.EntityStatusOk},
			{EntityID: "binB", DisplayName: "Bin", FullPathDisplay: "Bin", Status: inventory.EntityStatusOk},
		},
		borrowResults: []app.EntityResult{
			{EntityID: "n", DisplayName: "Drill", FullPathDisplay: "Bin:Drill", ParentID: "binA", HomeID: "binA",
				Status: inventory.EntityStatusBorrowed, StatusContext: "Alice"},
		},
	}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, _ := postBorrow(t, ts, url.Values{"display_name": {"Drill"}, "from": {"Alice"}, "parent_id": {"binB"}})

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Hx-Trigger"), `"parentId":"binA"`)
}
