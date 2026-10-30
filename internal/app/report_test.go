package app_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

var reportNow = time.Date(2026, 10, 30, 14, 2, 11, 0, time.UTC)

func TestBuildReport_SortsCaseInsensitiveByPath(t *testing.T) {
	all := []app.EntityResult{
		{EntityID: "4", FullPathDisplay: "garage:b", Status: inventory.EntityStatusOk},
		{EntityID: "1", FullPathDisplay: "Attic", Status: inventory.EntityStatusOk},
		{EntityID: "3", FullPathDisplay: "Garage:A", Status: inventory.EntityStatusOk},
		{EntityID: "2", FullPathDisplay: "attic:x", Status: inventory.EntityStatusOk},
	}
	r := app.BuildReport(all, app.EntityFilter{}, "Home", reportNow)
	assert.Equal(t, []string{"1", "2", "3", "4"}, filterIDs(r.Entities))
	assert.Equal(t, 4, r.Count())
}

func TestBuildReport_ExcludesRemoved(t *testing.T) {
	all := []app.EntityResult{
		{EntityID: "k", FullPathDisplay: "Kept", Status: inventory.EntityStatusOk},
		{EntityID: "r", FullPathDisplay: "Gone", Status: inventory.EntityStatusRemoved},
	}
	r := app.BuildReport(all, app.EntityFilter{}, "Home", reportNow)
	assert.Equal(t, []string{"k"}, filterIDs(r.Entities))
}

func TestBuildReport_NamesPresetOrCustom(t *testing.T) {
	loaned := inventory.EntityStatusLoaned.String()
	tests := []struct {
		name  string
		f     app.EntityFilter
		slug  string
		title string
		label string
	}{
		{"all", app.EntityFilter{}, "all", "All", "Everything"},
		{"loaned", app.EntityFilter{Status: loaned}, "loaned", "Loaned", "Loaned"},
		{"displaced", app.EntityFilter{Displaced: true}, "displaced", "Displaced", "Displaced"},
		{"by parent", app.EntityFilter{Under: "Garage"}, app.ReportSlugByParent, "By parent", "under Garage"},
		{
			"custom",
			app.EntityFilter{Status: loaned, Under: "Garage:Toolbox"},
			"custom", "Custom filter", "Loaned · under Garage:Toolbox",
		},
		{
			"displaced home under",
			app.EntityFilter{Displaced: true, HomeUnder: "Kitchen"},
			"custom", "Custom filter", "Displaced · home under Kitchen",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := app.BuildReport(nil, tt.f, "Home", reportNow)
			assert.Equal(t, tt.slug, r.Slug)
			assert.Equal(t, tt.title, r.Title)
			assert.Equal(t, tt.label, r.FilterLabel())
		})
	}
}

func TestReport_Filename(t *testing.T) {
	est := time.FixedZone("EST", -5*60*60)
	tests := []struct {
		name string
		f    app.EntityFilter
		ext  string
		want string
	}{
		{"unfiltered", app.EntityFilter{}, "json", "wherehouse-all-20261030T140211Z.json"},
		{
			"status and under",
			app.EntityFilter{Status: inventory.EntityStatusLoaned.String(), Under: "Garage:Tool Box"},
			"csv", "wherehouse-loaned-under-garage-tool-box-20261030T140211Z.csv",
		},
		{
			"displaced home under",
			app.EntityFilter{Displaced: true, HomeUnder: "Kitchen"},
			"csv", "wherehouse-displaced-home-under-kitchen-20261030T140211Z.csv",
		},
		{
			"non-ascii name keeps a valid name",
			app.EntityFilter{Under: "日本"},
			"csv", "wherehouse-under-20261030T140211Z.csv",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := app.BuildReport(nil, tt.f, "Home", reportNow.In(est))
			assert.Equal(t, tt.want, r.Filename(tt.ext))
		})
	}
}

func TestCountPresets(t *testing.T) {
	all := filterFixture()
	all = append(all, app.EntityResult{EntityID: "x", FullPathDisplay: "Gone", Status: inventory.EntityStatusRemoved})

	got := app.CountPresets(all)
	require.Len(t, got, 6)

	counts := map[string]int{}
	for _, p := range got {
		if p.Slug == app.ReportSlugByParent {
			assert.False(t, p.HasCount, "by parent has no badge")
			continue
		}
		assert.True(t, p.HasCount, p.Slug)
		counts[p.Slug] = p.Count
	}
	assert.Equal(t, map[string]int{"missing": 0, "loaned": 1, "borrowed": 0, "displaced": 1, "all": 4}, counts)
}
