//go:build browser

package web_test

import (
	"testing"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"
)

// Esc in any inline detail form closes it, as Cancel does.
func TestBrowserEscCancelsInlineForms(t *testing.T) {
	t.Parallel()
	forms := []struct {
		open, field string
	}{
		{open: "Edit name", field: "New name"},
		{open: "Loan", field: "Loaned to"},
		{open: "Mark missing", field: "Note (optional)"},
		{open: "Move", field: "Move to"},
		{open: "Use", field: "Use at"},
		{open: "Remove", field: "Note (optional)"},
		{open: "+ tag", field: "New tag"},
	}
	forEachDevice(t, func(s *session) {
		bench := s.createEntity("Bench", "")
		s.createEntity("Shelf", "")

		s.visit("/entities/" + bench.EntityID)
		for _, f := range forms {
			s.click(s.button(f.open))
			field := s.page.GetByLabel(f.field, playwright.PageGetByLabelOptions{Exact: new(true)})
			require.NoError(s.t, expect.Locator(field).ToBeVisible(), f.open)
			s.press(field, "Escape")
			require.NoError(s.t, expect.Locator(field).ToHaveCount(0), f.open)
		}

		// Return is offered only away from ok-at-home, so it needs its own entity.
		ladder := s.borrowEntity("Ladder", "", "Bob")
		s.visit("/entities/" + ladder.EntityID)
		s.click(s.button("Return"))
		field := s.page.GetByLabel("Note (optional)", playwright.PageGetByLabelOptions{Exact: new(true)})
		require.NoError(s.t, expect.Locator(field).ToBeVisible(), "Return")
		s.press(field, "Escape")
		require.NoError(s.t, expect.Locator(field).ToHaveCount(0), "Return")
		require.NoError(s.t, expect.Locator(s.button("Return")).ToBeVisible())
	})
}

// With the add modal open over the drawer, Esc closes only the modal; the
// drawer stays open behind it.
func TestBrowserEscClosesModalBeforeDrawer(t *testing.T) {
	t.Parallel()
	forDevices(t, devicesIn(narrow), sessionConfig{}, func(s *session) {
		s.createEntity("Garage", "")

		s.visit("/")
		s.openTree()
		s.click(s.treePanel().GetByRole(*playwright.AriaRoleButton, playwright.LocatorGetByRoleOptions{
			Name: "Add child item under Garage",
		}))
		dialog := s.addDialog()
		require.NoError(s.t, expect.Locator(dialog).ToBeVisible())

		s.press(dialog.GetByLabel("Display Name"), "Escape")
		require.NoError(s.t, expect.Locator(dialog).ToBeHidden())
		require.NoError(s.t, expect.Locator(s.treePanel()).ToBeVisible())
		require.NoError(s.t, expect.Locator(s.treeToggle()).ToHaveAttribute("aria-expanded", "true"))

		// The next Esc reaches the drawer.
		s.closeTree()
		require.NoError(s.t, expect.Locator(s.treePanel()).ToBeHidden())
	})
}
