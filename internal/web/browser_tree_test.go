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
		require.NoError(s.t, s.treeRow("Drawer").Click())

		require.NoError(s.t, expect.Locator(s.detailHeading("Drawer")).ToBeVisible())
		require.NoError(s.t, expect.Locator(s.treeItem("Drawer")).ToHaveAttribute("aria-selected", "true"))
		if s.layout == narrow {
			// Loading a detail from the tree closes the drawer to show it.
			require.NoError(s.t, expect.Locator(s.treeToggle()).ToHaveAttribute("aria-expanded", "false"))
		}
	})
}
