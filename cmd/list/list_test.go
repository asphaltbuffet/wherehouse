package list_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/list"
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

func appendEntity(t *testing.T, db *database.Database, id, name, entityType string) {
	t.Helper()
	ctx := context.Background()
	_, err := db.AppendEvent(ctx, database.EntityCreatedEvent, "testuser", map[string]any{
		"entity_id":    id,
		"display_name": name,
		"entity_type":  entityType,
		"parent_id":    nil,
	}, "")
	require.NoError(t, err)
}

func TestList_All(t *testing.T) {
	db, ctx := openTestDB(t)

	appendEntity(t, db, nanoid.MustNew(), "Garage", "place")
	appendEntity(t, db, nanoid.MustNew(), "Toolbox", "container")

	cmd := list.NewListCmd(db)
	cmd.SetContext(ctx)
	var out bytes.Buffer
	cmd.SetOut(&out)

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "Garage")
	assert.Contains(t, out.String(), "Toolbox")
}

func TestList_FilterByType(t *testing.T) {
	db, ctx := openTestDB(t)

	appendEntity(t, db, nanoid.MustNew(), "Garage", "place")
	appendEntity(t, db, nanoid.MustNew(), "Toolbox", "container")

	cmd := list.NewListCmd(db)
	cmd.SetContext(ctx)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--type", "place"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "Garage")
	assert.NotContains(t, out.String(), "Toolbox")
}

func TestList_FilterByStatus(t *testing.T) {
	db, ctx := openTestDB(t)

	hammerID := nanoid.MustNew()
	appendEntity(t, db, hammerID, "hammer", "leaf")

	// Mark hammer as borrowed.
	_, err := db.AppendEvent(ctx, database.EntityStatusChangedEvent, "testuser", map[string]any{
		"entity_id":      hammerID,
		"status":         "borrowed",
		"status_context": nil,
	}, "")
	require.NoError(t, err)

	appendEntity(t, db, nanoid.MustNew(), "wrench", "leaf")

	cmd := list.NewListCmd(db)
	cmd.SetContext(ctx)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--status", "borrowed"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "hammer")
	assert.NotContains(t, out.String(), "wrench")
}
