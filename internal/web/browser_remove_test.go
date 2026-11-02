//go:build browser

package web_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
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

// A focused tree row that is removed hands focus to its next sibling, then
// its previous sibling, then its parent (#304). Wide only: the narrow drawer
// is closed while the detail pane is in use.
func TestBrowserRemoveFocusedTreeNode(t *testing.T) {
	t.Parallel()
	forDevices(t, devicesIn(wide), sessionConfig{}, func(s *session) {
		garage := s.createEntity("Garage", "")
		drill := s.createEntity("Drill", garage.EntityID)
		hammer := s.createEntity("Hammer", garage.EntityID)
		saw := s.createEntity("Saw", garage.EntityID)

		s.visit("/entities/" + garage.EntityID)
		s.expandTreeItem("Garage")

		// removeFocused opens e's remove form, focuses e's tree row, then
		// submits the form without moving focus off the row.
		removeFocused := func(e app.EntityResult) {
			s.t.Helper()
			s.visit("/entities/" + e.EntityID)
			s.click(s.button("Remove"))
			s.settle()
			require.NoError(s.t, s.treeRow(e.DisplayName).Focus())
			_, err := s.page.Evaluate(`document.querySelector('form[data-action="remove"]').requestSubmit()`)
			require.NoError(s.t, err)
			require.NoError(s.t, expect.Locator(s.treeItem(e.DisplayName)).ToHaveCount(0))
		}

		removeFocused(hammer)
		require.NoError(s.t, expect.Locator(s.treeRow("Saw")).ToBeFocused())

		removeFocused(saw)
		require.NoError(s.t, expect.Locator(s.treeRow("Drill")).ToBeFocused())

		removeFocused(drill)
		require.NoError(s.t, expect.Locator(s.treeRow("Garage")).ToBeFocused())
	})
}
