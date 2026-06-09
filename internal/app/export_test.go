package app_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
)

func TestGetAllEvents_TracesBullet(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Box", ActorID: "alice",
	})
	require.NoError(t, err)

	results, err := a.GetAllEvents(ctx)
	require.NoError(t, err)
	require.Len(t, results, 1)

	r := results[0]
	assert.Equal(t, "entity.created", r.EventType)
	assert.NotNil(t, r.Payload)
	assert.Positive(t, r.EventID)
}

func TestGetAllEvents_PayloadRoundTrips(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Shelf", ActorID: "bob",
	})
	require.NoError(t, err)

	results, err := a.GetAllEvents(ctx)
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Payload must marshal as a nested JSON object, not a base64 string.
	out, err := json.Marshal(results[0])
	require.NoError(t, err)

	var envelope map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(out, &envelope))

	// payload value must start with '{', not '"' (which would indicate base64).
	payloadBytes := envelope["payload"]
	require.NotEmpty(t, payloadBytes)
	assert.Equal(t, byte('{'), payloadBytes[0], "payload should be a JSON object, not a base64 string")
}

func TestGetAllEvents_NilFieldsPreserved(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Bin", ActorID: "carol",
	})
	require.NoError(t, err)

	results, err := a.GetAllEvents(ctx)
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.Nil(t, results[0].Note, "Note should be nil when not set")
	assert.NotNil(t, results[0].EntityID, "EntityID should be non-nil for entity.created events")
}

func TestGetAllEvents_OrderPreserved(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Alpha", ActorID: "dave",
	})
	require.NoError(t, err)

	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Beta", ActorID: "dave",
	})
	require.NoError(t, err)

	results, err := a.GetAllEvents(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(results), 2)

	for i := 1; i < len(results); i++ {
		assert.Less(t, results[i-1].EventID, results[i].EventID, "events should be ordered by event_id ASC")
	}
}
