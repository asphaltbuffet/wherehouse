package app_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// seedAttr creates Garage (root) with child Box, and returns both.
func seedAttr(t *testing.T, a *app.App) (app.EntityResult, app.EntityResult) {
	t.Helper()
	ctx := context.Background()
	garage, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Garage", ActorID: "alice"})
	require.NoError(t, err)
	box, err := a.CreateEntity(
		ctx,
		app.CreateEntityRequest{
			DisplayName: "Box",
			ParentID:    apptesting.MustLookupID(t, a, "Garage"),
			ActorID:     "alice",
		},
	)
	require.NoError(t, err)
	return garage, box
}

func eventTypes(t *testing.T, a *app.App, id string) []inventory.EventType {
	t.Helper()
	hist, err := a.GetHistory(context.Background(), app.GetHistoryRequest{EntityID: id, OldestFirst: true})
	require.NoError(t, err)
	out := make([]inventory.EventType, len(hist))
	for i, h := range hist {
		out[i] = h.EventType
	}
	return out
}

func TestSetAttributes_SetsAndClears(t *testing.T) {
	cases := []struct {
		name         string
		start        app.CreateEntityRequest
		req          app.SetAttributesRequest
		wantLocked   bool
		wantDiscrete bool
		wantEvent    inventory.EventType
	}{
		{"lock", app.CreateEntityRequest{DisplayName: "Shelf"},
			app.SetAttributesRequest{Locked: new(true)}, true, false, inventory.EntityLockedEvent},
		{"unlock", app.CreateEntityRequest{DisplayName: "Shelf", Locked: true},
			app.SetAttributesRequest{Locked: new(false)}, false, false, inventory.EntityUnlockedEvent},
		{"mark discrete", app.CreateEntityRequest{DisplayName: "Nails"},
			app.SetAttributesRequest{Discrete: new(true)}, false, true, inventory.EntityDiscreteSetEvent},
		{"clear discrete", app.CreateEntityRequest{DisplayName: "Nails", Discrete: true},
			app.SetAttributesRequest{Discrete: new(false)}, false, false, inventory.EntityDiscreteClearedEvent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := openTestApp(t)
			ctx := context.Background()
			tc.start.ActorID = "alice"
			e, err := a.CreateEntity(ctx, tc.start)
			require.NoError(t, err)

			tc.req.EntityID = e.EntityID
			tc.req.ActorID = "bob"
			tc.req.Note = "why"
			results, err := a.SetAttributes(ctx, []app.SetAttributesRequest{tc.req})
			require.NoError(t, err)
			require.Len(t, results, 1)
			assert.Equal(t, tc.wantLocked, results[0].Locked)
			assert.Equal(t, tc.wantDiscrete, results[0].Discrete)

			hist, err := a.GetHistory(ctx, app.GetHistoryRequest{EntityID: e.EntityID, OldestFirst: true})
			require.NoError(t, err)
			last := hist[len(hist)-1]
			assert.Equal(t, tc.wantEvent, last.EventType)
			assert.Equal(t, "bob", last.ActorUserID)
			assert.Equal(t, "why", last.Note)
		})
	}
}

func TestSetAttributes_BothInOneRequest_LockedFirst(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()
	e, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Nails", Locked: true, ActorID: "alice"})
	require.NoError(t, err)

	results, err := a.SetAttributes(ctx, []app.SetAttributesRequest{{
		EntityID: e.EntityID, Discrete: new(true), Locked: new(false), ActorID: "alice",
	}})
	require.NoError(t, err)
	assert.False(t, results[0].Locked)
	assert.True(t, results[0].Discrete)

	types := eventTypes(t, a, e.EntityID)
	require.GreaterOrEqual(t, len(types), 2)
	assert.Equal(t,
		[]inventory.EventType{inventory.EntityUnlockedEvent, inventory.EntityDiscreteSetEvent},
		types[len(types)-2:])
}

func TestSetAttributes_NoChangeIsErrorWithoutEvent(t *testing.T) {
	cases := []struct {
		name  string
		start app.CreateEntityRequest
		req   app.SetAttributesRequest
		msg   string
	}{
		{"already locked", app.CreateEntityRequest{DisplayName: "Shelf", Locked: true},
			app.SetAttributesRequest{Locked: new(true)}, `cannot lock "Shelf": already locked`},
		{"not locked", app.CreateEntityRequest{DisplayName: "Shelf"},
			app.SetAttributesRequest{Locked: new(false)}, `cannot unlock "Shelf": not locked`},
		{"already discrete", app.CreateEntityRequest{DisplayName: "Nails", Discrete: true},
			app.SetAttributesRequest{Discrete: new(true)}, `cannot mark "Nails" discrete: already discrete`},
		{"not discrete", app.CreateEntityRequest{DisplayName: "Nails"},
			app.SetAttributesRequest{Discrete: new(false)}, `cannot clear discrete on "Nails": not discrete`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := openTestApp(t)
			ctx := context.Background()
			tc.start.ActorID = "alice"
			e, err := a.CreateEntity(ctx, tc.start)
			require.NoError(t, err)
			before := len(eventTypes(t, a, e.EntityID))

			tc.req.EntityID = e.EntityID
			_, err = a.SetAttributes(ctx, []app.SetAttributesRequest{tc.req})
			require.ErrorIs(t, err, app.ErrNoChange)
			require.ErrorContains(t, err, tc.msg)
			assert.Len(t, eventTypes(t, a, e.EntityID), before, "no event may be written")
		})
	}
}

func TestSetAttributes_LockRequiresOkOrBorrowed(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()
	_, box := seedAttr(t, a)

	// missing
	_, err := a.MarkLost(ctx, []app.ChangeStatusRequest{{EntityID: box.EntityID, ActorID: "alice"}})
	require.NoError(t, err)
	_, err = a.SetAttributes(ctx, []app.SetAttributesRequest{{EntityID: box.EntityID, Locked: new(true)}})
	require.ErrorContains(t, err, "only ok or borrowed entities can be locked")

	// loaned
	_, err = a.MarkLoaned(ctx, []app.ChangeStatusRequest{
		{EntityID: box.EntityID, StatusContext: "Bob", ActorID: "alice"},
	})
	require.NoError(t, err)
	_, err = a.SetAttributes(ctx, []app.SetAttributesRequest{{EntityID: box.EntityID, Locked: new(true)}})
	require.ErrorContains(t, err, "only ok or borrowed entities can be locked")

	// borrowed is allowed
	borrowed, err := a.BorrowEntities(ctx, []app.BorrowEntityRequest{
		{
			DisplayName:   "Bench",
			ParentID:      apptesting.MustLookupID(t, a, "Garage"),
			StatusContext: "Alice",
			ActorID:       "alice",
		},
	})
	require.NoError(t, err)
	results, err := a.SetAttributes(ctx, []app.SetAttributesRequest{
		{EntityID: borrowed[0].EntityID, Locked: new(true)},
	})
	require.NoError(t, err)
	assert.True(t, results[0].Locked)
}

func TestSetAttributes_DiscreteRequiresNoLiveChildren(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()
	garage, box := seedAttr(t, a)

	_, err := a.SetAttributes(ctx, []app.SetAttributesRequest{{EntityID: garage.EntityID, Discrete: new(true)}})
	require.ErrorIs(t, err, app.ErrHasChildren)
	require.ErrorContains(t, err, `cannot mark "Garage" discrete (1 inside; move or remove them first)`)

	// A removed child does not count.
	require.NoError(t, a.RemoveEntity(ctx, app.RemoveEntityRequest{EntityID: box.EntityID, ActorID: "alice"}))
	results, err := a.SetAttributes(ctx, []app.SetAttributesRequest{{EntityID: garage.EntityID, Discrete: new(true)}})
	require.NoError(t, err)
	assert.True(t, results[0].Discrete)
}

func TestSetAttributes_RemovedEntityNotFound(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()
	_, box := seedAttr(t, a)
	require.NoError(t, a.RemoveEntity(ctx, app.RemoveEntityRequest{EntityID: box.EntityID, ActorID: "alice"}))

	_, err := a.SetAttributes(ctx, []app.SetAttributesRequest{{EntityID: box.EntityID, Locked: new(false)}})
	assert.ErrorIs(t, err, app.ErrNotFound)
}

func TestSetAttributes_AtomicAcrossBatch(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()
	garage, box := seedAttr(t, a)
	_, err := a.SetAttributes(ctx, []app.SetAttributesRequest{{EntityID: garage.EntityID, Locked: new(true)}})
	require.NoError(t, err)

	_, err = a.SetAttributes(ctx, []app.SetAttributesRequest{
		{EntityID: box.EntityID, Locked: new(true)},
		{EntityID: garage.EntityID, Locked: new(true)}, // already locked
	})
	require.ErrorIs(t, err, app.ErrNoChange)

	got, err := a.GetEntityByID(ctx, box.EntityID)
	require.NoError(t, err)
	assert.False(t, got.Locked, "first request must roll back")
}

func TestSetAttributes_AtomicWithinRequest(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()
	garage, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Garage", Locked: true, ActorID: "alice"})
	require.NoError(t, err)
	_, err = a.CreateEntity(
		ctx,
		app.CreateEntityRequest{
			DisplayName: "Box",
			ParentID:    apptesting.MustLookupID(t, a, "Garage"),
			ActorID:     "alice",
		},
	)
	require.NoError(t, err)

	_, err = a.SetAttributes(ctx, []app.SetAttributesRequest{{
		EntityID: garage.EntityID, Locked: new(false), Discrete: new(true),
	}})
	require.ErrorIs(t, err, app.ErrHasChildren)

	got, err := a.GetEntityByID(ctx, garage.EntityID)
	require.NoError(t, err)
	assert.True(t, got.Locked, "valid unlock must not apply when discrete fails")
}

func TestSetAttributes_RejectsBadBatches(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()
	garage, _ := seedAttr(t, a)

	_, err := a.SetAttributes(ctx, nil)
	require.Error(t, err)

	_, err = a.SetAttributes(ctx, []app.SetAttributesRequest{{EntityID: garage.EntityID}})
	require.ErrorIs(t, err, app.ErrNoAttributes)

	_, err = a.SetAttributes(ctx, []app.SetAttributesRequest{
		{EntityID: garage.EntityID, Locked: new(true)},
		{EntityID: garage.EntityID, Discrete: new(true)},
	})
	require.ErrorIs(t, err, app.ErrDuplicateEntity)
}

func TestSetAttributes_SurvivesReplay(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()
	garage, box := seedAttr(t, a)
	_, err := a.SetAttributes(ctx, []app.SetAttributesRequest{
		{EntityID: garage.EntityID, Locked: new(true)},
		{EntityID: box.EntityID, Discrete: new(true)},
	})
	require.NoError(t, err)

	_, err = a.TruncateAndReplay(ctx)
	require.NoError(t, err)

	g, err := a.GetEntityByID(ctx, garage.EntityID)
	require.NoError(t, err)
	b, err := a.GetEntityByID(ctx, box.EntityID)
	require.NoError(t, err)
	assert.True(t, g.Locked)
	assert.True(t, b.Discrete)
}
