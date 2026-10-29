//go:build browser

package web_test

import (
	"testing"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"
)

func TestBrowserSearch(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		garage := s.createEntity("Garage", "")
		s.createEntity("Claw Hammer", garage.EntityID)
		s.createEntity("Screwdriver", garage.EntityID)

		s.visit("/")
		search := s.page.GetByRole(*playwright.AriaRoleSearchbox, playwright.PageGetByRoleOptions{
			Name: "Search inventory",
		})
		// Type rather than Fill and Enter: Fill fires no keyups, so Enter would
		// both submit and schedule a second, debounced search that can land
		// after the result's detail and replace it.
		s.typeText(search, "hammer")

		results := s.page.GetByRole(*playwright.AriaRoleMain).GetByRole(*playwright.AriaRoleListitem)
		match := results.Filter(playwright.LocatorFilterOptions{HasText: "Claw Hammer"})
		require.NoError(s.t, expect.Locator(match).ToBeVisible())
		require.NoError(s.t, expect.Locator(match).ToContainText("Garage:Claw Hammer"))
		require.NoError(s.t, expect.Locator(results.Filter(playwright.LocatorFilterOptions{
			HasText: "Screwdriver",
		})).ToHaveCount(0))

		s.click(match)
		require.NoError(s.t, expect.Locator(s.detailHeading("Claw Hammer")).ToBeVisible())
	})
}
