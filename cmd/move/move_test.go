package move_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/move"
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

func TestMoveEntity_Success(t *testing.T) {
	db, ctx := openTestDB(t)

	garageID := nanoid.MustNew()
	workshopID := nanoid.MustNew()
	toolboxID := nanoid.MustNew()

	appendEntity(t, db, garageID, "Garage", "place", nil)
	appendEntity(t, db, workshopID, "Workshop", "place", nil)
	appendEntity(t, db, toolboxID, "Toolbox", "container", &garageID)

	cmd := move.NewMoveCmd(db)
	cmd.SetContext(ctx)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{toolboxID, "--to", workshopID})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "Workshop::Toolbox")

	toolbox, err := db.GetEntity(ctx, toolboxID)
	require.NoError(t, err)
	assert.Equal(t, "Workshop::Toolbox", toolbox.FullPathDisplay)
}

func TestMoveEntity_PlaceRejected(t *testing.T) {
	db, ctx := openTestDB(t)

	garageID := nanoid.MustNew()
	workshopID := nanoid.MustNew()

	appendEntity(t, db, garageID, "Garage", "place", nil)
	appendEntity(t, db, workshopID, "Workshop", "place", nil)

	cmd := move.NewMoveCmd(db)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{garageID, "--to", workshopID})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "place entities cannot be moved")
}

func TestMoveEntity_UnknownEntity_Errors(t *testing.T) {
	db, ctx := openTestDB(t)

	workshopID := nanoid.MustNew()
	appendEntity(t, db, workshopID, "Workshop", "place", nil)

	cmd := move.NewMoveCmd(db)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"doesnotexist", "--to", workshopID})

	err := cmd.Execute()
	require.Error(t, err)
}

func TestMoveEntity_UnknownDestination_Errors(t *testing.T) {
	db, ctx := openTestDB(t)

	toolboxID := nanoid.MustNew()
	appendEntity(t, db, toolboxID, "Toolbox", "container", nil)

	cmd := move.NewMoveCmd(db)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{toolboxID, "--to", "doesnotexist"})

	err := cmd.Execute()
	require.Error(t, err)
}
