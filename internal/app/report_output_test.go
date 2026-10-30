package app_test

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func outputReport(entities ...app.EntityResult) app.Report {
	return app.BuildReport(entities, app.EntityFilter{Displaced: true, Under: "Attic"}, "Home inventory", reportNow)
}

var displacedLadder = app.EntityResult{
	EntityID: "l", ParentID: "a", HomeID: "g", HomePathDisplay: "Garage",
	FullPathDisplay: "Attic:Ladder", Status: inventory.EntityStatusLoaned, StatusContext: "Sam",
	Locked: false, Discrete: true, Tags: []string{"tall", "metal"},
}

func TestToReportOutput_JSON(t *testing.T) {
	b, err := json.Marshal(app.ToReportOutput(outputReport(displacedLadder)))
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"report": "custom",
		"filter": {"under": "Attic", "displaced": true},
		"database": "Home inventory",
		"generated_at": "2026-10-30T14:02:11Z",
		"count": 1,
		"entities": [{
			"path": "Attic:Ladder", "status": "loaned", "status_context": "Sam",
			"home": "Garage", "locked": false, "discrete": true,
			"tags": ["tall", "metal"], "entity_id": "l"
		}]
	}`, string(b))
}

func TestToReportOutput_EmptyIsArrays(t *testing.T) {
	b, err := json.Marshal(app.ToReportOutput(outputReport()))
	require.NoError(t, err)
	assert.Contains(t, string(b), `"entities":[]`)
	assert.Contains(t, string(b), `"count":0`)
}

func TestWriteReportCSV(t *testing.T) {
	atHome := app.EntityResult{
		EntityID: "b", ParentID: "a", HomeID: "a",
		FullPathDisplay: "Attic:Box", Status: inventory.EntityStatusOk,
	}
	r := app.BuildReport([]app.EntityResult{displacedLadder, atHome}, app.EntityFilter{}, "Home", reportNow)

	var buf bytes.Buffer
	require.NoError(t, app.WriteReportCSV(&buf, r))

	rows, err := csv.NewReader(&buf).ReadAll()
	require.NoError(t, err)
	assert.Equal(t, [][]string{
		{"path", "status", "status_context", "home", "locked", "discrete", "tags", "entity_id"},
		{"Attic:Box", "ok", "", "", "false", "false", "", "b"},
		{"Attic:Ladder", "loaned", "Sam", "Garage", "false", "true", "tall;metal", "l"},
	}, rows)
}

func TestWriteReportCSV_NeutralizesFormulas(t *testing.T) {
	evil := app.EntityResult{
		EntityID: "e", FullPathDisplay: `=HYPERLINK("http://x","y")`,
		Status: inventory.EntityStatusLoaned, StatusContext: "+cmd", Tags: []string{"@x"},
	}
	r := app.BuildReport([]app.EntityResult{evil}, app.EntityFilter{}, "Home", reportNow)

	var buf bytes.Buffer
	require.NoError(t, app.WriteReportCSV(&buf, r))
	rows, err := csv.NewReader(&buf).ReadAll()
	require.NoError(t, err)

	assert.Equal(t, `'=HYPERLINK("http://x","y")`, rows[1][0])
	assert.Equal(t, "'+cmd", rows[1][2])
	assert.Equal(t, "'@x", rows[1][6])
}
