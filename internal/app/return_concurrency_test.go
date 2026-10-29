package app_test

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// concurrentReturners is how many goroutines race to return the same entity.
const concurrentReturners = 8

// raceMarkReturned calls MarkReturned for entityID from concurrentReturners
// goroutines released at once and returns every call's error.
func raceMarkReturned(t *testing.T, a *app.App, entityID string) []error {
	t.Helper()
	start := make(chan struct{})
	errs := make([]error, concurrentReturners)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Go(func() {
			<-start
			_, errs[i] = a.MarkReturned(t.Context(), []app.ChangeStatusRequest{{EntityID: entityID, ActorID: "test"}})
		})
	}
	close(start)
	wg.Wait()
	return errs
}

func historyLen(t *testing.T, a *app.App, entityID string) int {
	t.Helper()
	h, err := a.GetHistory(t.Context(), app.GetHistoryRequest{EntityID: entityID})
	require.NoError(t, err)
	return len(h)
}

// assertOneStatusEvent checks exactly one event was appended since before and
// that it is a status change to want.
func assertOneStatusEvent(t *testing.T, a *app.App, entityID string, before int, want inventory.EntityStatus) {
	t.Helper()
	h, err := a.GetHistory(t.Context(), app.GetHistoryRequest{EntityID: entityID})
	require.NoError(t, err)
	require.Len(t, h, before+1, "exactly one return event")

	newest := h[0]
	assert.Equal(t, inventory.EntityStatusChangedEvent, newest.EventType)
	var p eventbus.EntityStatusChangedPayload
	require.NoError(t, json.Unmarshal(newest.Payload, &p))
	assert.Equal(t, want.String(), p.Status)
}

// assertOneWinner checks exactly one call succeeded and no loser failed on
// lock contention (SQLITE_BUSY) rather than on the already-returned state.
func assertOneWinner(t *testing.T, errs []error) []error {
	t.Helper()
	var losers []error
	for _, err := range errs {
		if err != nil {
			assert.NotContains(t, err.Error(), "SQLITE_BUSY")
			losers = append(losers, err)
		}
	}
	require.Len(t, losers, concurrentReturners-1, "exactly one MarkReturned must succeed")
	return losers
}

// #299: concurrent returns of a borrowed entity must serialize, so losers see
// it already removed (ErrNotFound) instead of failing on SQLITE_BUSY.
func TestMarkReturned_ConcurrentBorrowedOneWinner(t *testing.T) {
	a := apptesting.OpenApp(t)
	b, err := a.BorrowEntities(t.Context(), []app.BorrowEntityRequest{
		{DisplayName: "Drill", StatusContext: "Alice", ActorID: "test"},
	})
	require.NoError(t, err)
	id := b[0].EntityID
	before := historyLen(t, a, id)

	for _, err := range assertOneWinner(t, raceMarkReturned(t, a, id)) {
		require.ErrorIs(t, err, app.ErrNotFound)
	}
	assertOneStatusEvent(t, a, id, before, inventory.EntityStatusRemoved)
}

// #299: concurrent returns of a loaned entity must not all pass the status
// guard and append duplicate status-changed events.
func TestMarkReturned_ConcurrentLoanedOneWinner(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	e, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Hammer", ActorID: "test"})
	require.NoError(t, err)
	_, err = a.MarkLoaned(ctx, []app.ChangeStatusRequest{
		{EntityID: e.EntityID, StatusContext: "Bob", ActorID: "test"},
	})
	require.NoError(t, err)
	before := historyLen(t, a, e.EntityID)

	for _, err := range assertOneWinner(t, raceMarkReturned(t, a, e.EntityID)) {
		require.ErrorContains(t, err, "already at home")
	}
	assertOneStatusEvent(t, a, e.EntityID, before, inventory.EntityStatusOk)
}
