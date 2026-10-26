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
	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func TestHandleBorrowForm_DefaultParent(t *testing.T) {
	entities := []app.EntityResult{
		{EntityID: "g", DisplayName: "Garage", FullPathDisplay: "Garage", Status: inventory.EntityStatusOk},
		{EntityID: "nails", DisplayName: "Box of Nails", FullPathDisplay: "Garage:Box of Nails",
			ParentID: "g", Discrete: true, Status: inventory.EntityStatusOk},
		{EntityID: "old", DisplayName: "Old Shelf", FullPathDisplay: "Garage:Old Shelf",
			ParentID: "g", Status: inventory.EntityStatusRemoved},
		{EntityID: "rootbox", DisplayName: "Crate", FullPathDisplay: "Crate",
			Discrete: true, Status: inventory.EntityStatusOk},
		{EntityID: "case", DisplayName: "Drill Case", FullPathDisplay: "Drill Case",
			Status: inventory.EntityStatusBorrowed, StatusContext: "Alice"},
	}
	tests := []struct {
		name       string
		currentURL string // path appended to ts.URL; "" sends no header
		rawHeader  string // used verbatim when non-empty
		wantID     string
		wantLabel  string
	}{
		{"eligible current entity", "/entities/g", "", "g", "Garage"},
		{"borrowed entity is eligible", "/entities/case", "", "case", "Drill Case"},
		{"discrete falls back to parent", "/entities/nails", "", "g", "Garage"},
		{"removed falls back to parent", "/entities/old", "", "g", "Garage"},
		{"discrete root falls back to root", "/entities/rootbox", "", "", "(root)"},
		{"no header", "", "", "", "(root)"},
		{"index page", "/", "", "", "(root)"},
		{"search page", "/search?q=x", "", "", "(root)"},
		{"sub-route", "/entities/g/edit/name", "", "", "(root)"},
		{"unknown id", "/entities/nope", "", "", "(root)"},
		{"malformed url", "", "%zz", "", "(root)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newTestServer(t, &fakeApp{entities: entities})
			defer ts.Close()

			req, err := http.NewRequest(http.MethodGet, ts.URL+"/entities/borrow", nil)
			require.NoError(t, err)
			req.Header.Set("Hx-Request", "true")
			switch {
			case tt.rawHeader != "":
				req.Header.Set("Hx-Current-Url", tt.rawHeader)
			case tt.currentURL != "":
				req.Header.Set("Hx-Current-Url", ts.URL+tt.currentURL)
			}
			resp, err := ts.Client().Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)

			require.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Contains(t, string(body), `name="parent_id" value="`+tt.wantID+`"`)
			assert.Contains(t, string(body), `<span id="borrow_parent_label">`+tt.wantLabel+`</span>`)
		})
	}
}

func TestHeader_HasBorrowButton(t *testing.T) {
	ts := newTestServer(t, &fakeApp{})
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	assert.Contains(t, string(body), `hx-get="/entities/borrow"`)
	assert.Contains(t, string(body), ">Borrow</button>")
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
			{EntityID: "n", DisplayName: "Drill", FullPathDisplay: "Garage:Drill", ParentID: "g",
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
	assert.Equal(t, "Garage", fake.lastBorrowReq.ParentPath)
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
	assert.Empty(t, fake.lastBorrowReq.ParentPath)
	assert.Contains(t, resp.Header.Get("Hx-Trigger"), `"parentId":""`)
}

// Review Focus #3: parent removed between opening the form and submitting it.
func TestHandleBorrow_StaleParent(t *testing.T) {
	fake := &fakeApp{entities: []app.EntityResult{
		{EntityID: "gone", DisplayName: "Old Shelf", FullPathDisplay: "Old Shelf",
			Status: inventory.EntityStatusRemoved},
	}}
	ts := newTestServer(t, fake)
	defer ts.Close()

	resp, body := postBorrow(t, ts, url.Values{"display_name": {"Drill"}, "from": {"Alice"}, "parent_id": {"gone"}})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "#add-modal-body", resp.Header.Get("Hx-Retarget"))
	assert.Equal(t, "innerHTML", resp.Header.Get("Hx-Reswap"))
	assert.Contains(t, body, "no longer exists")
	assert.Contains(t, body, `name="parent_id" value=""`)
	assert.Zero(t, fake.borrowCalls)
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
