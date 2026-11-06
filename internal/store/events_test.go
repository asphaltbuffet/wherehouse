package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// appendEvent writes ev in its own transaction and returns its ID.
func appendEvent(t *testing.T, s *store.Store, ev *inventory.Event) int64 {
	t.Helper()
	var id int64
	require.NoError(t, s.ExecInTransaction(t.Context(), func(tx store.Tx) error {
		var err error
		id, err = s.AppendEvent(t.Context(), tx, ev)
		return err
	}))
	return id
}

// garageEvent returns an EntityCreatedEvent for entity "abc".
func garageEvent() *inventory.Event {
	entityID := "abc"
	return &inventory.Event{
		EventType:    inventory.EntityCreatedEvent,
		TimestampUTC: "2020-01-02T03:04:05Z",
		ActorUserID:  "alice",
		Payload:      json.RawMessage(`{"entity_id":"abc","display_name":"Garage"}`),
		EntityID:     &entityID,
	}
}

func TestAppendEvent_StoresFieldsVerbatim(t *testing.T) {
	s := openTestStore(t)
	note := "first"
	ev := garageEvent()
	ev.Note = &note

	id := appendEvent(t, s, ev)

	got, err := s.GetEventByID(t.Context(), id)
	require.NoError(t, err)
	assert.Equal(t, id, got.EventID)
	assert.Equal(t, inventory.EntityCreatedEvent, got.EventType)
	assert.Equal(t, "2020-01-02T03:04:05Z", got.TimestampUTC, "timestamp is the caller's, not re-stamped")
	assert.Equal(t, "alice", got.ActorUserID)
	assert.JSONEq(t, string(ev.Payload), string(got.Payload))
	require.NotNil(t, got.Note)
	assert.Equal(t, "first", *got.Note)
	require.NotNil(t, got.EntityID)
	assert.Equal(t, "abc", *got.EntityID)
	assert.Zero(t, ev.EventID, "AppendEvent must not mutate its input")
}

func TestAppendEvent_NilNoteIsNull(t *testing.T) {
	s := openTestStore(t)

	id := appendEvent(t, s, garageEvent())

	got, err := s.GetEventByID(t.Context(), id)
	require.NoError(t, err)
	assert.Nil(t, got.Note)
}

func TestAppendEvent_IDsIncrease(t *testing.T) {
	s := openTestStore(t)

	first := appendEvent(t, s, garageEvent())
	second := appendEvent(t, s, garageEvent())

	assert.Greater(t, second, first)
}

func TestAppendEvent_RolledBackWithTransaction(t *testing.T) {
	s := openTestStore(t)
	errAbort := errors.New("abort")

	err := s.ExecInTransaction(t.Context(), func(tx store.Tx) error {
		if _, appendErr := s.AppendEvent(t.Context(), tx, garageEvent()); appendErr != nil {
			return appendErr
		}
		return errAbort
	})
	require.ErrorIs(t, err, errAbort)

	has, err := s.HasEvents(t.Context())
	require.NoError(t, err)
	assert.False(t, has, "AppendEvent must join the caller's transaction")
}

func TestGetEventByID(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	eventID := appendEvent(t, s, garageEvent())

	ev, err := s.GetEventByID(ctx, eventID)
	require.NoError(t, err)
	assert.Equal(t, inventory.EntityCreatedEvent, ev.EventType)
	assert.Equal(t, "alice", ev.ActorUserID)
}

func TestGetEventsByEntity(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	appendEvent(t, s, garageEvent())

	events, err := s.GetEventsByEntity(ctx, "abc")
	require.NoError(t, err)
	assert.Len(t, events, 1)
}

func TestHasEvents_EmptyDatabase(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	has, err := s.HasEvents(ctx)
	require.NoError(t, err)
	assert.False(t, has)
}

func TestHasEvents_AfterInsert(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	appendEvent(t, s, garageEvent())

	has, err := s.HasEvents(ctx)
	require.NoError(t, err)
	assert.True(t, has)
}

func TestClearAllData_RemovesEventsAndEntities(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	// Seed an event and an entity projection
	appendEvent(t, s, garageEvent())

	err := s.ClearAllData(ctx)
	require.NoError(t, err)

	has, err := s.HasEvents(ctx)
	require.NoError(t, err)
	assert.False(t, has, "events table should be empty after ClearAllData")

	_, err = s.GetEntity(ctx, "abc")
	assert.ErrorIs(t, err, store.ErrNotFound, "entities_current should be empty after ClearAllData")
}

func TestClearAllData_RemovesTaggedEntities(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	entityID := seedTagEntity(t, s)
	require.NoError(t, s.ExecInTransaction(ctx, func(tx store.Tx) error {
		return s.InsertTagTx(ctx, tx, entityID, "tool")
	}))

	require.NoError(t, s.ClearAllData(ctx))

	tags, err := s.GetDistinctTags(ctx)
	require.NoError(t, err)
	assert.Empty(t, tags)
	_, err = s.GetEntityAnyStatus(ctx, entityID)
	assert.ErrorIs(t, err, store.ErrNotFound)
}

func TestClearAllData_PreservesSchemaMetadata(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	err := s.ClearAllData(ctx)
	require.NoError(t, err)

	// schema_metadata is seeded by the migration with "created_at" and "app_version"
	val, err := s.GetMetadata(ctx, "app_version")
	require.NoError(t, err)
	assert.NotEmpty(t, val, "schema_metadata should survive ClearAllData")
}

func TestGetAllEventsTx_SeesUncommittedRows(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	entityID := "abc"
	appendEvent(t, s, garageEvent())

	err := s.ExecInTransaction(ctx, func(tx store.Tx) error {
		_, execErr := tx.ExecContext(ctx,
			`INSERT INTO events (event_type, timestamp_utc, actor_user_id, payload, note, entity_id)
			 VALUES (?, ?, ?, ?, NULL, ?)`,
			"entity.renamed", "2026-01-01T00:00:00Z", "alice", `{}`, &entityID,
		)
		require.NoError(t, execErr)

		events, getErr := s.GetAllEventsTx(ctx, tx)
		require.NoError(t, getErr)
		require.Len(t, events, 2)
		assert.Less(t, events[0].EventID, events[1].EventID)
		return nil
	})
	require.NoError(t, err)
}

func TestGetAllEventsRaw(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	entityID := "abc"
	appendEvent(t, s, garageEvent())

	// Insert an event with an unknown type directly — AppendEvent enforces typed EventType.
	_, err := s.DB().ExecContext(ctx,
		`INSERT INTO events (event_type, timestamp_utc, actor_user_id, payload, note, entity_id)
		 VALUES (?, ?, ?, ?, NULL, ?)`,
		"entity.unknown_future", "2026-01-01T00:00:00Z", "alice", `{}`, &entityID,
	)
	require.NoError(t, err)

	events, err := s.GetAllEventsRaw(ctx)
	require.NoError(t, err)
	require.Len(t, events, 2)

	assert.Equal(t, "entity.created", events[0].EventType)
	assert.Equal(t, "entity.unknown_future", events[1].EventType)
	assert.NotNil(t, events[0].EntityID)
	assert.Equal(t, "abc", *events[0].EntityID)
}
