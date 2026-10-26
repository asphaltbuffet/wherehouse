package app_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
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
	assert.Equal(t, before+1, historyLen(t, a, id), "exactly one return event")
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

	assertOneWinner(t, raceMarkReturned(t, a, e.EntityID))
	assert.Equal(t, before+1, historyLen(t, a, e.EntityID), "exactly one return event")
}
