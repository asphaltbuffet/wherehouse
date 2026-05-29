package app_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func createdRecord(id int64, entityID, name string) app.ExportResult {
	payload, _ := json.Marshal(map[string]string{
		"entity_id":    entityID,
		"display_name": name,
		"entity_type":  "place",
	})
	return app.ExportResult{
		EventID:      id,
		EventType:    inventory.EntityCreatedEvent.String(),
		TimestampUTC: "2020-01-01T00:00:00Z",
		ActorUserID:  "alice",
		EntityID:     &entityID,
		Payload:      payload,
	}
}

// --- slice 1: non-monotonic event_id → error, no replay ---

func TestImportEvents_NonMonotonicOrder_ReturnsError(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	events := []app.ExportResult{
		createdRecord(1, "e1", "Garage"),
		createdRecord(3, "e2", "Shelf"),
		createdRecord(2, "e3", "Box"), // out of order
	}

	_, err := a.ImportEvents(ctx, events, app.ImportOptions{})
	require.Error(t, err)

	// Confirm nothing was written.
	results, err2 := a.GetAllEvents(ctx)
	require.NoError(t, err2)
	assert.Empty(t, results, "no events should be replayed when order is invalid")
}

func TestImportEvents_EmptyInput_ReturnsZeroResult(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	result, err := a.ImportEvents(ctx, nil, app.ImportOptions{})
	require.NoError(t, err)
	assert.Equal(t, app.ImportResult{}, result)
}

func TestImportEvents_SingleEvent_ReplayedCountIsOne(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	events := []app.ExportResult{createdRecord(1, "e1", "Garage")}

	result, err := a.ImportEvents(ctx, events, app.ImportOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Replayed)
	assert.Equal(t, 0, result.Failed)
	assert.Equal(t, 0, result.Warnings)
}

func TestImportEvents_PathChangedEventNotCountedAsReplayed(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	entityID := "e1"
	pcPayload, _ := json.Marshal(map[string]any{
		"entity_id":           entityID,
		"full_path_display":   "Garage",
		"full_path_canonical": "garage",
		"depth":               0,
	})

	events := []app.ExportResult{
		createdRecord(1, entityID, "Garage"),
		{
			EventID:      2,
			EventType:    inventory.EntityPathChangedEvent.String(),
			TimestampUTC: "2020-01-01T00:00:00Z",
			ActorUserID:  "alice",
			EntityID:     &entityID,
			Payload:      pcPayload,
		},
	}

	result, err := a.ImportEvents(ctx, events, app.ImportOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Replayed, "path-changed event should not increment Replayed")
}

func TestImportEvents_NoReparentEvents_WarningsIsZero(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	events := []app.ExportResult{
		createdRecord(1, "e1", "Shelf"),
		createdRecord(2, "e2", "Box"),
	}

	result, err := a.ImportEvents(ctx, events, app.ImportOptions{})
	require.NoError(t, err)
	assert.Equal(t, 0, result.Warnings)
}

func TestImportEvents_OrphanedPathChanged_WarningIsOne(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	entityID := "e1"
	pcPayload, _ := json.Marshal(map[string]any{
		"entity_id":           entityID,
		"full_path_display":   "Garage",
		"full_path_canonical": "garage",
		"depth":               0,
	})

	// path-changed with no preceding reparent → orphan
	events := []app.ExportResult{
		{
			EventID:      1,
			EventType:    inventory.EntityPathChangedEvent.String(),
			TimestampUTC: "2020-01-01T00:00:00Z",
			ActorUserID:  "alice",
			EntityID:     &entityID,
			Payload:      pcPayload,
		},
		createdRecord(2, "e2", "Box"),
	}

	result, err := a.ImportEvents(ctx, events, app.ImportOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Warnings)
}

func TestImportEvents_Continue_AccumulatesFailedAndDoesNotAbort(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	// Middle event renames an entity that was never created — passes Level 2
	// validation (known type, valid JSON object payload) but fails at replay
	// time inside applyEventTx. --continue must accumulate this as a per-event
	// failure rather than aborting.
	ghostID := "ghost"
	renamePayload, _ := json.Marshal(map[string]string{
		"entity_id":    ghostID,
		"display_name": "WillFail",
	})
	events := []app.ExportResult{
		createdRecord(1, "e1", "Garage"),
		{
			EventID:      2,
			EventType:    inventory.EntityRenamedEvent.String(),
			TimestampUTC: "2020-01-01T00:00:00Z",
			ActorUserID:  "alice",
			Payload:      renamePayload,
			EntityID:     &ghostID,
		},
		createdRecord(3, "e2", "Shelf"),
	}

	result, err := a.ImportEvents(ctx, events, app.ImportOptions{Continue: true})
	require.NoError(t, err)
	assert.Equal(t, 2, result.Replayed)
	assert.Equal(t, 1, result.Failed)
	assert.Len(t, result.Errors, 1)
}

// seedReparentScenario creates a 3-level hierarchy (grandparent → parent → child),
// reparents the parent under a new grandparent, and returns the export stream.
// The export will contain a reparent event followed by a path-changed event for the child.
func seedReparentScenario(t *testing.T) (*app.App, []app.ExportResult) {
	t.Helper()
	a := openTestApp(t)
	ctx := context.Background()

	gp1, err := a.CreateEntity(
		ctx,
		app.CreateEntityRequest{DisplayName: "GP1", EntityType: inventory.EntityTypePlace, ActorID: "alice"},
	)
	require.NoError(t, err)

	gp2, err := a.CreateEntity(
		ctx,
		app.CreateEntityRequest{DisplayName: "GP2", EntityType: inventory.EntityTypePlace, ActorID: "alice"},
	)
	require.NoError(t, err)

	parent, err := a.CreateEntity(
		ctx,
		app.CreateEntityRequest{
			DisplayName: "Parent",
			EntityType:  inventory.EntityTypeContainer,
			ActorID:     "alice",
			ParentPath:  gp1.FullPathDisplay,
		},
	)
	require.NoError(t, err)

	_, err = a.CreateEntity(
		ctx,
		app.CreateEntityRequest{
			DisplayName: "Child",
			EntityType:  inventory.EntityTypeContainer,
			ActorID:     "alice",
			ParentPath:  parent.FullPathDisplay,
		},
	)
	require.NoError(t, err)

	_, err = a.ReparentEntity(
		ctx,
		app.ReparentEntityRequest{
			EntityPath:    parent.FullPathDisplay,
			NewParentPath: gp2.FullPathDisplay,
			ActorID:       "alice",
		},
	)
	require.NoError(t, err)

	events, err := a.GetAllEvents(ctx)
	require.NoError(t, err)
	return a, events
}

func TestImportEvents_ReparentWithMatchingPathChanged_WarningsIsZero(t *testing.T) {
	_, exportedEvents := seedReparentScenario(t)

	dest := openTestApp(t)
	ctx := context.Background()

	result, err := dest.ImportEvents(ctx, exportedEvents, app.ImportOptions{})
	require.NoError(t, err)
	assert.Equal(t, 0, result.Warnings, "matching path-changed records should not produce warnings")
}

func TestImportEvents_ReparentWithMismatchedPathChangedPayload_WarningIsOne(t *testing.T) {
	_, exportedEvents := seedReparentScenario(t)

	// Corrupt the path-changed payload in the export stream.
	for i, ev := range exportedEvents {
		if ev.EventType == inventory.EntityPathChangedEvent.String() {
			exportedEvents[i].Payload = json.RawMessage(
				`{"entity_id":"corrupted","full_path_display":"WRONG","full_path_canonical":"wrong","depth":0}`,
			)
			break
		}
	}

	dest := openTestApp(t)
	ctx := context.Background()

	result, err := dest.ImportEvents(ctx, exportedEvents, app.ImportOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Warnings)
}

func TestImportEvents_ReparentWithTooFewPathChangedRecords_WarningIsOne(t *testing.T) {
	_, exportedEvents := seedReparentScenario(t)

	// Remove all path-changed records from the export stream.
	filtered := exportedEvents[:0]
	for _, ev := range exportedEvents {
		if ev.EventType != inventory.EntityPathChangedEvent.String() {
			filtered = append(filtered, ev)
		}
	}

	dest := openTestApp(t)
	ctx := context.Background()

	result, err := dest.ImportEvents(ctx, filtered, app.ImportOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Warnings)
}

func TestImportEvents_ReplaceTrue_ClearsExistingEventsBeforeReplay(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	first, err := a.ImportEvents(ctx, []app.ExportResult{createdRecord(1, "old", "OldPlace")}, app.ImportOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, first.Replayed)

	second, err := a.ImportEvents(
		ctx,
		[]app.ExportResult{createdRecord(2, "new", "NewPlace")},
		app.ImportOptions{Replace: true},
	)
	require.NoError(t, err)
	require.Equal(t, 1, second.Replayed)

	has, err := a.HasEvents(ctx)
	require.NoError(t, err)
	assert.True(t, has, "second import should leave its own event in place")
}

func TestImportEvents_ReplaceTrue_MalformedRecordLeavesDBUntouched(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	first, err := a.ImportEvents(ctx, []app.ExportResult{createdRecord(1, "keep", "KeepMe")}, app.ImportOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, first.Replayed)

	bad := app.ExportResult{
		EventID:      2,
		EventType:    "entity.created",
		TimestampUTC: "2020-01-02T00:00:00Z",
		ActorUserID:  "alice",
		Payload:      json.RawMessage(`{`),
	}
	_, err = a.ImportEvents(ctx, []app.ExportResult{bad}, app.ImportOptions{Replace: true})
	require.Error(t, err)
	require.Contains(t, err.Error(), "event_id 2")

	has, err := a.HasEvents(ctx)
	require.NoError(t, err)
	assert.True(t, has, "Replace must not clear the database when pre-validation fails")
}

func TestImportEvents_UnknownEventType_RejectedBeforeReplay(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	bad := app.ExportResult{
		EventID:      1,
		EventType:    "entity.exploded",
		TimestampUTC: "2020-01-01T00:00:00Z",
		ActorUserID:  "alice",
		Payload:      json.RawMessage(`{}`),
	}
	_, err := a.ImportEvents(ctx, []app.ExportResult{bad}, app.ImportOptions{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "validation failed")
	require.Contains(t, err.Error(), "event_id 1")
}
