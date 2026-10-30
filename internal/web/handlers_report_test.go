package web_test

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/web"
)

// reportFixture: Garage holds a loaned Drill; the Ladder belongs in the
// Garage but is in the Attic; Gone is removed and must never show.
func reportFixture() *fakeApp {
	return &fakeApp{
		name: "Home inventory",
		entities: []app.EntityResult{
			{
				EntityID: "garage0001", DisplayName: "Garage",
				FullPathDisplay: "Garage", Status: inventory.EntityStatusOk,
			},
			{
				EntityID: "drill00001", DisplayName: "Drill", ParentID: "garage0001", HomeID: "garage0001",
				FullPathDisplay: "Garage:Drill", Status: inventory.EntityStatusLoaned, StatusContext: "Sam",
			},
			{
				EntityID: "attic00001", DisplayName: "Attic",
				FullPathDisplay: "Attic", Status: inventory.EntityStatusOk,
			},
			{
				EntityID: "ladder0001", DisplayName: "Ladder", ParentID: "attic00001", HomeID: "garage0001",
				HomePathDisplay: "Garage", FullPathDisplay: "Attic:Ladder", Status: inventory.EntityStatusOk,
			},
			{
				EntityID: "gone000001", DisplayName: "Gone", ParentID: "garage0001", HomeID: "garage0001",
				FullPathDisplay: "Garage:Gone", Status: inventory.EntityStatusRemoved,
			},
		},
	}
}

func getPath(t *testing.T, a web.App, path string) (*http.Response, string) {
	t.Helper()
	ts := newTestServer(t, a)
	t.Cleanup(ts.Close)
	resp, err := ts.Client().Get(ts.URL + path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp, readBody(t, resp)
}

func TestHandleReports_LandingShowsAllWithCounts(t *testing.T) {
	resp, body := getPath(t, reportFixture(), "/reports")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	assert.Contains(t, body,
		`<a class="report-preset" href="/reports?status=loaned">Loaned <span class="report-count">1</span></a>`)
	assert.Contains(t, body,
		`<a class="report-preset" href="/reports" aria-current="page">All <span class="report-count">4</span></a>`)
	assert.Contains(t, body, `<label class="report-preset" for="report_under_q">By parent</label>`)
	assert.Contains(t, body, "Home inventory")
	assert.Contains(t, body, "Everything")
	assert.Contains(t, body, "Garage:Drill")
	assert.Contains(t, body, "Attic:Ladder")
	assert.NotContains(t, body, "Garage:Gone")
}

// The picker search boxes belong to an empty hidden form, so the filter form
// never submits their q and the markup names a real form.
func TestHandleReports_PickerSearchFormExists(t *testing.T) {
	_, body := getPath(t, reportFixture(), "/reports")
	assert.Contains(t, body, `<form id="report-picker-search" hidden></form>`)
	assert.Equal(t, 2, strings.Count(body, `form="report-picker-search"`))
}

func TestHandleReports_StatusPreset(t *testing.T) {
	_, body := getPath(t, reportFixture(), "/reports?status=loaned")
	assert.Contains(t, body, `href="/reports?status=loaned" aria-current="page"`)
	assert.Contains(t, body, "Garage:Drill")
	assert.Contains(t, body, "Sam")
	assert.NotContains(t, body, "Attic:Ladder")
	assert.Contains(t, body, `href="/reports?format=csv&amp;status=loaned"`)
	assert.Contains(t, body, `href="/reports?format=json&amp;status=loaned"`)
}

func TestHandleReports_DisplacedShowsHome(t *testing.T) {
	_, body := getPath(t, reportFixture(), "/reports?displaced=1")
	assert.Contains(t, body, "Attic:Ladder")
	assert.Contains(t, body, `<td data-label="Home">Garage</td>`)
	assert.NotContains(t, body, "Garage:Drill")
}

func TestHandleReports_UnderResolvesIDToPath(t *testing.T) {
	_, body := getPath(t, reportFixture(), "/reports?under=garage0001")
	assert.Contains(t, body, `<h3 class="report-title">By parent</h3>`)
	assert.Contains(t, body, `<p class="report-filter-label">under Garage</p>`)
	assert.Contains(t, body, `name="under" value="garage0001"`)
	assert.Contains(t, body, "Garage:Drill")
	assert.NotContains(t, body, "Attic:Ladder")
}

func TestHandleReports_CustomFilter(t *testing.T) {
	_, body := getPath(t, reportFixture(), "/reports?status=ok&home_under=garage0001")
	assert.Contains(t, body, "Custom filter")
	assert.Contains(t, body, "Ok · home under Garage")
	assert.Contains(t, body, "Attic:Ladder")
}

func TestHandleReports_EmptyState(t *testing.T) {
	_, body := getPath(t, reportFixture(), "/reports?status=missing")
	assert.Contains(t, body, "No entities match.")
	assert.NotContains(t, body, `<table class="report-table">`)
}

func TestHandleReports_UnknownUnderIs404(t *testing.T) {
	for _, q := range []string{
		"under=nope", "home_under=nope", "under=gone000001",
		"under=nope&format=csv", "home_under=nope&format=json",
	} {
		resp, body := getPath(t, reportFixture(), "/reports?"+q)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, q)
		assert.Contains(t, body, "entity not found", q)
	}
}

func TestHandleReports_BadParamsAre400(t *testing.T) {
	for _, q := range []string{
		"status=removed", "status=bogus", "format=xml", "displaced=0", "displaced=false",
		"under=" + strings.Repeat("x", 101),
	} {
		resp, body := getPath(t, reportFixture(), "/reports?"+q)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, q)
		assert.Contains(t, body, "bad report parameter", q)
	}
}

func TestHandleReports_ListErrorIs500(t *testing.T) {
	fake := reportFixture()
	fake.err = errTest
	resp, _ := getPath(t, fake, "/reports")
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestHandleReports_NameErrorIs500(t *testing.T) {
	fake := reportFixture()
	fake.nameErr = errTest
	resp, _ := getPath(t, fake, "/reports")
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestReportLinks_HeaderAndDetail(t *testing.T) {
	_, index := getPath(t, reportFixture(), "/")
	assert.Contains(t, index, `<a class="app-nav-link" href="/reports">Reports</a>`)

	_, detail := getPath(t, reportFixture(), "/entities/garage0001")
	assert.Contains(t, detail, `href="/reports?under=garage0001"`)
}

func TestHandleReports_CSVDownload(t *testing.T) {
	resp, body := getPath(t, reportFixture(), "/reports?status=loaned&under=garage0001&format=csv")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/csv; charset=utf-8", resp.Header.Get("Content-Type"))
	assert.Regexp(t,
		`^attachment; filename=wherehouse-loaned-under-garage-\d{8}T\d{6}Z\.csv$`,
		resp.Header.Get("Content-Disposition"))

	rows, err := csv.NewReader(strings.NewReader(body)).ReadAll()
	require.NoError(t, err)
	assert.Equal(t, [][]string{
		{"path", "status", "status_context", "home", "locked", "discrete", "tags", "entity_id"},
		{"Garage:Drill", "loaned", "Sam", "", "false", "false", "", "drill00001"},
	}, rows)
}

func TestHandleReports_JSONDownload(t *testing.T) {
	resp, body := getPath(t, reportFixture(), "/reports?displaced=1&format=json")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	assert.Regexp(t,
		`^attachment; filename=wherehouse-displaced-\d{8}T\d{6}Z\.json$`,
		resp.Header.Get("Content-Disposition"))

	var out app.ReportOutput
	require.NoError(t, json.Unmarshal([]byte(body), &out))
	assert.Equal(t, "displaced", out.Report)
	assert.Equal(t, "Home inventory", out.Database)
	assert.True(t, out.Filter.Displaced)
	assert.Equal(t, 1, out.Count)
	require.Len(t, out.Entities, 1)
	assert.Equal(t, "Attic:Ladder", out.Entities[0].Path)
	assert.Equal(t, "Garage", out.Entities[0].Home)
	assert.Regexp(t, `^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ$`, out.GeneratedAt)
}

func TestHandleReports_EmptyExportStillHasHeader(t *testing.T) {
	_, body := getPath(t, reportFixture(), "/reports?status=missing&format=csv")
	assert.Equal(t, "path,status,status_context,home,locked,discrete,tags,entity_id\n", body)
}
