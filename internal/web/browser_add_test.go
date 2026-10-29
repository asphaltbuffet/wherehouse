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
		s.click(s.button("Add child"))
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
		s.click(s.button("Add root"))
		s.submitAddForm("Garage")

		require.NoError(s.t, expect.Locator(s.childLink("Garage")).ToBeVisible())
		s.openTree()
		require.NoError(s.t, expect.Locator(s.treeItem("Garage")).ToBeVisible())
		s.requireChild("", "Garage")
	})
}

// addDialog is the modal that hosts the add and borrow forms.
func (s *session) addDialog() playwright.Locator {
	return s.page.GetByRole(*playwright.AriaRoleDialog, playwright.PageGetByRoleOptions{Name: "Add item"})
}

// submitAddForm fills the open add-item dialog with name, submits it, and
// waits for the dialog to close.
func (s *session) submitAddForm(name string) {
	s.t.Helper()
	dialog := s.addDialog()
	require.NoError(s.t, expect.Locator(dialog).ToBeVisible())
	s.fill(dialog.GetByLabel("Display Name"), name)
	s.click(dialog.GetByRole(*playwright.AriaRoleButton, playwright.LocatorGetByRoleOptions{
		Name:  "Add",
		Exact: new(true),
	}))
	require.NoError(s.t, expect.Locator(dialog).ToBeHidden())
}
