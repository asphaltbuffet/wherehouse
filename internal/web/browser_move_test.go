//go:build browser

package web_test

import (
	"testing"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrowserMove(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		garage := s.createEntity("Garage", "")
		drill := s.createEntity("Drill", garage.EntityID)
		shed := s.createEntity("Shed", "")

		s.visit("/entities/" + drill.EntityID)
		s.click(s.button("Move"))
		s.fill(s.page.GetByLabel("Move to"), "Shed")
		options := s.page.GetByRole(*playwright.AriaRoleList, playwright.PageGetByRoleOptions{
			Name: "Matching locations",
		})
		s.click(options.GetByRole(*playwright.AriaRoleButton, playwright.LocatorGetByRoleOptions{
			Name:  "Shed",
			Exact: new(true),
		}))
		s.click(s.button("Move"))

		path := s.page.GetByRole(*playwright.AriaRoleNavigation, playwright.PageGetByRoleOptions{Name: "Location path"})
		require.NoError(s.t, expect.Locator(path).ToHaveText("Inventory › Shed › Drill"))
		s.openTree()
		s.expandTreeItem("Shed")
		require.NoError(s.t, expect.Locator(s.treeItem("Shed").GetByRole(*playwright.AriaRoleTreeitem,
			playwright.LocatorGetByRoleOptions{Name: "Drill", Exact: new(true)})).ToBeVisible())

		assert.Equal(s.t, shed.EntityID, s.stored(drill.EntityID).ParentID)
	})
}

// A moved node keeps its tree state: still selected, still expanded (#315).
func TestBrowserMoveKeepsTreeState(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		garage := s.createEntity("Garage", "")
		toolbox := s.createEntity("Toolbox", garage.EntityID)
		s.createEntity("Wrench", toolbox.EntityID)
		shed := s.createEntity("Shed", "")
		s.createEntity("Rake", shed.EntityID)

		s.visit("/entities/" + toolbox.EntityID)
		s.openTree()
		s.expandTreeItem("Toolbox")
		s.expandTreeItem("Shed")
		s.closeTree()

		s.click(s.button("Move"))
		s.fill(s.page.GetByLabel("Move to"), "Shed")
		options := s.page.GetByRole(*playwright.AriaRoleList, playwright.PageGetByRoleOptions{
			Name: "Matching locations",
		})
		s.click(options.GetByRole(*playwright.AriaRoleButton, playwright.LocatorGetByRoleOptions{
			Name:  "Shed",
			Exact: new(true),
		}))
		s.click(s.button("Move"))

		path := s.page.GetByRole(*playwright.AriaRoleNavigation, playwright.PageGetByRoleOptions{Name: "Location path"})
		require.NoError(s.t, expect.Locator(path).ToHaveText("Inventory › Shed › Toolbox"))
		s.settle()
		s.openTree()
		moved := s.treeItem("Shed").GetByRole(*playwright.AriaRoleTreeitem, playwright.LocatorGetByRoleOptions{
			Name: "Toolbox", Exact: new(true),
		})
		require.NoError(s.t, expect.Locator(moved).ToHaveAttribute("aria-selected", "true"))
		require.NoError(s.t, expect.Locator(moved).ToHaveAttribute("aria-expanded", "true"))
		require.NoError(s.t, expect.Locator(moved.GetByRole(*playwright.AriaRoleTreeitem,
			playwright.LocatorGetByRoleOptions{Name: "Wrench", Exact: new(true)})).ToBeVisible())
	})
}
