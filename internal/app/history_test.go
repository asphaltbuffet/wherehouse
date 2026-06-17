package app_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func TestGetHistory_ByPath(t *testing.T) {
	a := openTestApp(t)
	ctx := context.Background()

	garage, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage", ActorID: "alice",
	})
	require.NoError(t, err)

	history, err := a.GetHistory(ctx, app.GetHistoryRequest{EntityID: garage.EntityID})
	require.NoError(t, err)
	assert.Len(t, history, 1)
	assert.Equal(t, inventory.EntityCreatedEvent, history[0].EventType)
}
