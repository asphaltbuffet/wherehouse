//go:build browser

package web_test

import (
	"testing"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"
)

func TestBrowserAddUnderParent(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		garage := s.createEntity("Garage", "")

		s.visit("/entities/" + garage.EntityID)
		require.NoError(s.t, s.button("Add child").Click())
		s.submitAddForm("Toolbox")

		require.NoError(s.t, expect.Locator(s.detailHeading("Garage")).ToBeVisible())
		require.NoError(s.t, expect.Locator(s.childLink("Toolbox")).ToBeVisible())
		s.requireChild(garage.EntityID, "Toolbox")
	})
}

func TestBrowserAddAtRoot(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		s.visit("/")
		require.NoError(s.t, s.button("Add root").Click())
		s.submitAddForm("Garage")

		require.NoError(s.t, expect.Locator(s.childLink("Garage")).ToBeVisible())
		s.openTree()
		require.NoError(s.t, expect.Locator(s.treeItem("Garage")).ToBeVisible())
		s.requireChild("", "Garage")
	})
}

// submitAddForm fills the open add-item dialog with name, submits it, and
// waits for the dialog to close.
func (s *session) submitAddForm(name string) {
	s.t.Helper()
	dialog := s.page.GetByRole(*playwright.AriaRoleDialog, playwright.PageGetByRoleOptions{Name: "Add item"})
	require.NoError(s.t, expect.Locator(dialog).ToBeVisible())
	require.NoError(s.t, dialog.GetByLabel("Display Name").Fill(name))
	require.NoError(s.t, dialog.GetByRole(*playwright.AriaRoleButton, playwright.LocatorGetByRoleOptions{
		Name:  "Add",
		Exact: new(true),
	}).Click())
	require.NoError(s.t, expect.Locator(dialog).ToBeHidden())
}
