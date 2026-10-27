package apptesting

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// RemoveLeavingLiveChildren marks entityID removed without the live-children
// guard, reproducing data written before #287: live entities under a removed
// parent. It appends an entity.removed event and rebuilds the projections by
// replaying the whole log.
func RemoveLeavingLiveChildren(t *testing.T, a *app.App, entityID string) {
	t.Helper()
	ctx := t.Context()

	events, err := a.GetAllEvents(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, events, "seed the entity before removing it")

	payload, err := json.Marshal(eventbus.EntityRemovedPayload{EntityID: entityID})
	require.NoError(t, err)
	last := events[len(events)-1]
	events = append(events, app.ExportResult{
		EventID:      last.EventID + 1,
		EventType:    inventory.EntityRemovedEvent.String(),
		TimestampUTC: last.TimestampUTC,
		ActorUserID:  "test",
		EntityID:     &entityID,
		Payload:      payload,
	})

	res, err := a.ImportEvents(ctx, events, app.ImportOptions{Replace: true})
	require.NoError(t, err)
	require.Empty(t, res.Errors)
}
