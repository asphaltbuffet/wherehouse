package app_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
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
		DisplayName: "Wrench", ParentID: apptesting.MustLookupID(t, a, "Garage"), ActorID: "alice",
	})
	require.NoError(t, err)
}

func TestTagEntity_Add(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForTags(t, a)
	ctx := context.Background()

	wrench, err := a.LookupEntityByPath(ctx, "Garage:Wrench")
	require.NoError(t, err)
	err = a.TagEntity(ctx, app.TagEntityRequest{
		EntityID: wrench.EntityID,
		ActorID:  "alice",
		Add:      []string{"tool", "hand_tool"},
	})
	require.NoError(t, err)

	result, err := a.LookupEntityByPath(ctx, "Garage:Wrench")
	require.NoError(t, err)
	assert.Equal(t, []string{"hand_tool", "tool"}, result.Tags)
}

func TestTagEntity_Remove(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForTags(t, a)
	ctx := context.Background()

	wrench, err := a.LookupEntityByPath(ctx, "Garage:Wrench")
	require.NoError(t, err)
	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityID: wrench.EntityID, ActorID: "alice", Add: []string{"tool", "hand_tool"},
	}))
	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityID: wrench.EntityID, ActorID: "alice", Remove: []string{"hand_tool"},
	}))

	result, err := a.LookupEntityByPath(ctx, "Garage:Wrench")
	require.NoError(t, err)
	assert.Equal(t, []string{"tool"}, result.Tags)
}

func TestTagEntity_MixedAddRemove(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForTags(t, a)
	ctx := context.Background()

	wrench, err := a.LookupEntityByPath(ctx, "Garage:Wrench")
	require.NoError(t, err)
	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityID: wrench.EntityID, ActorID: "alice", Add: []string{"tool"},
	}))
	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityID: wrench.EntityID,
		ActorID:  "alice",
		Add:      []string{"hand_tool"},
		Remove:   []string{"tool"},
	}))

	result, err := a.LookupEntityByPath(ctx, "Garage:Wrench")
	require.NoError(t, err)
	assert.Equal(t, []string{"hand_tool"}, result.Tags)
}

func TestTagEntity_OverlapCancels(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForTags(t, a)
	ctx := context.Background()

	wrench, err := a.LookupEntityByPath(ctx, "Garage:Wrench")
	require.NoError(t, err)

	// "tool" in both add and remove — should cancel; only "hand_tool" added
	err = a.TagEntity(ctx, app.TagEntityRequest{
		EntityID: wrench.EntityID,
		ActorID:  "alice",
		Add:      []string{"tool", "hand_tool"},
		Remove:   []string{"tool"},
	})
	require.NoError(t, err)

	result, err := a.LookupEntityByPath(ctx, "Garage:Wrench")
	require.NoError(t, err)
	assert.Equal(t, []string{"hand_tool"}, result.Tags)
}

func TestTagEntity_AddDuplicate(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForTags(t, a)
	ctx := context.Background()

	wrench, err := a.LookupEntityByPath(ctx, "Garage:Wrench")
	require.NoError(t, err)
	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityID: wrench.EntityID, ActorID: "alice", Add: []string{"tool"},
	}))
	// Re-adding the same tag must be a no-op (no error).
	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityID: wrench.EntityID, ActorID: "alice", Add: []string{"tool"},
	}))

	result, err := a.LookupEntityByPath(ctx, "Garage:Wrench")
	require.NoError(t, err)
	assert.Equal(t, []string{"tool"}, result.Tags)
}

func TestTagEntity_RemoveMissing(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForTags(t, a)
	ctx := context.Background()

	wrench, err := a.LookupEntityByPath(ctx, "Garage:Wrench")
	require.NoError(t, err)

	// Removing a tag that doesn't exist must be a no-op (no error).
	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityID: wrench.EntityID, ActorID: "alice", Remove: []string{"nonexistent"},
	}))
}

func TestTagEntity_UnknownPath(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := context.Background()

	err := a.TagEntity(ctx, app.TagEntityRequest{
		EntityID: "nonexistent-id", ActorID: "alice", Add: []string{"tool"},
	})
	require.Error(t, err)
}

func TestListTags(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForTags(t, a)
	ctx := context.Background()

	wrench, err := a.LookupEntityByPath(ctx, "Garage:Wrench")
	require.NoError(t, err)
	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityID: wrench.EntityID, ActorID: "alice", Add: []string{"tool", "screwdriver"},
	}))

	tags, err := a.ListTags(ctx, app.ListTagsRequest{EntityID: wrench.EntityID})
	require.NoError(t, err)
	assert.Equal(t, []string{"screwdriver", "tool"}, tags)
}

func TestListTags_UnknownPath(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := context.Background()

	_, err := a.ListTags(ctx, app.ListTagsRequest{EntityID: "nonexistent-id"})
	require.Error(t, err)
}

func TestTagEntity_AppearsInHistory(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForTags(t, a)
	ctx := context.Background()

	wrench, err := a.LookupEntityByPath(ctx, "Garage:Wrench")
	require.NoError(t, err)
	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityID: wrench.EntityID, ActorID: "alice", Add: []string{"tool"},
	}))

	history, err := a.GetHistory(ctx, app.GetHistoryRequest{EntityID: wrench.EntityID})
	require.NoError(t, err)

	var eventTypes []string
	for _, h := range history {
		eventTypes = append(eventTypes, h.EventType.String())
	}
	assert.Contains(t, eventTypes, inventory.EntityTagAddedEvent.String())
}

func TestTagEntity_EmptyCanonicalAddRejected(t *testing.T) {
	tests := []struct {
		name string
		add  []string
	}{
		{name: "blank", add: []string{"   "}},
		{name: "separators only", add: []string{"--"}},
		{name: "valid alongside empty", add: []string{"tool", "_ _"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := apptesting.OpenApp(t)
			seedForTags(t, a)
			ctx := context.Background()

			wrench, err := a.LookupEntityByPath(ctx, "Garage:Wrench")
			require.NoError(t, err)
			err = a.TagEntity(ctx, app.TagEntityRequest{
				EntityID: wrench.EntityID, ActorID: "alice", Add: tt.add,
			})
			require.ErrorIs(t, err, app.ErrEmptyTag)

			tags, err := a.ListTags(ctx, app.ListTagsRequest{EntityID: wrench.EntityID})
			require.NoError(t, err)
			assert.Empty(t, tags, "a rejected call writes no tag")
		})
	}
}

func TestTagEntity_RemoveEmptyWritesNoEvent(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForTags(t, a)
	ctx := context.Background()

	wrench, err := a.LookupEntityByPath(ctx, "Garage:Wrench")
	require.NoError(t, err)
	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityID: wrench.EntityID, ActorID: "alice", Remove: []string{"--"},
	}))

	history, err := a.GetHistory(ctx, app.GetHistoryRequest{EntityID: wrench.EntityID})
	require.NoError(t, err)
	for _, h := range history {
		assert.NotEqual(t, inventory.EntityTagRemovedEvent, h.EventType)
	}
}

func TestAllTags(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForTags(t, a)
	ctx := context.Background()

	garage, err := a.LookupEntityByPath(ctx, "Garage")
	require.NoError(t, err)
	wrench, err := a.LookupEntityByPath(ctx, "Garage:Wrench")
	require.NoError(t, err)
	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityID: garage.EntityID, ActorID: "alice", Add: []string{"Place", "tool"},
	}))
	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityID: wrench.EntityID, ActorID: "alice", Add: []string{"tool"},
	}))

	tags, err := a.AllTags(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"place", "tool"}, tags)
}

// seedLegacyEmptyTag imports an entity carrying the "" tag that builds before
// #324 recorded for `tag -a "--"`, and returns its ID.
func seedLegacyEmptyTag(t *testing.T, a *app.App) string {
	t.Helper()
	ctx := context.Background()
	entityID := "e1"
	payload, err := json.Marshal(eventbus.EntityTagAddedPayload{EntityID: entityID, Tag: ""})
	require.NoError(t, err)
	_, err = a.ImportEvents(ctx, []app.ExportResult{
		createdRecord(1, entityID, "Garage"),
		{
			EventID: 2, EventType: inventory.EntityTagAddedEvent.String(),
			TimestampUTC: "2020-01-01T00:00:00Z", ActorUserID: "alice",
			EntityID: &entityID, Payload: payload,
		},
	}, app.ImportOptions{})
	require.NoError(t, err)

	tags, err := a.ListTags(ctx, app.ListTagsRequest{EntityID: entityID})
	require.NoError(t, err)
	require.Equal(t, []string{""}, tags)
	return entityID
}

func TestTagEntity_RemoveLegacyEmptyTag(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := context.Background()
	entityID := seedLegacyEmptyTag(t, a)

	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityID: entityID, ActorID: "alice", Remove: []string{"--"},
	}))

	tags, err := a.ListTags(ctx, app.ListTagsRequest{EntityID: entityID})
	require.NoError(t, err)
	assert.Empty(t, tags, "a recorded empty tag can still be removed")
}

func TestAllTags_ExcludesLegacyEmptyTag(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedLegacyEmptyTag(t, a)

	tags, err := a.AllTags(context.Background())
	require.NoError(t, err)
	assert.Empty(t, tags, "the empty tag is never suggested")
}

// tagEventCount counts the tag add/remove events in entityID's history.
func tagEventCount(t *testing.T, a *app.App, entityID string) int {
	t.Helper()
	history, err := a.GetHistory(context.Background(), app.GetHistoryRequest{EntityID: entityID})
	require.NoError(t, err)
	n := 0
	for _, h := range history {
		if h.EventType == inventory.EntityTagAddedEvent || h.EventType == inventory.EntityTagRemovedEvent {
			n++
		}
	}
	return n
}

func TestTagEntity_NoOpsWriteNoEvents(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForTags(t, a)
	ctx := context.Background()

	wrench, err := a.LookupEntityByPath(ctx, "Garage:Wrench")
	require.NoError(t, err)
	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityID: wrench.EntityID, ActorID: "alice", Add: []string{"tool"},
	}))
	require.Equal(t, 1, tagEventCount(t, a, wrench.EntityID))

	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityID: wrench.EntityID, ActorID: "alice", Add: []string{"Tool"}, Remove: []string{"absent"},
	}))

	assert.Equal(t, 1, tagEventCount(t, a, wrench.EntityID),
		"a duplicate add and an absent remove append nothing to the log")
}
