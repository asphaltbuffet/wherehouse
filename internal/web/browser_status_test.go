//go:build browser

package web_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func TestBrowserLostAndFound(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		drill := s.createEntity("Drill", "")

		s.visit("/entities/" + drill.EntityID)
		require.NoError(s.t, s.button("Mark missing").Click())
		require.NoError(s.t, expect.Locator(s.mainText("Missing")).ToBeVisible())
		require.NoError(s.t, expect.Locator(s.button("Mark found")).ToBeVisible())
		assert.Equal(s.t, inventory.EntityStatusMissing, s.stored(drill.EntityID).Status)

		require.NoError(s.t, s.button("Mark found").Click())
		require.NoError(s.t, expect.Locator(s.button("Mark missing")).ToBeVisible())
		require.NoError(s.t, expect.Locator(s.mainText("Missing")).ToHaveCount(0))
		assert.Equal(s.t, inventory.EntityStatusOk, s.stored(drill.EntityID).Status)
	})
}

func TestBrowserAttributes(t *testing.T) {
	t.Parallel()
	forEachDevice(t, func(s *session) {
		drill := s.createEntity("Drill", "")

		s.visit("/entities/" + drill.EntityID)
		require.NoError(s.t, s.button("Lock").Click())
		require.NoError(s.t, expect.Locator(s.button("Unlock")).ToBeVisible())
		require.NoError(s.t, expect.Locator(s.mainText("Locked")).ToBeVisible())
		assert.True(s.t, s.stored(drill.EntityID).Locked, "locked")

		require.NoError(s.t, s.button("Mark discrete").Click())
		require.NoError(s.t, expect.Locator(s.button("Clear discrete")).ToBeVisible())
		require.NoError(s.t, expect.Locator(s.mainText("Discrete")).ToBeVisible())
		assert.True(s.t, s.stored(drill.EntityID).Discrete, "discrete")

		require.NoError(s.t, s.button("Unlock").Click())
		require.NoError(s.t, expect.Locator(s.button("Lock")).ToBeVisible())
		require.NoError(s.t, expect.Locator(s.mainText("Locked")).ToHaveCount(0))
		assert.False(s.t, s.stored(drill.EntityID).Locked, "locked")
	})
}
