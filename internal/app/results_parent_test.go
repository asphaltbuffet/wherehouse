package app_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
)

func TestEntityResult_ParentID(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	box, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Box", ActorID: "test"})
	require.NoError(t, err)
	item, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Item", ParentPath: "Box", ActorID: "test",
	})
	require.NoError(t, err)

	gotBox, err := a.GetEntityByID(ctx, box.EntityID)
	require.NoError(t, err)
	assert.Empty(t, gotBox.ParentID, "root entity has no parent")

	gotItem, err := a.GetEntityByID(ctx, item.EntityID)
	require.NoError(t, err)
	assert.Equal(t, box.EntityID, gotItem.ParentID)
}
