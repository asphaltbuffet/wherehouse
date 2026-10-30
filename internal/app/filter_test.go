package app_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// filterFixture: Garage holds a loaned Box (at home); the Ladder lives in the
// Garage but is displaced to the Attic.
func filterFixture() []app.EntityResult {
	return []app.EntityResult{
		{EntityID: "g", FullPathDisplay: "Garage", Status: inventory.EntityStatusOk},
		{
			EntityID: "b", ParentID: "g", HomeID: "g",
			FullPathDisplay: "Garage:Box", Status: inventory.EntityStatusLoaned,
		},
		{EntityID: "a", FullPathDisplay: "Attic", Status: inventory.EntityStatusOk},
		{
			EntityID: "l", ParentID: "a", HomeID: "g", HomePathDisplay: "Garage",
			FullPathDisplay: "Attic:Ladder", Status: inventory.EntityStatusOk,
		},
	}
}

func filterIDs(rs []app.EntityResult) []string {
	var ids []string
	for _, r := range rs {
		ids = append(ids, r.EntityID)
	}
	return ids
}

func TestFilterEntities(t *testing.T) {
	tests := []struct {
		name string
		f    app.EntityFilter
		want []string
	}{
		{"zero matches all in order", app.EntityFilter{}, []string{"g", "b", "a", "l"}},
		{"status", app.EntityFilter{Status: inventory.EntityStatusLoaned.String()}, []string{"b"}},
		{"under is inclusive", app.EntityFilter{Under: "Garage"}, []string{"g", "b"}},
		{"under is segment-wise", app.EntityFilter{Under: "Gar"}, nil},
		{"displaced", app.EntityFilter{Displaced: true}, []string{"l"}},
		{"home under uses home, not location", app.EntityFilter{HomeUnder: "Garage"}, []string{"b", "l"}},
		{
			"criteria AND together",
			app.EntityFilter{Under: "Garage", Status: inventory.EntityStatusLoaned.String()},
			[]string{"b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, filterIDs(app.FilterEntities(filterFixture(), tt.f)))
		})
	}
}
