//go:build browser

package web_test

import (
	"testing"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
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
		s.pickParent(shed)
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

// Two locations share a path; the picker row's ID decides which one is stored (#294).
func TestBrowserMovePicksSameNamedLocationByID(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		garage := s.createEntity("Garage", "")
		drill := s.createEntity("Drill", garage.EntityID)
		s.createEntity("Shed", "")
		shed := s.createEntity("Shed", "")

		s.visit("/entities/" + drill.EntityID)
		s.click(s.button("Move"))
		s.fill(s.page.GetByLabel("Move to"), "Shed")
		s.pickParent(shed)
		s.click(s.button("Move"))

		path := s.page.GetByRole(*playwright.AriaRoleNavigation, playwright.PageGetByRoleOptions{Name: "Location path"})
		require.NoError(s.t, expect.Locator(path).ToHaveText("Inventory › Shed › Drill"))
		assert.Equal(s.t, shed.EntityID, s.stored(drill.EntityID).ParentID)
	})
}

// A focused node moved into a shown list keeps focus; moved into a collapsed
// one, focus goes to a neighbour in its old list (#304). Wide only: the
// narrow drawer is closed while the detail pane is in use.
func TestBrowserMoveFocusedTreeNode(t *testing.T) {
	t.Parallel()
	forDevices(t, devicesIn(wide), sessionConfig{}, func(s *session) {
		garage := s.createEntity("Garage", "")
		drill := s.createEntity("Drill", garage.EntityID)
		hammer := s.createEntity("Hammer", garage.EntityID)
		shed := s.createEntity("Shed", "")
		s.createEntity("Rake", shed.EntityID)

		// moveFocused opens e's move form, picks to, focuses e's tree row,
		// then submits the form without moving focus off the row.
		moveFocused := func(e, to app.EntityResult) {
			s.t.Helper()
			s.click(s.button("Move"))
			s.fill(s.page.GetByLabel("Move to"), to.DisplayName)
			s.pickParent(to)
			require.NoError(s.t, s.treeRow(e.DisplayName).Focus())
			_, err := s.page.Evaluate(`document.querySelector('form[data-action="move"]').requestSubmit()`)
			require.NoError(s.t, err)
			path := s.page.GetByRole(*playwright.AriaRoleNavigation,
				playwright.PageGetByRoleOptions{Name: "Location path"})
			require.NoError(s.t, expect.Locator(path).ToHaveText("Inventory › "+to.DisplayName+" › "+e.DisplayName))
			s.settle()
		}

		s.visit("/entities/" + drill.EntityID)
		s.expandTreeItem("Shed")
		moveFocused(drill, shed)
		require.NoError(s.t, expect.Locator(s.treeItem("Shed").GetByRole(*playwright.AriaRoleTreeitem,
			playwright.LocatorGetByRoleOptions{Name: "Drill", Exact: new(true)})).ToBeVisible())
		require.NoError(s.t, expect.Locator(s.treeRow("Drill")).ToBeFocused())

		s.visit("/entities/" + hammer.EntityID)
		s.expandTreeItem("Shed")
		s.click(s.treeRow("Shed").Locator(".chevron")) // collapse: loaded but hidden
		require.NoError(s.t, expect.Locator(s.treeItem("Shed")).ToHaveAttribute("aria-expanded", "false"))
		moveFocused(hammer, shed)
		require.NoError(s.t, expect.Locator(s.treeRow("Garage")).ToBeFocused())
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
		s.pickParent(shed)
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
