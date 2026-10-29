//go:build browser

package web_test

import (
	"testing"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

// The actor header is server-side behaviour, so one device suffices.
func TestBrowserActorHeader(t *testing.T) {
	t.Parallel()
	s := newSession(t, devices[0], sessionConfig{actorHeader: "X-Forwarded-User", actor: "alice"})

	s.visit("/")
	footer := s.page.GetByRole(*playwright.AriaRoleContentinfo)
	require.NoError(s.t, expect.Locator(footer.GetByText("acting as alice", playwright.LocatorGetByTextOptions{
		Exact: new(true),
	})).ToBeVisible())

	require.NoError(s.t, s.button("Add root").Click())
	s.submitAddForm("Garage")
	require.NoError(s.t, expect.Locator(s.childLink("Garage")).ToBeVisible())

	garage := s.requireChild("", "Garage")
	history, err := s.app.GetHistory(s.t.Context(), app.GetHistoryRequest{EntityID: garage.EntityID})
	require.NoError(s.t, err)
	require.NotEmpty(s.t, history)
	for _, ev := range history {
		assert.Equal(s.t, "alice", ev.ActorUserID, "event %d (%s)", ev.EventID, ev.EventType)
	}
}
