package scry_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/scry"
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

func TestScry_ByName_TwoMatches(t *testing.T) {
	db, ctx := openTestDB(t)

	id1 := nanoid.MustNew()
	id2 := nanoid.MustNew()
	appendEntity(t, db, id1, "screwdriver", "leaf")
	appendEntity(t, db, id2, "screwdriver", "leaf")

	cmd := scry.NewScryCmd(db)
	cmd.SetContext(ctx)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"screwdriver"})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), id1)
	assert.Contains(t, out.String(), id2)
}

func TestScry_NoArgs_ListsAll(t *testing.T) {
	db, ctx := openTestDB(t)

	appendEntity(t, db, nanoid.MustNew(), "hammer", "leaf")
	appendEntity(t, db, nanoid.MustNew(), "wrench", "leaf")

	cmd := scry.NewScryCmd(db)
	cmd.SetContext(ctx)
	var out bytes.Buffer
	cmd.SetOut(&out)

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "hammer")
	assert.Contains(t, out.String(), "wrench")
}

func TestScry_Nonexistent_EmptyOutput(t *testing.T) {
	db, ctx := openTestDB(t)

	cmd := scry.NewScryCmd(db)
	cmd.SetContext(ctx)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"doesnotexist"})

	require.NoError(t, cmd.Execute())
	assert.Empty(t, out.String())
}
