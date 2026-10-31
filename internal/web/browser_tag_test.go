//go:build browser

package web_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

func TestBrowserTags(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		bench := s.createEntity("Bench", "")

		s.visit("/entities/" + bench.EntityID)
		s.click(s.button("+ tag"))
		input := s.page.GetByLabel("New tag")
		s.fill(input, "Power Tools")
		s.press(input, "Enter")
		require.NoError(s.t, expect.Locator(s.button("Remove tag power_tools")).ToBeVisible())

		// The input stays open and focused for the next tag.
		require.NoError(s.t, expect.Locator(input).ToBeFocused())
		s.fill(input, "garden")
		s.press(input, "Enter")
		require.NoError(s.t, expect.Locator(s.button("Remove tag garden")).ToBeVisible())

		// Esc closes the re-rendered input.
		s.press(input, "Escape")
		require.NoError(s.t, expect.Locator(input).ToHaveCount(0))
		require.NoError(s.t, expect.Locator(s.button("+ tag")).ToBeVisible())

		s.click(s.button("Remove tag garden"))
		require.NoError(s.t, expect.Locator(s.button("Remove tag garden")).ToHaveCount(0))

		got, err := s.app.ListTags(s.t.Context(), app.ListTagsRequest{EntityID: bench.EntityID})
		require.NoError(s.t, err)
		assert.Equal(s.t, []string{"power_tools"}, got)
	})
}

// A tag with quotes and angle brackets survives the HTML round trip: the ×
// posts the decoded hidden value, which canonicalizes back to the stored tag.
func TestBrowserTagMetacharacters(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		bench := s.createEntity("Bench", "")
		const odd = `a"b<c>`

		s.visit("/entities/" + bench.EntityID)
		s.click(s.button("+ tag"))
		input := s.page.GetByLabel("New tag")
		s.fill(input, odd)
		s.press(input, "Enter")
		require.NoError(s.t, expect.Locator(s.button("Remove tag "+odd)).ToBeVisible())

		s.click(s.button("Remove tag " + odd))
		require.NoError(s.t, expect.Locator(s.button("Remove tag "+odd)).ToHaveCount(0))

		got, err := s.app.ListTags(s.t.Context(), app.ListTagsRequest{EntityID: bench.EntityID})
		require.NoError(s.t, err)
		assert.Empty(s.t, got)
	})
}
