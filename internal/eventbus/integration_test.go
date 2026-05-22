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

func TestEntityLifecycle_CreateAndRename(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)
	ctx := context.Background()

	createPlace(t, b, "p1", "Garage", nil)

	entity, err := s.GetEntity(ctx, "p1")
	require.NoError(t, err)
	assert.Equal(t, "Garage", entity.DisplayName)
	assert.Equal(t, "garage", entity.CanonicalName)
	assert.Equal(t, "Garage", entity.FullPathDisplay)

	renamePayload, err := json.Marshal(eventbus.EntityRenamedPayload{EntityID: "p1", DisplayName: "Workshop"})
	require.NoError(t, err)
	_, err = b.Dispatch(ctx, inventory.EntityRenamedEvent, "alice", renamePayload, nil)
	require.NoError(t, err)

	renamed, err := s.GetEntity(ctx, "p1")
	require.NoError(t, err)
	assert.Equal(t, "Workshop", renamed.DisplayName)
	assert.Equal(t, "workshop", renamed.CanonicalName)
}

func TestEntityLifecycle_PathPropagation(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)
	ctx := context.Background()

	createPlace(t, b, "p1", "Garage", nil)
	p1ID := "p1"
	createPlace(t, b, "p2", "Toolbox", &p1ID)
	p2ID := "p2"
	createPlace(t, b, "p3", "Socket Set", &p2ID)

	renamePayload, err := json.Marshal(eventbus.EntityRenamedPayload{EntityID: "p1", DisplayName: "Workshop"})
	require.NoError(t, err)
	_, err = b.Dispatch(ctx, inventory.EntityRenamedEvent, "alice", renamePayload, nil)
	require.NoError(t, err)

	toolbox, err := s.GetEntity(ctx, "p2")
	require.NoError(t, err)
	assert.Equal(t, "Workshop:Toolbox", toolbox.FullPathDisplay)

	socketSet, err := s.GetEntity(ctx, "p3")
	require.NoError(t, err)
	assert.Equal(t, "Workshop:Toolbox:Socket Set", socketSet.FullPathDisplay)
}

func TestEntityLifecycle_Reparent(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)
	ctx := context.Background()

	// Garage > Toolbox > Wrench
	createPlace(t, b, "p1", "Garage", nil)
	p1ID := "p1"
	createPlace(t, b, "p2", "Toolbox", &p1ID)
	p2ID := "p2"
	createLeaf(t, b, "l1", "Wrench", &p2ID)

	// Create a second top-level place to reparent into.
	createPlace(t, b, "p3", "Workshop", nil)

	// Reparent Toolbox from Garage to Workshop.
	p3ID := "p3"
	reparentPayload, err := json.Marshal(eventbus.EntityReparentedPayload{
		EntityID:    "p2",
		NewParentID: &p3ID,
	})
	require.NoError(t, err)
	_, err = b.Dispatch(ctx, inventory.EntityReparentedEvent, "alice", reparentPayload, nil)
	require.NoError(t, err)

	toolbox, err := s.GetEntity(ctx, "p2")
	require.NoError(t, err)
	assert.Equal(t, "Workshop:Toolbox", toolbox.FullPathDisplay)
	assert.Equal(t, "workshop:toolbox", toolbox.FullPathCanonical)
	assert.Equal(t, 1, toolbox.Depth)

	wrench, err := s.GetEntity(ctx, "l1")
	require.NoError(t, err)
	assert.Equal(t, "Workshop:Toolbox:Wrench", wrench.FullPathDisplay)
	assert.Equal(t, "workshop:toolbox:wrench", wrench.FullPathCanonical)
	assert.Equal(t, 2, wrench.Depth)
}

func TestEntityLifecycle_StatusChange(t *testing.T) {
	s := openTestStore(t)
	b := eventbus.New(s)
	ctx := context.Background()

	createPlace(t, b, "p1", "Garage", nil)
	p1ID := "p1"
	createLeaf(t, b, "l1", "Wrench", &p1ID)

	note := "left at job site"
	statusPayload, err := json.Marshal(eventbus.EntityStatusChangedPayload{
		EntityID:      "l1",
		Status:        "missing",
		StatusContext: &note,
	})
	require.NoError(t, err)
	_, err = b.Dispatch(ctx, inventory.EntityStatusChangedEvent, "alice", statusPayload, nil)
	require.NoError(t, err)

	entity, err := s.GetEntity(ctx, "l1")
	require.NoError(t, err)
	assert.Equal(t, inventory.EntityStatusMissing, entity.Status)
	require.NotNil(t, entity.StatusContext)
	assert.Equal(t, "left at job site", *entity.StatusContext)
}
