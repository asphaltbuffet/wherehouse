//go:build browser

package web_test

import (
	"testing"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"
)

// Computed forms of --color-surface (#24292a) and --color-text (#e4e7e1) in
// app.css's `prefers-color-scheme: dark` block.
const (
	darkSurface = "rgb(36, 41, 42)"
	darkText    = "rgb(228, 231, 225)"
)

var darkScheme = sessionConfig{colorScheme: playwright.ColorSchemeDark}

func TestBrowserDarkModeWide(t *testing.T) {
	t.Parallel()
	forDevices(t, devicesIn(wide), darkScheme, func(s *session) {
		s.visit("/")
		s.requireDarkPage()
	})
}

func TestBrowserDarkModeNarrow(t *testing.T) {
	t.Parallel()
	forDevices(t, devicesIn(narrow), darkScheme, func(s *session) {
		s.visit("/")
		s.requireDarkPage()
		// The drawer paints its own background over the page.
		s.openTree()
		require.NoError(s.t, expect.Locator(s.treePanel()).ToHaveCSS("background-color", darkSurface))
	})
}

func (s *session) requireDarkPage() {
	s.t.Helper()
	body := s.page.Locator("body")
	require.NoError(s.t, expect.Locator(body).ToHaveCSS("background-color", darkSurface))
	require.NoError(s.t, expect.Locator(body).ToHaveCSS("color", darkText))
}
