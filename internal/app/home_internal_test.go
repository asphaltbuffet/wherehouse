package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func TestFillHomePaths(t *testing.T) {
	results := []EntityResult{
		{EntityID: "drill", ParentID: "deck", HomeID: "garage", Status: inventory.EntityStatusOk},
		{EntityID: "bit", ParentID: "drill", HomeID: "drill", Status: inventory.EntityStatusOk},
	}
	fillHomePaths(results, map[string]string{"garage": "Garage", "deck": "Deck", "drill": "Deck:Drill"})
	assert.Equal(t, "Garage", results[0].HomePathDisplay)
	assert.Empty(t, results[1].HomePathDisplay, "not displaced: no lookup")
}
