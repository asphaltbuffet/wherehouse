package eventbus_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(store.Config{
		Path:        filepath.Join(t.TempDir(), "test.db"),
		AutoMigrate: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestDispatch_UnknownEventType(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)

	payload := json.RawMessage(`{}`)
	_, err := b.Dispatch(context.Background(), inventory.EventType(999), "alice", payload, nil)
	assert.Error(t, err)
}

func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

func TestReplayEvent_ReturnsNewDBAssignedID(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)
	ctx := context.Background()

	payload := mustMarshal(t, eventbus.EntityCreatedPayload{
		EntityID: "r1", DisplayName: "Replay Room", Locked: true,
	})
	ev := &inventory.Event{
		EventType:    inventory.EntityCreatedEvent,
		TimestampUTC: "2020-01-01T00:00:00Z",
		ActorUserID:  "alice",
		Payload:      payload,
	}

	id, _, err := b.ReplayEvent(ctx, ev)
	require.NoError(t, err)
	assert.Positive(t, id)
}

func TestReplayEvent_PreservesOriginalTimestamp(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)
	ctx := context.Background()

	const originalTS = "2020-06-15T12:00:00Z"
	payload := mustMarshal(t, eventbus.EntityCreatedPayload{
		EntityID: "r2", DisplayName: "Time Capsule", Locked: true,
	})
	ev := &inventory.Event{
		EventType:    inventory.EntityCreatedEvent,
		TimestampUTC: originalTS,
		ActorUserID:  "alice",
		Payload:      payload,
	}

	id, _, err := b.ReplayEvent(ctx, ev)
	require.NoError(t, err)

	stored, err := s.GetEventByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, originalTS, stored.TimestampUTC)
}

func TestReplayEvent_PathChangedEvent_IsNoOp(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)
	ctx := context.Background()

	payload := mustMarshal(t, eventbus.EntityPathChangedPayload{
		EntityID: "p99", FullPathDisplay: "Ghost:Path", FullPathCanonical: "ghost:path", Depth: 2,
	})
	ev := &inventory.Event{
		EventType:    inventory.EntityPathChangedEvent,
		TimestampUTC: "2021-03-01T00:00:00Z",
		ActorUserID:  "system",
		Payload:      payload,
	}

	id, _, err := b.ReplayEvent(ctx, ev)
	require.NoError(t, err)
	assert.Zero(t, id, "path-changed replay should return 0 (no row inserted)")

	// Confirm nothing was written
	_, err = s.GetEventByID(ctx, 1)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestReplayEvent_NonPathChangedEvent_AppliesProjection(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)
	ctx := context.Background()

	payload := mustMarshal(t, eventbus.EntityCreatedPayload{
		EntityID: "r3", DisplayName: "Projected Room", Locked: true,
	})
	ev := &inventory.Event{
		EventType:    inventory.EntityCreatedEvent,
		TimestampUTC: "2022-09-10T08:00:00Z",
		ActorUserID:  "bob",
		Payload:      payload,
	}

	_, _, err := b.ReplayEvent(ctx, ev)
	require.NoError(t, err)

	entity, err := s.GetEntity(ctx, "r3")
	require.NoError(t, err)
	assert.Equal(t, "Projected Room", entity.DisplayName)
}

func TestPayloadFactories_CoversAllEventTypes(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)

	factories := b.PayloadFactories()

	allTypes := []inventory.EventType{
		inventory.EntityCreatedEvent,
		inventory.EntityRenamedEvent,
		inventory.EntityReparentedEvent,
		inventory.EntityPathChangedEvent,
		inventory.EntityStatusChangedEvent,
		inventory.EntityRemovedEvent,
		inventory.EntityUsedEvent,
		inventory.EntityReturnedHomeEvent,
		inventory.EntityRehomedEvent,
	}

	for _, et := range allTypes {
		factory, ok := factories[et]
		require.True(t, ok, "PayloadFactories missing entry for %s", et)
		require.NotNil(t, factory, "factory for %s is nil", et)
		instance := factory()
		assert.NotNil(t, instance, "factory() for %s returned nil", et)
	}
}

func dispatch(t *testing.T, b *eventbus.Bus, et inventory.EventType, payload any) {
	t.Helper()
	_, err := b.Dispatch(t.Context(), et, "test", mustMarshal(t, payload), nil)
	require.NoError(t, err)
}

func TestHome_FollowsCreateAndReparent(t *testing.T) {
	ctx := t.Context()
	s := openTestStore(t)
	b := eventbus.New(s)
	created := func(id, name string, parent *string) {
		dispatch(t, b, inventory.EntityCreatedEvent,
			eventbus.EntityCreatedPayload{EntityID: id, DisplayName: name, ParentID: parent})
	}
	garage := "garage0001"
	created(garage, "Garage", nil)
	created("shed000001", "Shed", nil)
	created("drill00001", "Drill", &garage)

	e, err := s.GetEntity(ctx, "drill00001")
	require.NoError(t, err)
	require.NotNil(t, e.HomeID)
	assert.Equal(t, garage, *e.HomeID)

	shed := "shed000001"
	dispatch(t, b, inventory.EntityReparentedEvent,
		eventbus.EntityReparentedPayload{EntityID: "drill00001", NewParentID: &shed})
	e, err = s.GetEntity(ctx, "drill00001")
	require.NoError(t, err)
	require.NotNil(t, e.HomeID)
	assert.Equal(t, shed, *e.HomeID)
	assert.False(t, e.IsDisplaced())

	_, err = b.TruncateAndReplay(ctx)
	require.NoError(t, err)
	e, err = s.GetEntity(ctx, "drill00001")
	require.NoError(t, err)
	require.NotNil(t, e.HomeID)
	assert.Equal(t, shed, *e.HomeID, "replay must reproduce home")
}

func newTestBus(t *testing.T) (*store.Store, *eventbus.Bus) {
	t.Helper()
	s := openTestStore(t)
	return s, eventbus.New(s)
}

func seedDrillInGarage(t *testing.T, b *eventbus.Bus) {
	t.Helper()
	create := func(id, name string, parent *string) {
		t.Helper()
		dispatch(t, b, inventory.EntityCreatedEvent,
			eventbus.EntityCreatedPayload{EntityID: id, DisplayName: name, ParentID: parent})
	}
	garage, drill := "garage0001", "drill00001"
	create(garage, "Garage", nil)
	create("deck000001", "Deck", nil)
	create(drill, "Drill", &garage)
	create("bit0000001", "Bit", &drill)
}

func TestUsed_KeepsHomeAndPropagatesPaths(t *testing.T) {
	ctx := t.Context()
	s, b := newTestBus(t)
	seedDrillInGarage(t, b)
	deck := "deck000001"
	useDrill(t, b, &deck)

	drill, err := s.GetEntity(ctx, "drill00001")
	require.NoError(t, err)
	assert.Equal(t, "Deck:Drill", drill.FullPathDisplay)
	require.NotNil(t, drill.HomeID)
	assert.Equal(t, "garage0001", *drill.HomeID)
	assert.True(t, drill.IsDisplaced())

	bit, err := s.GetEntity(ctx, "bit0000001")
	require.NoError(t, err)
	assert.Equal(t, "Deck:Drill:Bit", bit.FullPathDisplay)
	assert.False(t, bit.IsDisplaced(), "descendants travel with the container")
}

func TestReturnedHome_And_Rehomed(t *testing.T) {
	ctx := t.Context()
	s, b := newTestBus(t)
	seedDrillInGarage(t, b)
	deck, garage := "deck000001", "garage0001"
	useDrill(t, b, &deck)
	returnDrill(t, b, &garage)

	drill, err := s.GetEntity(ctx, "drill00001")
	require.NoError(t, err)
	assert.Equal(t, "Garage:Drill", drill.FullPathDisplay)
	assert.False(t, drill.IsDisplaced())

	useDrill(t, b, &deck)
	dispatch(t, b, inventory.EntityRehomedEvent, eventbus.EntityRehomedPayload{EntityID: "drill00001", HomeID: &deck})
	drill, err = s.GetEntity(ctx, "drill00001")
	require.NoError(t, err)
	require.NotNil(t, drill.HomeID)
	assert.Equal(t, deck, *drill.HomeID)
	assert.False(t, drill.IsDisplaced())
}

func TestHomeEvents_ReplayEquivalence(t *testing.T) {
	ctx := t.Context()
	s, b := newTestBus(t)
	seedDrillInGarage(t, b)
	deck, garage := "deck000001", "garage0001"
	useDrill(t, b, &deck)
	returnDrill(t, b, &garage)
	useDrill(t, b, &deck)

	before, err := s.ListAllEntities(ctx)
	require.NoError(t, err)
	eventsBefore, err := s.GetAllEvents(ctx)
	require.NoError(t, err)

	_, err = b.TruncateAndReplay(ctx)
	require.NoError(t, err)

	after, err := s.ListAllEntities(ctx)
	require.NoError(t, err)
	eventsAfter, err := s.GetAllEvents(ctx)
	require.NoError(t, err)
	assert.Len(t, eventsAfter, len(eventsBefore), "replay must not write events")
	require.Len(t, after, len(before))
	for i := range before {
		assert.Equal(t, before[i].FullPathDisplay, after[i].FullPathDisplay)
		assert.Equal(t, before[i].HomeID, after[i].HomeID)
		assert.Equal(t, before[i].ParentID, after[i].ParentID)
	}
}

func TestCountDisplacedWithHomeTx(t *testing.T) {
	ctx := t.Context()
	s, b := newTestBus(t)
	seedDrillInGarage(t, b)
	deck, garage := "deck000001", "garage0001"

	count := func(homeID string) int {
		t.Helper()
		var n int
		require.NoError(t, s.ExecInTransaction(ctx, func(tx store.Tx) error {
			var err error
			n, err = s.CountDisplacedWithHomeTx(ctx, tx, homeID)
			return err
		}))
		return n
	}

	assert.Equal(t, 0, count(garage), "nothing is displaced yet")

	useDrill(t, b, &deck)
	assert.Equal(t, 1, count(garage), "the used drill is displaced from its garage home")
	assert.Equal(t, 0, count(deck), "deck is not the drill's home")
	assert.Equal(t, 0, count("drill00001"), "the bit travels with the drill and is not displaced")

	dispatch(t, b, inventory.EntityRemovedEvent, eventbus.EntityRemovedPayload{EntityID: "drill00001"})
	assert.Equal(t, 0, count(garage), "a removed displaced entity is not counted")
}

func useDrill(t *testing.T, b *eventbus.Bus, parent *string) {
	t.Helper()
	dispatch(t, b, inventory.EntityUsedEvent,
		eventbus.EntityUsedPayload{EntityID: "drill00001", NewParentID: parent})
}

func returnDrill(t *testing.T, b *eventbus.Bus, parent *string) {
	t.Helper()
	dispatch(t, b, inventory.EntityReturnedHomeEvent,
		eventbus.EntityReturnedHomePayload{EntityID: "drill00001", ParentID: parent})
}
