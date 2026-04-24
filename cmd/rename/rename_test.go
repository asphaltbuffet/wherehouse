package rename_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/rename"
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

func TestRenameEntity_UpdatesPathAndDescendants(t *testing.T) {
	db, ctx := openTestDB(t)

	garageID := nanoid.MustNew()
	toolboxID := nanoid.MustNew()
	screwdriverID := nanoid.MustNew()

	appendEntity(t, db, garageID, "Garage", "place", nil)
	appendEntity(t, db, toolboxID, "Toolbox", "container", &garageID)
	appendEntity(t, db, screwdriverID, "screwdriver", "container", &toolboxID)

	cmd := rename.NewRenameCmd(db)
	cmd.SetContext(ctx)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{toolboxID, "--to", "Big Toolbox"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "Big Toolbox")

	toolbox, err := db.GetEntity(ctx, toolboxID)
	require.NoError(t, err)
	assert.Equal(t, "Garage::Big Toolbox", toolbox.FullPathDisplay)

	sd, err := db.GetEntity(ctx, screwdriverID)
	require.NoError(t, err)
	assert.Equal(t, "Garage::Big Toolbox::screwdriver", sd.FullPathDisplay)
}

func TestRenameEntity_NotFound_Errors(t *testing.T) {
	db, ctx := openTestDB(t)
	cmd := rename.NewRenameCmd(db)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"doesnotexist", "--to", "NewName"})
	err := cmd.Execute()
	require.Error(t, err)
}
