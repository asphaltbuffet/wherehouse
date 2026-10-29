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
	forEachDevice(t, func(s *session) {
		s.requireLayout(narrow)
		s.createEntity("Garage", "")

		s.visit("/")
		require.NoError(s.t, expect.Locator(s.treePanel()).ToBeHidden())
		require.NoError(s.t, expect.Locator(s.treeToggle()).ToHaveAttribute("aria-expanded", "false"))

		require.NoError(s.t, s.treeToggle().Click())
		require.NoError(s.t, expect.Locator(s.treePanel()).ToBeVisible())
		require.NoError(s.t, expect.Locator(s.treeToggle()).ToHaveAttribute("aria-expanded", "true"))

		require.NoError(s.t, s.treeRow("Garage").Click())
		require.NoError(s.t, expect.Locator(s.detailHeading("Garage")).ToBeVisible())
		require.NoError(s.t, expect.Locator(s.treePanel()).ToBeHidden())
		require.NoError(s.t, expect.Locator(s.treeToggle()).ToHaveAttribute("aria-expanded", "false"))
	})
}

func TestBrowserDrawerWide(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		s.requireLayout(wide)
		s.createEntity("Garage", "")

		s.visit("/")
		require.NoError(s.t, expect.Locator(s.treeToggle()).ToBeHidden())
		require.NoError(s.t, expect.Locator(s.treePanel()).ToBeVisible())
		require.NoError(s.t, expect.Locator(s.treeItem("Garage")).ToBeVisible())
	})
}
