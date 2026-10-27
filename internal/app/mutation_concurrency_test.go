package app_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// raceRounds is how many independent fixtures each race test sets up and
// releases at once. One pair rarely loses the race before the #301 fix;
// many pairs at once make a lost race near-certain.
const raceRounds = 20

// race runs fns concurrently, released together, and returns each one's error
// at the same index.
func race(fns ...func() error) []error {
	start := make(chan struct{})
	errs := make([]error, len(fns))
	var wg sync.WaitGroup
	for i, fn := range fns {
		wg.Go(func() {
			<-start
			errs[i] = fn()
		})
	}
	close(start)
	wg.Wait()
	return errs
}

// assertNoBusy checks no call failed on lock contention rather than on a
// domain rule: the busy timeout must absorb contention (#299, #300).
func assertNoBusy(t *testing.T, errs ...error) {
	t.Helper()
	for _, err := range errs {
		if err != nil {
			assert.NotContains(t, err.Error(), "SQLITE_BUSY")
		}
	}
}

func newRoot(t *testing.T, a *app.App, name string) string {
	t.Helper()
	e, err := a.CreateEntity(t.Context(), app.CreateEntityRequest{DisplayName: name, ActorID: "test"})
	require.NoError(t, err)
	return e.EntityID
}

// lastEventID returns the highest event_id of type typ in the entity's
// history, or 0 if there is none.
func lastEventID(t *testing.T, a *app.App, entityID string, typ inventory.EventType) int64 {
	t.Helper()
	h, err := a.GetHistory(t.Context(), app.GetHistoryRequest{EntityID: entityID})
	require.NoError(t, err)
	var last int64
	for _, ev := range h {
		if ev.EventType == typ && ev.EventID > last {
			last = ev.EventID
		}
	}
	return last
}

func countEvents(t *testing.T, a *app.App, entityID string, typ inventory.EventType) int {
	t.Helper()
	h, err := a.GetHistory(t.Context(), app.GetHistoryRequest{EntityID: entityID})
	require.NoError(t, err)
	n := 0
	for _, ev := range h {
		if ev.EventType == typ {
			n++
		}
	}
	return n
}

// #301: moving X into P while P is removed must not leave X live under a
// removed parent. Either order is legal; both succeeding is the bug.
func TestReparentEntity_ConcurrentRemoveOfDestination(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	parents := make([]string, raceRounds)
	children := make([]string, raceRounds)
	fns := make([]func() error, 0, 2*raceRounds)
	for i := range raceRounds {
		parents[i] = newRoot(t, a, fmt.Sprintf("Parent%d", i))
		children[i] = newRoot(t, a, fmt.Sprintf("Child%d", i))
		fns = append(fns,
			func() error {
				_, err := a.ReparentEntity(ctx, app.ReparentEntityRequest{
					EntityID: children[i], NewParentID: parents[i], ActorID: "test",
				})
				return err
			},
			func() error {
				return a.RemoveEntity(ctx, app.RemoveEntityRequest{EntityID: parents[i], ActorID: "test"})
			},
		)
	}
	errs := race(fns...)
	assertNoBusy(t, errs...)

	for i := range raceRounds {
		moveErr, removeErr := errs[2*i], errs[2*i+1]
		switch {
		case moveErr == nil:
			require.ErrorIs(t, removeErr, app.ErrHasChildren, "round %d: remove after move must see the child", i)
		case removeErr == nil:
			require.ErrorIs(t, moveErr, app.ErrNotFound, "round %d: move after remove must see no destination", i)
		default:
			t.Errorf("round %d: both failed: move=%v remove=%v", i, moveErr, removeErr)
		}
	}
}

// #301: crossing moves A→B and B→A must not both pass the cycle check.
func TestReparentEntity_ConcurrentCrossingMovesNoCycle(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	fns := make([]func() error, 0, 2*raceRounds)
	for i := range raceRounds {
		left := newRoot(t, a, fmt.Sprintf("Left%d", i))
		right := newRoot(t, a, fmt.Sprintf("Right%d", i))
		fns = append(fns,
			func() error {
				_, err := a.ReparentEntity(ctx, app.ReparentEntityRequest{
					EntityID: left, NewParentID: right, ActorID: "test",
				})
				return err
			},
			func() error {
				_, err := a.ReparentEntity(ctx, app.ReparentEntityRequest{
					EntityID: right, NewParentID: left, ActorID: "test",
				})
				return err
			},
		)
	}
	errs := race(fns...)
	assertNoBusy(t, errs...)

	for i := range raceRounds {
		first, second := errs[2*i], errs[2*i+1]
		if (first == nil) == (second == nil) {
			t.Errorf("round %d: want exactly one move to succeed, got %v and %v", i, first, second)
			continue
		}
		loser := first
		if loser == nil {
			loser = second
		}
		assert.ErrorIs(t, loser, app.ErrInvalidDestination, "round %d", i)
	}
}

// #301: a move that committed must report success even if the entity is
// removed right after; a move that lost must not have written an event.
func TestReparentEntity_ConcurrentRemoveOfSelf(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	movers := make([]string, raceRounds)
	fns := make([]func() error, 0, 2*raceRounds)
	for i := range raceRounds {
		dest := newRoot(t, a, fmt.Sprintf("Dest%d", i))
		movers[i] = newRoot(t, a, fmt.Sprintf("Mover%d", i))
		fns = append(fns,
			func() error {
				_, err := a.ReparentEntity(ctx, app.ReparentEntityRequest{
					EntityID: movers[i], NewParentID: dest, ActorID: "test",
				})
				return err
			},
			func() error {
				return a.RemoveEntity(ctx, app.RemoveEntityRequest{EntityID: movers[i], ActorID: "test"})
			},
		)
	}
	errs := race(fns...)
	assertNoBusy(t, errs...)

	for i := range raceRounds {
		moveErr, removeErr := errs[2*i], errs[2*i+1]
		require.NoError(t, removeErr, "round %d: a move never blocks removing the mover", i)

		moved := lastEventID(t, a, movers[i], inventory.EntityReparentedEvent)
		removed := lastEventID(t, a, movers[i], inventory.EntityRemovedEvent)
		if moveErr == nil {
			require.NotZero(t, moved, "round %d: successful move must have written its event", i)
			assert.Less(t, moved, removed, "round %d: move event must precede removal", i)
		} else {
			require.ErrorIs(t, moveErr, app.ErrNotFound, "round %d", i)
			assert.Zero(t, moved, "round %d: failed move must not write an event", i)
		}
	}
}

// #301: renaming X while X is removed must not append a rename after the
// removal, and a rename that committed must report success.
func TestRenameEntity_ConcurrentRemove(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	ids := make([]string, raceRounds)
	fns := make([]func() error, 0, 2*raceRounds)
	for i := range raceRounds {
		ids[i] = newRoot(t, a, fmt.Sprintf("Old%d", i))
		fns = append(fns,
			func() error {
				_, err := a.RenameEntity(ctx, app.RenameEntityRequest{
					EntityID: ids[i], NewName: fmt.Sprintf("New%d", i), ActorID: "test",
				})
				return err
			},
			func() error {
				return a.RemoveEntity(ctx, app.RemoveEntityRequest{EntityID: ids[i], ActorID: "test"})
			},
		)
	}
	errs := race(fns...)
	assertNoBusy(t, errs...)

	for i := range raceRounds {
		renameErr, removeErr := errs[2*i], errs[2*i+1]
		require.NoError(t, removeErr, "round %d: a rename never blocks removal", i)

		renamed := lastEventID(t, a, ids[i], inventory.EntityRenamedEvent)
		removed := lastEventID(t, a, ids[i], inventory.EntityRemovedEvent)
		if renameErr == nil {
			require.NotZero(t, renamed, "round %d: successful rename must have written its event", i)
			assert.Less(t, renamed, removed, "round %d: rename event must precede removal", i)
		} else {
			require.ErrorIs(t, renameErr, app.ErrNotFound, "round %d", i)
			assert.Zero(t, renamed, "round %d: failed rename must not write an event", i)
		}
	}
}

// #301: tagging X while X is removed must not append tag events after the
// removal, and one call's tag events commit together or not at all.
func TestTagEntity_ConcurrentRemove(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	ids := make([]string, raceRounds)
	fns := make([]func() error, 0, 2*raceRounds)
	for i := range raceRounds {
		ids[i] = newRoot(t, a, fmt.Sprintf("Tagged%d", i))
		fns = append(fns,
			func() error {
				return a.TagEntity(ctx, app.TagEntityRequest{
					EntityID: ids[i], Add: []string{"red", "blue"}, ActorID: "test",
				})
			},
			func() error {
				return a.RemoveEntity(ctx, app.RemoveEntityRequest{EntityID: ids[i], ActorID: "test"})
			},
		)
	}
	errs := race(fns...)
	assertNoBusy(t, errs...)

	for i := range raceRounds {
		tagErr, removeErr := errs[2*i], errs[2*i+1]
		require.NoError(t, removeErr, "round %d: tagging never blocks removal", i)

		added := countEvents(t, a, ids[i], inventory.EntityTagAddedEvent)
		removed := lastEventID(t, a, ids[i], inventory.EntityRemovedEvent)
		if tagErr == nil {
			assert.Equal(t, 2, added, "round %d: both tags commit together", i)
			assert.Less(t, lastEventID(t, a, ids[i], inventory.EntityTagAddedEvent), removed,
				"round %d: tag events must precede removal", i)
		} else {
			require.ErrorIs(t, tagErr, app.ErrNotFound, "round %d", i)
			assert.Zero(t, added, "round %d: failed tag call must write no events", i)
		}
	}
}
