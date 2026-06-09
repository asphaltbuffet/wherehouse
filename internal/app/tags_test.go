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

func seedForTags(t *testing.T, a *app.App) {
	t.Helper()
	ctx := context.Background()
	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage", ActorID: "alice",
	})
	require.NoError(t, err)
	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Wrench", ParentPath: "Garage", ActorID: "alice",
	})
	require.NoError(t, err)
}

func TestTagEntity_Add(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForTags(t, a)
	ctx := context.Background()

	err := a.TagEntity(ctx, app.TagEntityRequest{
		EntityPath: "Garage:Wrench",
		ActorID:    "alice",
		Add:        []string{"tool", "hand_tool"},
	})
	require.NoError(t, err)

	result, err := a.GetEntityByPath(ctx, "Garage:Wrench")
	require.NoError(t, err)
	assert.Equal(t, []string{"hand_tool", "tool"}, result.Tags)
}

func TestTagEntity_Remove(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForTags(t, a)
	ctx := context.Background()

	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityPath: "Garage:Wrench", ActorID: "alice", Add: []string{"tool", "hand_tool"},
	}))
	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityPath: "Garage:Wrench", ActorID: "alice", Remove: []string{"hand_tool"},
	}))

	result, err := a.GetEntityByPath(ctx, "Garage:Wrench")
	require.NoError(t, err)
	assert.Equal(t, []string{"tool"}, result.Tags)
}

func TestTagEntity_MixedAddRemove(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForTags(t, a)
	ctx := context.Background()

	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityPath: "Garage:Wrench", ActorID: "alice", Add: []string{"tool"},
	}))
	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityPath: "Garage:Wrench",
		ActorID:    "alice",
		Add:        []string{"hand_tool"},
		Remove:     []string{"tool"},
	}))

	result, err := a.GetEntityByPath(ctx, "Garage:Wrench")
	require.NoError(t, err)
	assert.Equal(t, []string{"hand_tool"}, result.Tags)
}

func TestTagEntity_OverlapCancels(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForTags(t, a)
	ctx := context.Background()

	// "tool" in both add and remove — should cancel; only "hand_tool" added
	err := a.TagEntity(ctx, app.TagEntityRequest{
		EntityPath: "Garage:Wrench",
		ActorID:    "alice",
		Add:        []string{"tool", "hand_tool"},
		Remove:     []string{"tool"},
	})
	require.NoError(t, err)

	result, err := a.GetEntityByPath(ctx, "Garage:Wrench")
	require.NoError(t, err)
	assert.Equal(t, []string{"hand_tool"}, result.Tags)
}

func TestTagEntity_AddDuplicate(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForTags(t, a)
	ctx := context.Background()

	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityPath: "Garage:Wrench", ActorID: "alice", Add: []string{"tool"},
	}))
	// Re-adding the same tag must be a no-op (no error).
	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityPath: "Garage:Wrench", ActorID: "alice", Add: []string{"tool"},
	}))

	result, err := a.GetEntityByPath(ctx, "Garage:Wrench")
	require.NoError(t, err)
	assert.Equal(t, []string{"tool"}, result.Tags)
}

func TestTagEntity_RemoveMissing(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForTags(t, a)
	ctx := context.Background()

	// Removing a tag that doesn't exist must be a no-op (no error).
	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityPath: "Garage:Wrench", ActorID: "alice", Remove: []string{"nonexistent"},
	}))
}

func TestTagEntity_UnknownPath(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := context.Background()

	err := a.TagEntity(ctx, app.TagEntityRequest{
		EntityPath: "Nope:DoesNotExist", ActorID: "alice", Add: []string{"tool"},
	})
	require.Error(t, err)
}

func TestListTags(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForTags(t, a)
	ctx := context.Background()

	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityPath: "Garage:Wrench", ActorID: "alice", Add: []string{"tool", "screwdriver"},
	}))

	tags, err := a.ListTags(ctx, app.ListTagsRequest{EntityPath: "Garage:Wrench"})
	require.NoError(t, err)
	assert.Equal(t, []string{"screwdriver", "tool"}, tags)
}

func TestListTags_UnknownPath(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := context.Background()

	_, err := a.ListTags(ctx, app.ListTagsRequest{EntityPath: "Nope:Missing"})
	require.Error(t, err)
}

func TestTagEntity_AppearsInHistory(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForTags(t, a)
	ctx := context.Background()

	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityPath: "Garage:Wrench", ActorID: "alice", Add: []string{"tool"},
	}))

	history, err := a.GetHistory(ctx, app.GetHistoryRequest{EntityPath: "Garage:Wrench"})
	require.NoError(t, err)

	var eventTypes []string
	for _, h := range history {
		eventTypes = append(eventTypes, h.EventType.String())
	}
	assert.Contains(t, eventTypes, inventory.EntityTagAddedEvent.String())
}
