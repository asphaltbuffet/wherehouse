//go:build browser

package web_test

import (
	"testing"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrowserRename(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		bench := s.createEntity("Bench", "")

		s.visit("/entities/" + bench.EntityID)
		s.click(s.button("Edit name"))
		name := s.page.GetByLabel("New name")
		s.fill(name, "Workbench")
		s.click(s.button("Save"))

		require.NoError(s.t, expect.Locator(s.detailHeading("Workbench")).ToBeVisible())
		s.openTree()
		require.NoError(s.t, expect.Locator(s.treeItem("Workbench")).ToBeVisible())
		require.NoError(s.t, expect.Locator(s.treeItem("Bench")).ToHaveCount(0))
		require.NoError(s.t, expect.Locator(s.treeItem("Workbench").GetByRole(*playwright.AriaRoleButton,
			playwright.LocatorGetByRoleOptions{Name: "Add child item under Workbench"})).ToBeAttached())

		got, err := s.app.GetEntityByID(s.t.Context(), bench.EntityID)
		require.NoError(s.t, err)
		assert.Equal(s.t, "Workbench", got.DisplayName)
	})
}

func TestBrowserRenameSortsTreeNode(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		attic := s.createEntity("Attic", "")
		s.createEntity("Basement", "")
		s.createEntity("Garage", "")

		s.visit("/entities/" + attic.EntityID)
		s.click(s.button("Edit name"))
		s.fill(s.page.GetByLabel("New name"), "Cellar")
		s.click(s.button("Save"))

		require.NoError(s.t, expect.Locator(s.detailHeading("Cellar")).ToBeVisible())
		s.openTree()
		require.NoError(s.t, expect.Locator(s.treeLabels("#tree-root-list")).
			ToHaveText([]string{"Basement", "Cellar", "Garage"}))
	})
}
