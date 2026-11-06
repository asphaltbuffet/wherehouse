package eventbus_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

func createPlace(t *testing.T, s *store.Store, b *eventbus.Bus, id, name string, parentID *string) {
	t.Helper()
	createEntity(t, s, b, id, name, parentID, true, false)
}

func createLeaf(t *testing.T, s *store.Store, b *eventbus.Bus, id, name string, parentID *string) {
	t.Helper()
	createEntity(t, s, b, id, name, parentID, false, true)
}

func createEntity(
	t *testing.T,
	s *store.Store,
	b *eventbus.Bus,
	id, name string,
	parentID *string,
	locked, discrete bool,
) {
	t.Helper()
	p := eventbus.EntityCreatedPayload{
		EntityID: id, DisplayName: name, Locked: locked, Discrete: discrete, ParentID: parentID,
	}
	dispatch(t, s, b, p)
}

func TestEntityCreated_LegacyPlacePayload_GetsLocked(t *testing.T) {
	// Old events with entity_type="place" in the payload should replay as locked=true.
	s := openTestStore(t)
	b := eventbus.New(s)

	// LegacyEntityType marshals as the old entity_type field.
	dispatch(t, s, b, eventbus.EntityCreatedPayload{EntityID: "p1", DisplayName: "Garage", LegacyEntityType: "place"})

	e, err := s.Reader().GetLiveEntity(context.Background(), "p1")
	require.NoError(t, err)
	assert.True(t, e.Locked, "legacy place entity should be replayed as locked=true")
	assert.False(t, e.Discrete)
}

func TestEntityCreated_LegacyLeafPayload_GetsDiscrete(t *testing.T) {
	// Old events with entity_type="leaf" should replay as locked=false, discrete=true.
	s := openTestStore(t)
	b := eventbus.New(s)

	// LegacyEntityType marshals as the old entity_type field.
	dispatch(t, s, b, eventbus.EntityCreatedPayload{EntityID: "l1", DisplayName: "Wrench", LegacyEntityType: "leaf"})

	e, err := s.Reader().GetLiveEntity(context.Background(), "l1")
	require.NoError(t, err)
	assert.False(t, e.Locked)
	assert.True(t, e.Discrete)
}

func TestEntityCreated_LegacyContainerPayload_IsUnlockedNonDiscrete(t *testing.T) {
	// Old events with entity_type="container" map to the zero value: locked=false, discrete=false.
	s := openTestStore(t)
	b := eventbus.New(s)

	// LegacyEntityType marshals as the old entity_type field.
	dispatch(
		t,
		s,
		b,
		eventbus.EntityCreatedPayload{EntityID: "c1", DisplayName: "Toolbox", LegacyEntityType: "container"},
	)

	e, err := s.Reader().GetLiveEntity(context.Background(), "c1")
	require.NoError(t, err)
	assert.False(t, e.Locked)
	assert.False(t, e.Discrete)
}

func TestEntityCreated_NewPayload_LockedAndDiscrete(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)

	createEntity(t, s, b, "e1", "Box of Nails", nil, false, true)

	e, err := s.Reader().GetLiveEntity(context.Background(), "e1")
	require.NoError(t, err)
	assert.False(t, e.Locked)
	assert.True(t, e.Discrete)
}

func TestEntityCreated_NestedUnderEntity_OK(t *testing.T) {
	// The bus handler enforces no parent-type restrictions.
	// Discrete-parent enforcement is at the app layer — see TestCreateEntity_DiscreteParent_Error.
	s := openTestStore(t)
	b := eventbus.New(s)

	createEntity(t, s, b, "p1", "Garage", nil, true, false)
	p1ID := "p1"
	createEntity(t, s, b, "c1", "Toolbox", &p1ID, false, false)

	e, err := s.Reader().GetLiveEntity(context.Background(), "c1")
	require.NoError(t, err)
	assert.Equal(t, "p1", *e.ParentID)
}
