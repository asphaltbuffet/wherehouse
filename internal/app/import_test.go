package app_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
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
	assert.Equal(t, 0, result.WarningCount)
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
	assert.Equal(t, 0, result.WarningCount)
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
	assert.Equal(t, 1, result.WarningCount)
	require.Len(t, result.Warnings, 1, "len(Warnings) must equal WarningCount")
	assert.Contains(t, result.Warnings[0].Error(), "orphaned")
	assert.Contains(t, result.Warnings[0].Error(), "event_id 1")
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
		app.CreateEntityRequest{DisplayName: "GP1", ActorID: "alice"},
	)
	require.NoError(t, err)

	gp2, err := a.CreateEntity(
		ctx,
		app.CreateEntityRequest{DisplayName: "GP2", ActorID: "alice"},
	)
	require.NoError(t, err)

	parent, err := a.CreateEntity(
		ctx,
		app.CreateEntityRequest{
			DisplayName: "Parent", ActorID: "alice",
			ParentID: gp1.EntityID,
		},
	)
	require.NoError(t, err)

	_, err = a.CreateEntity(
		ctx,
		app.CreateEntityRequest{
			DisplayName: "Child", ActorID: "alice",
			ParentID: parent.EntityID,
		},
	)
	require.NoError(t, err)

	_, err = a.ReparentEntity(
		ctx,
		app.ReparentEntityRequest{
			EntityID:    parent.EntityID,
			NewParentID: gp2.EntityID,
			ActorID:     "alice",
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
	assert.Equal(t, 0, result.WarningCount, "matching path-changed records should not produce warnings")
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
	assert.Equal(t, 1, result.WarningCount)
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
	assert.Equal(t, 1, result.WarningCount)
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

func TestImport_RoundTripPreservesHomes(t *testing.T) {
	for _, returnHome := range []bool{false, true} {
		name := "displaced"
		if returnHome {
			name = "returned"
		}
		t.Run(name, func(t *testing.T) {
			src := apptesting.OpenApp(t)
			ctx := t.Context()
			garage, err := src.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Garage", ActorID: "t"})
			require.NoError(t, err)
			deck, err := src.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Deck", ActorID: "t"})
			require.NoError(t, err)
			drill, err := src.CreateEntity(ctx, app.CreateEntityRequest{
				DisplayName: "Drill", ParentID: garage.EntityID, ActorID: "t",
			})
			require.NoError(t, err)
			_, err = src.CreateEntity(ctx, app.CreateEntityRequest{
				DisplayName: "Bit", ParentID: drill.EntityID, ActorID: "t",
			})
			require.NoError(t, err)
			_, err = src.UseEntity(ctx, app.UseEntityRequest{
				EntityID: drill.EntityID, NewParentID: deck.EntityID, ActorID: "t",
			})
			require.NoError(t, err)
			wantPath := "Deck:Drill"
			if returnHome {
				_, err = src.MarkReturned(ctx, []app.ChangeStatusRequest{{EntityID: drill.EntityID, ActorID: "t"}})
				require.NoError(t, err)
				wantPath = "Garage:Drill"
			}

			events, err := src.GetAllEvents(ctx)
			require.NoError(t, err)
			dst := apptesting.OpenApp(t)
			res, err := dst.ImportEvents(ctx, events, app.ImportOptions{})
			require.NoError(t, err)
			assert.Zero(t, res.WarningCount, "path_changed events for descendants must validate: %v", res.Warnings)
			assert.Zero(t, res.Failed)

			got, err := dst.GetEntityByID(ctx, drill.EntityID)
			require.NoError(t, err)
			assert.Equal(t, wantPath, got.FullPathDisplay)
			assert.Equal(t, garage.EntityID, got.HomeID)
			assert.Equal(t, returnHome, !got.IsDisplaced())
		})
	}
}

// Regression for #357: a rebuild of an imported log must leave descendants at
// their post-change paths, must not grow the log, and must match the source log.
func TestImport_RebuildKeepsDescendantPaths(t *testing.T) {
	tests := []struct {
		name   string
		change func(t *testing.T, a *app.App, b, d app.EntityResult)
		want   string
	}{
		{
			name: "move",
			change: func(t *testing.T, a *app.App, b, d app.EntityResult) {
				t.Helper()
				_, err := a.ReparentEntity(t.Context(), app.ReparentEntityRequest{
					EntityID: b.EntityID, NewParentID: d.EntityID, ActorID: "t",
				})
				require.NoError(t, err)
			},
			want: "D:B:C",
		},
		{
			name: "rename",
			change: func(t *testing.T, a *app.App, b, _ app.EntityResult) {
				t.Helper()
				_, err := a.RenameEntity(t.Context(), app.RenameEntityRequest{
					EntityID: b.EntityID, NewName: "Bee", ActorID: "t",
				})
				require.NoError(t, err)
			},
			want: "A:Bee:C",
		},
		{
			name: "case-only rename",
			change: func(t *testing.T, a *app.App, b, _ app.EntityResult) {
				t.Helper()
				_, err := a.RenameEntity(t.Context(), app.RenameEntityRequest{
					EntityID: b.EntityID, NewName: "b", ActorID: "t",
				})
				require.NoError(t, err)
			},
			want: "A:b:C",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			src := apptesting.OpenApp(t)
			a, err := src.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "A", ActorID: "t"})
			require.NoError(t, err)
			b, err := src.CreateEntity(
				ctx,
				app.CreateEntityRequest{DisplayName: "B", ParentID: a.EntityID, ActorID: "t"},
			)
			require.NoError(t, err)
			c, err := src.CreateEntity(
				ctx,
				app.CreateEntityRequest{DisplayName: "C", ParentID: b.EntityID, ActorID: "t"},
			)
			require.NoError(t, err)
			d, err := src.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "D", ActorID: "t"})
			require.NoError(t, err)
			tt.change(t, src, b, d)

			srcEvents, err := src.GetAllEvents(ctx)
			require.NoError(t, err)

			dst := apptesting.OpenApp(t)
			res, err := dst.ImportEvents(ctx, srcEvents, app.ImportOptions{})
			require.NoError(t, err)
			assert.Zero(t, res.WarningCount, "warnings: %v", res.Warnings)

			dstEvents, err := dst.GetAllEvents(ctx)
			require.NoError(t, err)
			require.Len(t, dstEvents, len(srcEvents), "imported log must match the source log")
			for i := range srcEvents {
				assert.Equal(t, srcEvents[i].EventType, dstEvents[i].EventType, "event %d type", i)
				assert.JSONEq(t, string(srcEvents[i].Payload), string(dstEvents[i].Payload), "event %d payload", i)
			}

			_, err = dst.TruncateAndReplay(ctx)
			require.NoError(t, err)

			got, err := dst.GetEntityByID(ctx, c.EntityID)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got.FullPathDisplay)

			rebuilt, err := dst.GetAllEvents(ctx)
			require.NoError(t, err)
			assert.Len(t, rebuilt, len(srcEvents), "rebuild must not write events")
		})
	}
}
