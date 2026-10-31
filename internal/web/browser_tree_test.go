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

// Clicking a row's label selects it and leaves its expansion alone (#364).
func TestBrowserTreeLabelSelectsWithoutToggling(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		shed := s.createEntity("Shed", "")
		s.createEntity("Bin", shed.EntityID)
		garage := s.createEntity("Garage", "")
		s.createEntity("Toolbox", garage.EntityID)

		s.visit("/")
		s.openTree()

		// Collapsed container: selected, stays collapsed, children never fetched.
		s.click(s.treeLabel("Shed"))
		require.NoError(s.t, expect.Locator(s.detailHeading("Shed")).ToBeVisible())
		if s.layout == narrow {
			require.NoError(s.t, expect.Locator(s.treeToggle()).ToHaveAttribute("aria-expanded", "false"))
			s.openTree()
		}
		require.NoError(s.t, expect.Locator(s.treeItem("Shed")).ToHaveAttribute("aria-selected", "true"))
		require.NoError(s.t, expect.Locator(s.treeItem("Shed")).ToHaveAttribute("aria-expanded", "false"))
		require.NoError(s.t, expect.Locator(s.treeItem("Bin")).ToHaveCount(0))

		// Expanded container: selected, stays expanded.
		s.expandTreeItem("Garage")
		s.click(s.treeLabel("Garage"))
		if s.layout == narrow {
			require.NoError(s.t, expect.Locator(s.treeToggle()).ToHaveAttribute("aria-expanded", "false"))
		}
		require.NoError(s.t, expect.Locator(s.detailHeading("Garage")).ToBeVisible())
		s.settle()
		s.openTree()
		require.NoError(s.t, expect.Locator(s.treeItem("Garage")).ToHaveAttribute("aria-selected", "true"))
		require.NoError(s.t, expect.Locator(s.treeItem("Garage")).ToHaveAttribute("aria-expanded", "true"))
		require.NoError(s.t, expect.Locator(s.treeItem("Toolbox")).ToBeVisible())
	})
}

// A chevron click expands or collapses without touching the selection, the
// detail panel, or the narrow drawer (#364, ADR 0037).
func TestBrowserTreeChevronTogglesWithoutSelecting(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		shed := s.createEntity("Shed", "")
		garage := s.createEntity("Garage", "")
		s.createEntity("Toolbox", garage.EntityID)

		s.visit("/entities/" + shed.EntityID)
		s.settle()
		s.openTree()

		chevron := s.treeRow("Garage").Locator(".chevron")
		for _, want := range []string{"true", "false"} {
			s.click(chevron)
			require.NoError(s.t, expect.Locator(s.treeItem("Garage")).ToHaveAttribute("aria-expanded", want))
			s.settle()
			require.NoError(s.t, expect.Locator(s.treeItem("Garage")).ToHaveAttribute("aria-selected", "false"))
			require.NoError(s.t, expect.Locator(s.treeItem("Shed")).ToHaveAttribute("aria-selected", "true"))
			if s.layout == narrow {
				require.NoError(s.t, expect.Locator(s.treeToggle()).ToHaveAttribute("aria-expanded", "true"))
			}
		}

		s.closeTree()
		require.NoError(s.t, expect.Locator(s.detailHeading("Shed")).ToBeVisible())
	})
}

// Opening a container from the Contents list expands only its ancestors, so
// the container itself stays collapsed.
func TestBrowserTreeChildrenListSelectsWithoutExpanding(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		garage := s.createEntity("Garage", "")
		s.createEntity("Toolbox", garage.EntityID)

		s.visit("/")
		s.click(s.childLink("Garage"))
		require.NoError(s.t, expect.Locator(s.detailHeading("Garage")).ToBeVisible())

		s.settle()
		s.openTree()
		require.NoError(s.t, expect.Locator(s.treeItem("Garage")).ToHaveAttribute("aria-selected", "true"))
		require.NoError(s.t, expect.Locator(s.treeItem("Garage")).ToHaveAttribute("aria-expanded", "false"))
	})
}
