package history_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/history"
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

func TestHistory_ShowsEvents(t *testing.T) {
	db, ctx := openTestDB(t)

	id := nanoid.MustNew()
	_, err := db.AppendEvent(ctx, database.EntityCreatedEvent, "testuser", map[string]any{
		"entity_id":    id,
		"display_name": "hammer",
		"entity_type":  "leaf",
		"parent_id":    nil,
	}, "")
	require.NoError(t, err)

	cmd := history.NewHistoryCmd(db)
	cmd.SetContext(ctx)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{id})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "entity.created")
}

func TestHistory_UnknownEntity_EmptyOutput(t *testing.T) {
	db, ctx := openTestDB(t)

	cmd := history.NewHistoryCmd(db)
	cmd.SetContext(ctx)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"doesnotexist"})

	require.NoError(t, cmd.Execute())
	assert.Empty(t, out.String())
}
