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
	p := eventbus.EntityCreatedPayload{
		EntityID: id, DisplayName: name, EntityType: "place", ParentID: parentID,
	}
	raw, err := json.Marshal(p)
	require.NoError(t, err)
	_, err = b.Dispatch(context.Background(), inventory.EntityCreatedEvent, "test", raw, nil)
	require.NoError(t, err)
}

func createLeaf(t *testing.T, b *eventbus.Bus, id, name string, parentID *string) {
	t.Helper()
	p := eventbus.EntityCreatedPayload{
		EntityID: id, DisplayName: name, EntityType: "leaf", ParentID: parentID,
	}
	raw, err := json.Marshal(p)
	require.NoError(t, err)
	_, err = b.Dispatch(context.Background(), inventory.EntityCreatedEvent, "test", raw, nil)
	require.NoError(t, err)
}

func TestValidatePlaceParent_PlaceInPlace_OK(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)

	createPlace(t, b, "p1", "Garage", nil)

	p1ID := "p1"
	p := eventbus.EntityCreatedPayload{
		EntityID: "p2", DisplayName: "Zone", EntityType: "place", ParentID: &p1ID,
	}
	raw, err := json.Marshal(p)
	require.NoError(t, err)
	_, err = b.Dispatch(context.Background(), inventory.EntityCreatedEvent, "test", raw, nil)
	assert.NoError(t, err)
}

func TestValidatePlaceParent_PlaceInLeaf_Error(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)

	createPlace(t, b, "p1", "Garage", nil)
	p1ID := "p1"
	createLeaf(t, b, "l1", "Wrench", &p1ID)

	leafID := "l1"
	p := eventbus.EntityCreatedPayload{
		EntityID: "p3", DisplayName: "Zone", EntityType: "place", ParentID: &leafID,
	}
	raw, err := json.Marshal(p)
	require.NoError(t, err)
	_, err = b.Dispatch(context.Background(), inventory.EntityCreatedEvent, "test", raw, nil)
	assert.Error(t, err)
}
