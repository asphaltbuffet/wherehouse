package eventbus_test

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
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
