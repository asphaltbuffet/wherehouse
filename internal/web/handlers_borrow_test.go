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
