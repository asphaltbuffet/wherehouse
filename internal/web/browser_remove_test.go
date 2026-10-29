//go:build browser

package web_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func TestBrowserRemove(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		garage := s.createEntity("Garage", "")
		drill := s.createEntity("Drill", garage.EntityID)
		s.createEntity("Hammer", garage.EntityID)

		// Load Drill's tree node first, so its removal must delete it.
		s.visit("/entities/" + garage.EntityID)
		s.openTree()
		s.expandTreeItem("Garage")
		require.NoError(s.t, expect.Locator(s.treeItem("Drill")).ToBeVisible())
		s.closeTree()

		s.click(s.childLink("Drill"))
		require.NoError(s.t, expect.Locator(s.detailHeading("Drill")).ToBeVisible())
		s.click(s.button("Remove"))
		s.click(s.button("Remove")) // the form's confirm button

		require.NoError(s.t, expect.Locator(s.detailHeading("Garage")).ToBeVisible())
		require.NoError(s.t, expect.Locator(s.childLink("Hammer")).ToBeVisible())
		require.NoError(s.t, expect.Locator(s.childLink("Drill")).ToHaveCount(0))
		s.openTree()
		require.NoError(s.t, expect.Locator(s.treeItem("Hammer")).ToBeVisible())
		require.NoError(s.t, expect.Locator(s.treeItem("Drill")).ToHaveCount(0))

		assert.Equal(s.t, inventory.EntityStatusRemoved, s.stored(drill.EntityID).Status)
	})
}
