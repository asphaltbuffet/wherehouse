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
	})
}
