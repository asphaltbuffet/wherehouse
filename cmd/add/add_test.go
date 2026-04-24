package add_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/add"
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

func TestAddEntity_TopLevel(t *testing.T) {
	db, ctx := openTestDB(t)
	cmd := add.NewAddCmd(db)
	cmd.SetContext(ctx)

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"Garage", "--type", "place"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Garage")
}

func TestAddEntity_DefaultTypeIsContainer(t *testing.T) {
	db, ctx := openTestDB(t)
	cmd := add.NewAddCmd(db)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"Toolbox"})

	err := cmd.Execute()
	require.NoError(t, err)

	results, err := db.GetEntitiesByCanonicalName(ctx, "toolbox")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, database.EntityTypeContainer, results[0].EntityType)
}

func TestAddEntity_NestedUnderParentByID(t *testing.T) {
	db, ctx := openTestDB(t)

	// Create parent first.
	parentCmd := add.NewAddCmd(db)
	parentCmd.SetContext(ctx)
	parentCmd.SetArgs([]string{"Garage", "--type", "place"})
	require.NoError(t, parentCmd.Execute())

	// Look up parent ID.
	parents, err := db.GetEntitiesByCanonicalName(ctx, "garage")
	require.NoError(t, err)
	require.Len(t, parents, 1)
	parentID := parents[0].EntityID

	// Add child using parent ID.
	childCmd := add.NewAddCmd(db)
	childCmd.SetContext(ctx)
	var out bytes.Buffer
	childCmd.SetOut(&out)
	childCmd.SetArgs([]string{"Toolbox", "--in", parentID})

	require.NoError(t, childCmd.Execute())
	assert.Contains(t, out.String(), "Garage::Toolbox")
}

func TestAddEntity_NestedUnderParentByName(t *testing.T) {
	db, ctx := openTestDB(t)

	parentCmd := add.NewAddCmd(db)
	parentCmd.SetContext(ctx)
	parentCmd.SetArgs([]string{"Workshop", "--type", "place"})
	require.NoError(t, parentCmd.Execute())

	childCmd := add.NewAddCmd(db)
	childCmd.SetContext(ctx)
	var out bytes.Buffer
	childCmd.SetOut(&out)
	childCmd.SetArgs([]string{"Shelf", "--in", "Workshop"})

	require.NoError(t, childCmd.Execute())
	assert.Contains(t, out.String(), "Workshop::Shelf")
}

func TestAddEntity_AmbiguousParentName_Errors(t *testing.T) {
	db, ctx := openTestDB(t)

	// Create two entities with the same name.
	for range 2 {
		cmd := add.NewAddCmd(db)
		cmd.SetContext(ctx)
		cmd.SetArgs([]string{"Shelf", "--type", "place"})
		require.NoError(t, cmd.Execute())
	}

	// Try to add under ambiguous name.
	cmd := add.NewAddCmd(db)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"box", "--in", "Shelf"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ambiguous")
}

func TestAddEntity_UnknownParent_Errors(t *testing.T) {
	db, ctx := openTestDB(t)
	cmd := add.NewAddCmd(db)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"thing", "--in", "doesnotexist"})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestAddEntity_InvalidType_Errors(t *testing.T) {
	db, ctx := openTestDB(t)
	cmd := add.NewAddCmd(db)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"thing", "--type", "bogus"})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestAddEntity_DBBootstrapsOnFirstRun(t *testing.T) {
	db, ctx := openTestDB(t)
	cmd := add.NewAddCmd(db)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"screwdriver"})
	require.NoError(t, cmd.Execute())

	results, err := db.GetEntitiesByCanonicalName(ctx, "screwdriver")
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Len(t, results[0].EntityID, nanoid.IDLength)
}
