package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func TestCheckHomeRows(t *testing.T) {
	ptr := func(s string) *string { return &s }
	rows := []*inventory.Entity{
		{EntityID: "garage", Status: inventory.EntityStatusOk},
		{EntityID: "box", Status: inventory.EntityStatusOk, Discrete: true},
		{EntityID: "gone", Status: inventory.EntityStatusRemoved},
		{EntityID: "ok1", ParentID: ptr("garage"), HomeID: ptr("garage"), Status: inventory.EntityStatusOk},
		{EntityID: "bad-removed", HomeID: ptr("gone"), Status: inventory.EntityStatusOk},
		{EntityID: "bad-discrete", HomeID: ptr("box"), Status: inventory.EntityStatusOk},
		{EntityID: "bad-self", HomeID: ptr("bad-self"), Status: inventory.EntityStatusOk},
		{EntityID: "bad-desc", HomeID: ptr("child"), Status: inventory.EntityStatusOk},
		{EntityID: "child", ParentID: ptr("bad-desc"), HomeID: ptr("bad-desc"), Status: inventory.EntityStatusOk},
		{EntityID: "removed-ignored", HomeID: ptr("gone"), Status: inventory.EntityStatusRemoved},
	}
	issues := checkHomeRows(rows)
	require.Len(t, issues, 4)
	for _, id := range []string{"bad-removed", "bad-discrete", "bad-self", "bad-desc"} {
		found := false
		for _, is := range issues {
			assert.Equal(t, DoctorKindProjection, is.Kind)
			if strings.HasPrefix(is.Description, "entity "+id+":") {
				found = true
			}
		}
		assert.True(t, found, id)
	}
}
