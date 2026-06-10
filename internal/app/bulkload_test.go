package app_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
)

func TestParseCSV_HappyPath(t *testing.T) {
	input := strings.NewReader("Garage:Toolbox:Screwdriver,false,true,tool craftsman\n")
	rows, err := app.ParseBulkCSV(input, false)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "Garage:Toolbox:Screwdriver", rows[0].Path)
	assert.False(t, rows[0].Locked)
	assert.True(t, rows[0].Discrete)
	assert.Equal(t, []string{"tool", "craftsman"}, rows[0].Tags)
}

func TestParseCSV_CommentsSkipped(t *testing.T) {
	input := strings.NewReader("# this is a comment\nGarage,false,false,\n# another comment\n")
	rows, err := app.ParseBulkCSV(input, false)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "Garage", rows[0].Path)
}

func TestParseCSV_TrailingColumnsOptional(t *testing.T) {
	cases := []struct {
		name     string
		line     string
		wantLock bool
		wantDisc bool
		wantTags []string
	}{
		{"path only", "Garage", false, false, nil},
		{"path+locked", "Garage,true", true, false, nil},
		{"path+locked+discrete", "Garage,false,true", false, true, nil},
		{"empty middle discrete", "Garage,,true", false, true, nil},
		{"empty tags field", "Garage,false,false,", false, false, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := app.ParseBulkCSV(strings.NewReader(tc.line+"\n"), false)
			require.NoError(t, err)
			require.Len(t, rows, 1)
			assert.Equal(t, tc.wantLock, rows[0].Locked)
			assert.Equal(t, tc.wantDisc, rows[0].Discrete)
			assert.Equal(t, tc.wantTags, rows[0].Tags)
		})
	}
}

func TestParseCSV_InvalidBooleanHardError(t *testing.T) {
	input := strings.NewReader("Garage:Toolbox,yes,false,\n")
	_, err := app.ParseBulkCSV(input, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "line 1")
	assert.Contains(t, err.Error(), `"locked"`)
	assert.Contains(t, err.Error(), `"yes"`)
}

func TestParseCSV_WithinFileDedup_FirstWins(t *testing.T) {
	input := strings.NewReader("Garage,false,false,first\nGarage,true,true,second\n")
	rows, err := app.ParseBulkCSV(input, false)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.False(t, rows[0].Locked, "first-wins: locked should be false")
	assert.Equal(t, []string{"first"}, rows[0].Tags)
}

func TestBulkAddEntities_HappyPath(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	rows := []app.BulkAddRow{
		{Path: "Garage", Locked: false, Discrete: false},
		{Path: "Garage:Toolbox", Locked: false, Discrete: false},
		{Path: "Garage:Toolbox:Screwdriver", Locked: false, Discrete: true, Tags: []string{"tool", "craftsman"}},
	}

	result, err := a.BulkAddEntities(ctx, rows, app.BulkAddOptions{ActorID: "test"})
	require.NoError(t, err)
	assert.Len(t, result.Created, 3)
	assert.Empty(t, result.Skipped)

	// Verify entity and tags exist in DB.
	entities, err := a.ListEntities(ctx)
	require.NoError(t, err)
	var screwdriver *app.EntityResult
	for i := range entities {
		if entities[i].FullPathDisplay == "Garage:Toolbox:Screwdriver" {
			screwdriver = &entities[i]
		}
	}
	require.NotNil(t, screwdriver, "Screwdriver entity should exist")
	assert.True(t, screwdriver.Discrete)
	assert.ElementsMatch(t, []string{"tool", "craftsman"}, screwdriver.Tags)
}

func TestBulkAddEntities_DBDuplicate_SkipWithWarning(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	// Pre-create Garage.
	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Garage", ActorID: "test"})
	require.NoError(t, err)

	rows := []app.BulkAddRow{{Path: "Garage"}}
	result, err := a.BulkAddEntities(ctx, rows, app.BulkAddOptions{ActorID: "test"})
	require.NoError(t, err)
	assert.Empty(t, result.Created)
	require.Len(t, result.Skipped, 1)
	assert.Equal(t, "Garage", result.Skipped[0].Path)
	assert.NotEmpty(t, result.Warnings)
}

func TestBulkAddEntities_AllowDuplicates_CreatesSecond(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Garage", ActorID: "test"})
	require.NoError(t, err)

	rows := []app.BulkAddRow{{Path: "Garage"}}
	result, err := a.BulkAddEntities(ctx, rows, app.BulkAddOptions{ActorID: "test", AllowDuplicates: true})
	require.NoError(t, err)
	assert.Len(t, result.Created, 1)
	assert.Empty(t, result.Skipped)
}

func TestBulkAddEntities_CreateParents_MissingAncestors(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	rows := []app.BulkAddRow{
		{Path: "Garage:Toolbox:Screwdriver", Discrete: true, Tags: []string{"tool"}},
	}
	result, err := a.BulkAddEntities(ctx, rows, app.BulkAddOptions{ActorID: "test", CreateParents: true})
	require.NoError(t, err)
	assert.Len(t, result.Created, 3) // Garage + Toolbox + Screwdriver

	entities, err := a.ListEntities(ctx)
	require.NoError(t, err)
	paths := make(map[string]bool)
	for _, e := range entities {
		paths[e.FullPathDisplay] = true
	}
	assert.True(t, paths["Garage"])
	assert.True(t, paths["Garage:Toolbox"])
	assert.True(t, paths["Garage:Toolbox:Screwdriver"])
}

func TestBulkAddEntities_CreateParents_ExistingParentWarns(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Garage", ActorID: "test"})
	require.NoError(t, err)

	rows := []app.BulkAddRow{{Path: "Garage:Wrench"}}
	result, err := a.BulkAddEntities(ctx, rows, app.BulkAddOptions{ActorID: "test", CreateParents: true})
	require.NoError(t, err)
	assert.Len(t, result.Created, 1)
	assert.NotEmpty(t, result.Warnings, "should warn that Garage already exists")
}

func TestParseCSV_WithinFileDedup_AllowDuplicates_BothRowsPass(t *testing.T) {
	input := strings.NewReader("Widget,false,false,\nWidget,false,false,\n")
	rows, err := app.ParseBulkCSV(input, true)
	require.NoError(t, err)
	assert.Len(t, rows, 2, "allowDuplicates=true should pass both rows through")
}

func TestBulkAddEntities_DiscreteParent_HardError(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Box", Discrete: true, ActorID: "test"})
	require.NoError(t, err)

	rows := []app.BulkAddRow{{Path: "Box:Nail"}}
	_, err = a.BulkAddEntities(ctx, rows, app.BulkAddOptions{ActorID: "test", CreateParents: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "discrete")
}

func TestBulkAddEntities_DiscreteDirectParent_HardError(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	// Garage exists and is not discrete; Box is discrete.
	// Adding Box:Nail should fail because Box (direct parent) is discrete.
	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Garage", ActorID: "test"})
	require.NoError(t, err)
	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Box", ParentPath: "Garage", Discrete: true, ActorID: "test",
	})
	require.NoError(t, err)

	rows := []app.BulkAddRow{{Path: "Garage:Box:Nail"}}
	_, err = a.BulkAddEntities(ctx, rows, app.BulkAddOptions{ActorID: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "discrete")
}

func TestBulkAddEntities_MissingParent_HardErrorWithoutFlag(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	rows := []app.BulkAddRow{{Path: "Garage:Toolbox:Wrench"}}
	_, err := a.BulkAddEntities(ctx, rows, app.BulkAddOptions{ActorID: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--create-parents")
}
