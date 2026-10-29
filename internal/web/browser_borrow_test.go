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
		require.NoError(s.t, s.button("Borrow").Click())
		dialog := s.addDialog()
		require.NoError(s.t, expect.Locator(dialog).ToBeVisible())
		name := dialog.GetByLabel("Name", playwright.LocatorGetByLabelOptions{Exact: new(true)})
		require.NoError(s.t, name.Fill("Ladder"))
		require.NoError(s.t, dialog.GetByLabel("Borrowed from").Fill("Bob"))
		require.NoError(s.t, dialog.GetByRole(*playwright.AriaRoleButton, playwright.LocatorGetByRoleOptions{
			Name:  "Borrow",
			Exact: new(true),
		}).Click())

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
		require.NoError(s.t, s.button("Loan").Click())
		require.NoError(s.t, s.page.GetByLabel("Loaned to").Fill("Carol"))
		require.NoError(s.t, s.button("Loan").Click())

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
		require.NoError(s.t, s.button("Return").Click())

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
		require.NoError(s.t, s.button("Return").Click())

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
