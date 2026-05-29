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
		EntityID: "r1", DisplayName: "Replay Room", EntityType: "place",
	})
	ev := &inventory.Event{
		EventType:    inventory.EntityCreatedEvent,
		TimestampUTC: "2020-01-01T00:00:00Z",
		ActorUserID:  "alice",
		Payload:      payload,
	}

	id, err := b.ReplayEvent(ctx, ev)
	require.NoError(t, err)
	assert.Positive(t, id)
}

func TestReplayEvent_PreservesOriginalTimestamp(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)
	ctx := context.Background()

	const originalTS = "2020-06-15T12:00:00Z"
	payload := mustMarshal(t, eventbus.EntityCreatedPayload{
		EntityID: "r2", DisplayName: "Time Capsule", EntityType: "place",
	})
	ev := &inventory.Event{
		EventType:    inventory.EntityCreatedEvent,
		TimestampUTC: originalTS,
		ActorUserID:  "alice",
		Payload:      payload,
	}

	id, err := b.ReplayEvent(ctx, ev)
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

	id, err := b.ReplayEvent(ctx, ev)
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
		EntityID: "r3", DisplayName: "Projected Room", EntityType: "place",
	})
	ev := &inventory.Event{
		EventType:    inventory.EntityCreatedEvent,
		TimestampUTC: "2022-09-10T08:00:00Z",
		ActorUserID:  "bob",
		Payload:      payload,
	}

	_, err := b.ReplayEvent(ctx, ev)
	require.NoError(t, err)

	entity, err := s.GetEntity(ctx, "r3")
	require.NoError(t, err)
	assert.Equal(t, "Projected Room", entity.DisplayName)
}
