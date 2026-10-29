//go:build browser

package web_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Narrow layouts show the tree as a drawer; wide layouts always show it
// (ADR 0037). iPad Mini's 768px viewport proves the breakpoint is Narrow.

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
