//go:build browser

package web_test

import (
	"testing"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func TestBrowserBorrow(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		s.visit("/")
		s.click(s.button("Borrow"))
		dialog := s.addDialog()
		require.NoError(s.t, expect.Locator(dialog).ToBeVisible())
		name := dialog.GetByLabel("Name", playwright.LocatorGetByLabelOptions{Exact: new(true)})
		s.fill(name, "Ladder")
		s.fill(dialog.GetByLabel("Borrowed from"), "Bob")
		s.click(dialog.GetByRole(*playwright.AriaRoleButton, playwright.LocatorGetByRoleOptions{
			Name:  "Borrow",
			Exact: new(true),
		}))

		require.NoError(s.t, expect.Locator(dialog).ToBeHidden())
		require.NoError(s.t, expect.Locator(s.detailHeading("Ladder")).ToBeVisible())
		require.NoError(s.t, expect.Locator(s.mainText("Borrowed from Bob")).ToBeVisible())

		ladder := s.requireChild("", "Ladder")
		assert.Equal(s.t, inventory.EntityStatusBorrowed, ladder.Status)
		assert.Equal(s.t, "Bob", ladder.StatusContext)
	})
}

func TestBrowserLoan(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		ladder := s.createEntity("Ladder", "")

		s.visit("/entities/" + ladder.EntityID)
		s.click(s.button("Loan"))
		s.fill(s.page.GetByLabel("Loaned to"), "Carol")
		s.click(s.button("Loan"))

		require.NoError(s.t, expect.Locator(s.mainText("Loaned to Carol")).ToBeVisible())
		got := s.stored(ladder.EntityID)
		assert.Equal(s.t, inventory.EntityStatusLoaned, got.Status)
		assert.Equal(s.t, "Carol", got.StatusContext)
	})
}

func TestBrowserReturnBorrowed(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		garage := s.createEntity("Garage", "")
		ladder := s.borrowEntity("Ladder", garage.EntityID, "Bob")
		s.acceptDialogs()

		s.visit("/entities/" + ladder.EntityID)
		s.click(s.button("Return"))

		// Returning a borrowed entity removes it and shows its parent.
		require.NoError(s.t, expect.Locator(s.detailHeading("Garage")).ToBeVisible())
		require.NoError(s.t, expect.Locator(s.childLink("Ladder")).ToHaveCount(0))
		assert.Equal(s.t, inventory.EntityStatusRemoved, s.stored(ladder.EntityID).Status)
	})
}

func TestBrowserReturnRefusedWithChildren(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		crate := s.borrowEntity("Crate", "", "Bob")
		s.createEntity("Bolt", crate.EntityID)
		s.acceptDialogs()

		s.visit("/entities/" + crate.EntityID)
		s.click(s.button("Return"))

		alert := s.page.GetByRole(*playwright.AriaRoleMain).GetByRole(*playwright.AriaRoleAlert)
		require.NoError(s.t, expect.Locator(alert).ToContainText("still contains other entities"))
		require.NoError(s.t, expect.Locator(s.childLink("Bolt")).ToBeVisible())
		assert.Equal(s.t, inventory.EntityStatusBorrowed, s.stored(crate.EntityID).Status)
	})
}

// borrowEntity seeds a borrowed entity through the app layer.
func (s *session) borrowEntity(name, parentID, from string) app.EntityResult {
	s.t.Helper()
	results, err := s.app.BorrowEntities(s.t.Context(), []app.BorrowEntityRequest{{
		DisplayName:   name,
		ParentID:      parentID,
		StatusContext: from,
		ActorID:       "browser-test",
	}})
	require.NoError(s.t, err)
	require.Len(s.t, results, 1)
	return results[0]
}

// acceptDialogs confirms every native dialog, such as hx-confirm's; by
// default Playwright dismisses them.
func (s *session) acceptDialogs() {
	s.page.OnDialog(func(d playwright.Dialog) {
		_ = d.Accept()
	})
}
