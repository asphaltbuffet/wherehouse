package eventbus_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func createPlace(t *testing.T, b *eventbus.Bus, id, name string, parentID *string) {
	t.Helper()
	createEntity(t, b, id, name, parentID, true, false)
}

func createLeaf(t *testing.T, b *eventbus.Bus, id, name string, parentID *string) {
	t.Helper()
	createEntity(t, b, id, name, parentID, false, true)
}

func createEntity(t *testing.T, b *eventbus.Bus, id, name string, parentID *string, locked, discrete bool) {
	t.Helper()
	p := eventbus.EntityCreatedPayload{
		EntityID: id, DisplayName: name, Locked: locked, Discrete: discrete, ParentID: parentID,
	}
	raw, err := json.Marshal(p)
	require.NoError(t, err)
	_, err = b.Dispatch(context.Background(), inventory.EntityCreatedEvent, "test", raw, nil)
	require.NoError(t, err)
}

func TestEntityCreated_LegacyPlacePayload_GetsLocked(t *testing.T) {
	// Old events with entity_type="place" in the payload should replay as locked=true.
	s := openTestStore(t)
	b := eventbus.New(s)

	// Dispatch using the raw legacy JSON shape (entity_type field instead of locked/discrete).
	raw := []byte(`{"entity_id":"p1","display_name":"Garage","entity_type":"place"}`)
	_, err := b.Dispatch(context.Background(), inventory.EntityCreatedEvent, "test", raw, nil)
	require.NoError(t, err)

	e, err := s.GetEntity(context.Background(), "p1")
	require.NoError(t, err)
	assert.True(t, e.Locked, "legacy place entity should be replayed as locked=true")
	assert.False(t, e.Discrete)
}

func TestEntityCreated_LegacyLeafPayload_GetsDiscrete(t *testing.T) {
	// Old events with entity_type="leaf" should replay as locked=false, discrete=true.
	s := openTestStore(t)
	b := eventbus.New(s)

	raw := []byte(`{"entity_id":"l1","display_name":"Wrench","entity_type":"leaf"}`)
	_, err := b.Dispatch(context.Background(), inventory.EntityCreatedEvent, "test", raw, nil)
	require.NoError(t, err)

	e, err := s.GetEntity(context.Background(), "l1")
	require.NoError(t, err)
	assert.False(t, e.Locked)
	assert.True(t, e.Discrete)
}

func TestEntityCreated_LegacyContainerPayload_IsUnlockedNonDiscrete(t *testing.T) {
	// Old events with entity_type="container" map to the zero value: locked=false, discrete=false.
	s := openTestStore(t)
	b := eventbus.New(s)

	raw := []byte(`{"entity_id":"c1","display_name":"Toolbox","entity_type":"container"}`)
	_, err := b.Dispatch(context.Background(), inventory.EntityCreatedEvent, "test", raw, nil)
	require.NoError(t, err)

	e, err := s.GetEntity(context.Background(), "c1")
	require.NoError(t, err)
	assert.False(t, e.Locked)
	assert.False(t, e.Discrete)
}

func TestEntityCreated_NewPayload_LockedAndDiscrete(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)

	createEntity(t, b, "e1", "Box of Nails", nil, false, true)

	e, err := s.GetEntity(context.Background(), "e1")
	require.NoError(t, err)
	assert.False(t, e.Locked)
	assert.True(t, e.Discrete)
}

func TestEntityCreated_NestedUnderEntity_OK(t *testing.T) {
	// The bus handler enforces no parent-type restrictions.
	// Discrete-parent enforcement is at the app layer — see TestCreateEntity_DiscreteParent_Error.
	s := openTestStore(t)
	b := eventbus.New(s)

	createEntity(t, b, "p1", "Garage", nil, true, false)
	p1ID := "p1"
	createEntity(t, b, "c1", "Toolbox", &p1ID, false, false)

	e, err := s.GetEntity(context.Background(), "c1")
	require.NoError(t, err)
	assert.Equal(t, "p1", *e.ParentID)
}
