package remove_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/remove"
	"github.com/asphaltbuffet/wherehouse/internal/database"
	"github.com/asphaltbuffet/wherehouse/internal/nanoid"
)

func openTestDB(t *testing.T) (*database.Database, context.Context) {
	t.Helper()
	ctx := context.Background()
	db, err := database.Open(database.Config{
		Path:        ":memory:",
		BusyTimeout: database.DefaultBusyTimeout,
		AutoMigrate: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db, ctx
}

func appendEntity(t *testing.T, db *database.Database, id, name, entityType string, parentID *string) {
	t.Helper()
	ctx := context.Background()
	_, err := db.AppendEvent(ctx, database.EntityCreatedEvent, "testuser", map[string]any{
		"entity_id":    id,
		"display_name": name,
		"entity_type":  entityType,
		"parent_id":    parentID,
	}, "")
	require.NoError(t, err)
}

func TestRemoveEntity_Success(t *testing.T) {
	db, ctx := openTestDB(t)

	id := nanoid.MustNew()
	appendEntity(t, db, id, "hammer", "leaf", nil)

	cmd := remove.NewRemoveCmd(db)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{id})
	require.NoError(t, cmd.Execute())

	e, err := db.GetEntity(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, database.EntityStatusRemoved, e.Status)
}

func TestRemoveEntity_WithNonRemovedChildren_Errors(t *testing.T) {
	db, ctx := openTestDB(t)

	parentID := nanoid.MustNew()
	childID := nanoid.MustNew()

	appendEntity(t, db, parentID, "Toolbox", "container", nil)
	appendEntity(t, db, childID, "screwdriver", "leaf", &parentID)

	cmd := remove.NewRemoveCmd(db)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{parentID})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestRemoveEntity_UnknownID_Errors(t *testing.T) {
	db, ctx := openTestDB(t)

	cmd := remove.NewRemoveCmd(db)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"doesnotexist"})
	err := cmd.Execute()
	require.Error(t, err)
}
