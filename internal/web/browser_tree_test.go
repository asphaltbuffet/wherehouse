//go:build browser

package web_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBrowserTreeNavigation(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		garage := s.createEntity("Garage", "")
		toolbox := s.createEntity("Toolbox", garage.EntityID)
		s.createEntity("Drawer", toolbox.EntityID)

		s.visit("/")
		s.openTree()
		s.expandTreeItem("Garage")
		s.expandTreeItem("Toolbox")
		s.click(s.treeRow("Drawer"))

		require.NoError(s.t, expect.Locator(s.detailHeading("Drawer")).ToBeVisible())
		if s.layout == narrow {
			// Loading a detail from the tree closes the drawer to show it.
			require.NoError(s.t, expect.Locator(s.treeToggle()).ToHaveAttribute("aria-expanded", "false"))
			// A closed drawer hides its items from role locators; reopen it.
			s.openTree()
		}
		require.NoError(s.t, expect.Locator(s.treeItem("Drawer")).ToHaveAttribute("aria-selected", "true"))
	})
}

// Navigating to an entity whose tree row is not rendered expands the tree
// along its ancestors and selects it (#363).
func TestBrowserTreeFollowsChildrenList(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		garage := s.createEntity("Garage", "")
		toolbox := s.createEntity("Toolbox", garage.EntityID)
		s.createEntity("Drawer", toolbox.EntityID)

		s.visit("/")
		s.click(s.childLink("Garage"))
		s.click(s.childLink("Toolbox"))
		s.click(s.childLink("Drawer"))
		require.NoError(s.t, expect.Locator(s.detailHeading("Drawer")).ToBeVisible())

		s.settle()
		s.openTree()
		require.NoError(s.t, expect.Locator(s.treeItem("Garage")).ToHaveAttribute("aria-expanded", "true"))
		require.NoError(s.t, expect.Locator(s.treeItem("Toolbox")).ToHaveAttribute("aria-expanded", "true"))
		require.NoError(s.t, expect.Locator(s.treeItem("Drawer")).ToHaveAttribute("aria-selected", "true"))
	})
}

func TestBrowserTreeFollowsDirectURL(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		garage := s.createEntity("Garage", "")
		toolbox := s.createEntity("Toolbox", garage.EntityID)
		drawer := s.createEntity("Drawer", toolbox.EntityID)

		s.visit("/entities/" + drawer.EntityID)
		require.NoError(s.t, expect.Locator(s.detailHeading("Drawer")).ToBeVisible())

		s.settle()
		s.openTree()
		require.NoError(s.t, expect.Locator(s.treeItem("Drawer")).ToHaveAttribute("aria-selected", "true"))
	})
}
