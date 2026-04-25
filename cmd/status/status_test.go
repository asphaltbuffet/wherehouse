package status_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/status"
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

func TestStatusCommand_Loaned(t *testing.T) {
	db, ctx := openTestDB(t)

	id := nanoid.MustNew()
	appendEntity(t, db, id, "hammer", "container")

	cmd := status.NewStatusCmd(db)
	cmd.SetContext(ctx)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{id, "--set", "loaned", "--note", "loaned to Alice"})

	require.NoError(t, cmd.Execute())

	e, err := db.GetEntity(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, database.EntityStatusLoaned, e.Status)
	require.NotNil(t, e.StatusContext)
	assert.Equal(t, "loaned to Alice", *e.StatusContext)
}

func TestStatusCommand_ReturnToOk(t *testing.T) {
	db, ctx := openTestDB(t)

	id := nanoid.MustNew()
	appendEntity(t, db, id, "hammer", "container")

	cmd1 := status.NewStatusCmd(db)
	cmd1.SetContext(ctx)
	cmd1.SetArgs([]string{id, "--set", "loaned", "--note", "loaned to Bob"})
	require.NoError(t, cmd1.Execute())

	cmd2 := status.NewStatusCmd(db)
	cmd2.SetContext(ctx)
	cmd2.SetArgs([]string{id, "--set", "ok"})
	require.NoError(t, cmd2.Execute())

	e, err := db.GetEntity(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, database.EntityStatusOk, e.Status)
	assert.Nil(t, e.StatusContext)
}

func TestStatusCommand_InvalidStatus_Errors(t *testing.T) {
	db, ctx := openTestDB(t)

	id := nanoid.MustNew()
	appendEntity(t, db, id, "hammer", "container")

	cmd := status.NewStatusCmd(db)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{id, "--set", "broken"})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestStatusCommand_UnknownEntity_Errors(t *testing.T) {
	db, ctx := openTestDB(t)

	cmd := status.NewStatusCmd(db)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"doesnotexist", "--set", "missing"})
	err := cmd.Execute()
	require.Error(t, err)
}
