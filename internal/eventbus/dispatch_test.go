package eventbus_test

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// dispatch writes p in its own transaction and returns the new event_id.
func dispatch(t *testing.T, s *store.Store, b *eventbus.Bus, p eventbus.Payload) int64 {
	t.Helper()
	var id int64
	require.NoError(t, s.ExecInTransaction(t.Context(), func(tx store.Tx) error {
		var err error
		id, err = b.Dispatch(t.Context(), tx, "test", p, "")
		return err
	}))
	return id
}

func readEventRow(t *testing.T, s *store.Store, id int64) (string, sql.NullString, sql.NullString) {
	t.Helper()
	var eventType string
	var entityID, note sql.NullString
	require.NoError(t, s.DB().QueryRowContext(t.Context(),
		`SELECT event_type, entity_id, note FROM events WHERE event_id = ?`, id,
	).Scan(&eventType, &entityID, &note))
	return eventType, entityID, note
}

func TestDispatch_TypeAndEntityIDComeFromPayload(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)

	id := dispatch(t, s, b, eventbus.EntityCreatedPayload{EntityID: "g1", DisplayName: "Garage"})

	et, entityID, note := readEventRow(t, s, id)
	assert.Equal(t, "entity.created", et)
	assert.Equal(t, "g1", entityID.String)
	assert.False(t, note.Valid, "empty note is stored as NULL")
}

func TestDispatch_NoteIsStored(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)

	var id int64
	require.NoError(t, s.ExecInTransaction(t.Context(), func(tx store.Tx) error {
		var err error
		id, err = b.Dispatch(t.Context(), tx, "test",
			eventbus.EntityCreatedPayload{EntityID: "g1", DisplayName: "Garage"}, "corrective")
		return err
	}))

	_, _, note := readEventRow(t, s, id)
	assert.Equal(t, sql.NullString{String: "corrective", Valid: true}, note)
}

func TestDispatch_EmptySubject_Rejected(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)

	err := s.ExecInTransaction(t.Context(), func(tx store.Tx) error {
		_, err := b.Dispatch(t.Context(), tx, "test", eventbus.EntityRemovedPayload{}, "")
		return err
	})
	require.ErrorIs(t, err, eventbus.ErrNoSubject)
}

func TestDispatch_DerivedEvent_Rejected(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)
	dispatch(t, s, b, eventbus.EntityCreatedPayload{EntityID: "g1", DisplayName: "Garage"})

	err := s.ExecInTransaction(t.Context(), func(tx store.Tx) error {
		_, err := b.Dispatch(t.Context(), tx, "test", eventbus.EntityPathChangedPayload{
			EntityID: "g1", FullPathDisplay: "Elsewhere", FullPathCanonical: "elsewhere", Depth: 1,
		}, "")
		return err
	})
	require.ErrorIs(t, err, eventbus.ErrDerivedEvent)

	var n int
	require.NoError(t, s.DB().QueryRowContext(t.Context(),
		`SELECT COUNT(*) FROM events WHERE event_type = 'entity.path_changed'`).Scan(&n))
	assert.Zero(t, n)
}

func TestDispatch_PathChangedRowsCopyTriggeringEvent(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)
	p1ID := "p1"
	createPlace(t, s, b, "p1", "Garage", nil)
	createPlace(t, s, b, "c1", "Toolbox", &p1ID)

	var renameID int64
	require.NoError(t, s.ExecInTransaction(t.Context(), func(tx store.Tx) error {
		var err error
		renameID, err = b.Dispatch(t.Context(), tx, "bob",
			eventbus.EntityRenamedPayload{EntityID: "p1", DisplayName: "Shed"}, "renamed")
		return err
	}))

	rename, err := s.Reader().GetEventByID(t.Context(), renameID)
	require.NoError(t, err)
	events, err := s.Reader().GetEventsByEntity(t.Context(), "c1")
	require.NoError(t, err)
	derived := events[len(events)-1]

	assert.Equal(t, inventory.EntityPathChangedEvent, derived.EventType)
	assert.Greater(t, derived.EventID, renameID)
	assert.Equal(t, rename.TimestampUTC, derived.TimestampUTC)
	assert.Equal(t, "bob", derived.ActorUserID)
	assert.Nil(t, derived.Note, "derived rows never carry the triggering note")
	require.NotNil(t, derived.EntityID)
	assert.Equal(t, "c1", *derived.EntityID)

	child, err := s.Reader().GetLiveEntity(t.Context(), "c1")
	require.NoError(t, err)
	assert.Equal(t, derived.EventID, child.LastEventID)
	assert.Equal(t, "Shed:Toolbox", child.FullPathDisplay)
}

func TestDispatch_RenameUpdatesRemovedDescendantPath(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)
	p1ID := "p1"
	createPlace(t, s, b, "p1", "Garage", nil)
	createLeaf(t, s, b, "c1", "Wrench", &p1ID)
	dispatch(t, s, b, eventbus.EntityRemovedPayload{EntityID: "c1"})

	dispatch(t, s, b, eventbus.EntityRenamedPayload{EntityID: "p1", DisplayName: "Shed"})

	child, err := s.Reader().GetEntityAnyStatus(t.Context(), "c1")
	require.NoError(t, err)
	assert.Equal(t, "Shed:Wrench", child.FullPathDisplay,
		"removed descendants keep following their ancestor's path")
	events, err := s.Reader().GetEventsByEntity(t.Context(), "c1")
	require.NoError(t, err)
	assert.Equal(t, inventory.EntityPathChangedEvent, events[len(events)-1].EventType)
}

func TestDispatch_ReparentUpdatesRemovedDescendantPath(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)
	p1ID, p2ID := "p1", "p2"
	createPlace(t, s, b, "p1", "Garage", nil)
	createPlace(t, s, b, "p2", "Shed", nil)
	createLeaf(t, s, b, "c1", "Wrench", &p1ID)
	dispatch(t, s, b, eventbus.EntityRemovedPayload{EntityID: "c1"})

	dispatch(t, s, b, eventbus.EntityReparentedPayload{EntityID: "p1", NewParentID: &p2ID})

	child, err := s.Reader().GetEntityAnyStatus(t.Context(), "c1")
	require.NoError(t, err)
	assert.Equal(t, "Shed:Garage:Wrench", child.FullPathDisplay,
		"removed descendants keep following their ancestor's path")
	events, err := s.Reader().GetEventsByEntity(t.Context(), "c1")
	require.NoError(t, err)
	assert.Equal(t, inventory.EntityPathChangedEvent, events[len(events)-1].EventType)
}
