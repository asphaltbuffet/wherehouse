//go:build browser

package web_test

import (
	"testing"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"
)

// Narrow layouts show the tree as a drawer; wide layouts always show it
// (ADR 0037). iPad Mini's 768px viewport proves the breakpoint is Narrow.

// backdropEdgeInset is how far inside the backdrop's right edge a click lands.
const backdropEdgeInset = 8

func TestBrowserDrawerNarrow(t *testing.T) {
	t.Parallel()
	forDevices(t, devicesIn(narrow), sessionConfig{}, func(s *session) {
		s.createEntity("Garage", "")

		s.visit("/")
		require.NoError(s.t, expect.Locator(s.treePanel()).ToBeHidden())
		require.NoError(s.t, expect.Locator(s.treeToggle()).ToHaveAttribute("aria-expanded", "false"))

		s.click(s.treeToggle())
		require.NoError(s.t, expect.Locator(s.treePanel()).ToBeVisible())
		require.NoError(s.t, expect.Locator(s.treeToggle()).ToHaveAttribute("aria-expanded", "true"))

		s.click(s.treeRow("Garage"))
		require.NoError(s.t, expect.Locator(s.detailHeading("Garage")).ToBeVisible())
		require.NoError(s.t, expect.Locator(s.treePanel()).ToBeHidden())
		require.NoError(s.t, expect.Locator(s.treeToggle()).ToHaveAttribute("aria-expanded", "false"))
	})
}

// Tapping the backdrop beside the open drawer closes it.
func TestBrowserDrawerBackdropCloses(t *testing.T) {
	t.Parallel()
	forDevices(t, devicesIn(narrow), sessionConfig{}, func(s *session) {
		s.createEntity("Garage", "")

		s.visit("/")
		s.openTree()
		require.NoError(s.t, expect.Locator(s.treePanel()).ToBeVisible())

		// The backdrop spans the viewport under the drawer, so its center may be
		// covered; click its right edge, which the drawer never reaches.
		backdrop := s.page.Locator("#tree-backdrop")
		box, err := backdrop.BoundingBox()
		require.NoError(s.t, err)
		s.settle()
		require.NoError(s.t, backdrop.Click(playwright.LocatorClickOptions{
			Position: &playwright.Position{X: box.Width - backdropEdgeInset, Y: box.Height / 2},
		}))

		require.NoError(s.t, expect.Locator(s.treePanel()).ToBeHidden())
		require.NoError(s.t, expect.Locator(s.treeToggle()).ToHaveAttribute("aria-expanded", "false"))
	})
}

func TestBrowserDrawerWide(t *testing.T) {
	t.Parallel()
	forDevices(t, devicesIn(wide), sessionConfig{}, func(s *session) {
		s.createEntity("Garage", "")

		s.visit("/")
		require.NoError(s.t, expect.Locator(s.treeToggle()).ToBeHidden())
		require.NoError(s.t, expect.Locator(s.treePanel()).ToBeVisible())
		require.NoError(s.t, expect.Locator(s.treeItem("Garage")).ToBeVisible())
	})
}
