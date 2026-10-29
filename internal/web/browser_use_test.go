//go:build browser

package web_test

import (
	"testing"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

func TestBrowserUseAndReturn(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		garage := s.createEntity("Garage", "")
		drill := s.createEntity("Drill", garage.EntityID)
		deck := s.createEntity("Deck", "")

		s.visit("/entities/" + drill.EntityID)
		s.click(s.button("Use"))
		s.fill(s.page.GetByLabel("Use at"), "Deck")
		options := s.page.GetByRole(*playwright.AriaRoleList, playwright.PageGetByRoleOptions{
			Name: "Matching locations",
		})
		s.click(options.GetByRole(*playwright.AriaRoleButton, playwright.LocatorGetByRoleOptions{
			Name:  "Deck",
			Exact: new(true),
		}))
		s.click(s.button("Use"))

		path := s.page.GetByRole(*playwright.AriaRoleNavigation, playwright.PageGetByRoleOptions{Name: "Location path"})
		require.NoError(s.t, expect.Locator(path).ToHaveText("Inventory › Deck › Drill"))
		require.NoError(s.t, expect.Locator(s.page.GetByText("away", playwright.PageGetByTextOptions{
			Exact: new(true),
		})).ToBeVisible())
		assert.Equal(s.t, deck.EntityID, s.stored(drill.EntityID).ParentID)
		assert.Equal(s.t, garage.EntityID, s.stored(drill.EntityID).HomeID)

		s.click(s.button("Return"))
		require.NoError(s.t, expect.Locator(path).ToHaveText("Inventory › Garage › Drill"))
		s.openTree()
		s.expandTreeItem("Garage")
		require.NoError(s.t, expect.Locator(s.treeItem("Garage").GetByRole(*playwright.AriaRoleTreeitem,
			playwright.LocatorGetByRoleOptions{Name: "Drill", Exact: new(true)})).ToBeVisible())
		assert.Equal(s.t, garage.EntityID, s.stored(drill.EntityID).ParentID)
	})
}

func TestBrowserRehomeViaMove(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		garage := s.createEntity("Garage", "")
		drill := s.createEntity("Drill", garage.EntityID)
		deck := s.createEntity("Deck", "")
		_, err := s.app.UseEntity(s.t.Context(), app.UseEntityRequest{
			EntityID: drill.EntityID, NewParentID: deck.EntityID, ActorID: "test",
		})
		require.NoError(s.t, err)

		s.visit("/entities/" + drill.EntityID)
		s.click(s.button("Move"))
		s.fill(s.page.GetByLabel("Move to"), "Deck")
		options := s.page.GetByRole(*playwright.AriaRoleList, playwright.PageGetByRoleOptions{
			Name: "Matching locations",
		})
		s.click(options.GetByRole(*playwright.AriaRoleButton, playwright.LocatorGetByRoleOptions{
			Name:  "Deck",
			Exact: new(true),
		}))
		s.click(s.button("Move"))

		require.NoError(s.t, expect.Locator(s.button("Return")).ToBeHidden())
		got := s.stored(drill.EntityID)
		assert.Equal(s.t, deck.EntityID, got.ParentID)
		assert.Equal(s.t, deck.EntityID, got.HomeID)
		assert.False(s.t, got.IsDisplaced())
	})
}
