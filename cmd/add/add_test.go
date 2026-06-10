package add_test

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/add"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

func TestRunAdd_HappyPath(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	// Pre-create parents so add can resolve the parent path
	for _, tc := range []struct {
		name   string
		parent string
	}{
		{"Garage", ""},
		{"Toolbox", "Garage"},
	} {
		_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: tc.name,
			ParentPath:  tc.parent,
			ActorID:     "test",
		})
		require.NoError(t, err)
	}

	cmd := add.NewAddCmd(a)
	cmd.SetArgs([]string{"Garage:Toolbox:Wrench"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	require.NoError(t, cmd.Execute())

	entities, err := a.ListEntities(ctx)
	require.NoError(t, err)

	var found bool
	for _, e := range entities {
		if e.FullPathDisplay == "Garage:Toolbox:Wrench" {
			found = true
		}
	}
	require.True(t, found, "Wrench entity should exist after add")
}

func TestRunAdd_LockedFlag(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	cmd := add.NewAddCmd(a)
	cmd.SetArgs([]string{"Garage", "--locked"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	result, err := a.GetEntityByPath(ctx, "Garage")
	require.NoError(t, err)
	assert.True(t, result.Locked)
	assert.False(t, result.Discrete)
}

func TestRunAdd_DiscreteFlag(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	cmd := add.NewAddCmd(a)
	cmd.SetArgs([]string{"Box of Nails", "--discrete"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	result, err := a.GetEntityByPath(ctx, "Box of Nails")
	require.NoError(t, err)
	assert.False(t, result.Locked)
	assert.True(t, result.Discrete)
}

func TestRunAdd_PropagatesAppError(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := add.NewAddCmd(a)
	// Path with only a separator is invalid
	cmd.SetArgs([]string{":"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
}

func TestRunAdd_Quiet_SuppressesSuccess(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := add.NewAddCmd(a)
	cmd.SetContext(
		context.WithValue(t.Context(), config.ConfigKey, apptesting.NewTestConfig(t, apptesting.WithQuiet())),
	)
	cmd.SetArgs([]string{"Garage"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	require.NoError(t, cmd.Execute())
	assert.Empty(t, stdout.String())
	assert.Empty(t, stderr.String())
}

func TestRunAdd_File_MutualExclusion_WithPositionalArgs(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := add.NewAddCmd(a)
	cmd.SetArgs([]string{"--file", "items.csv", "Garage"})
	require.Error(t, cmd.Execute())
}

func TestRunAdd_File_MutualExclusion_WithLockedFlag(t *testing.T) {
	a := apptesting.OpenApp(t)
	f := writeTempCSV(t, "Garage,false,false,\n")
	cmd := add.NewAddCmd(a)
	cmd.SetArgs([]string{"--file", f, "--locked"})
	require.Error(t, cmd.Execute())
}

func TestRunAdd_File_MutualExclusion_WithDiscreteFlag(t *testing.T) {
	a := apptesting.OpenApp(t)
	f := writeTempCSV(t, "Garage,false,false,\n")
	cmd := add.NewAddCmd(a)
	cmd.SetArgs([]string{"--file", f, "--discrete"})
	require.Error(t, cmd.Execute())
}

func TestRunAdd_File_NoArgs_NoFile_Error(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := add.NewAddCmd(a)
	cmd.SetArgs([]string{})
	require.Error(t, cmd.Execute())
}

func TestRunAdd_File_HappyPath_SummaryOutput(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	// Pre-create parents so we don't need --create-parents.
	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Garage", ActorID: "test"})
	require.NoError(t, err)

	f := writeTempCSV(t, "Garage:Wrench,false,true,tool\n")
	cmd := add.NewAddCmd(a)
	cmd.SetArgs([]string{"--file", f})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	require.NoError(t, cmd.Execute())

	assert.Contains(t, stdout.String(), "Created 1 entity")
	assert.Empty(t, stderr.String())
}

func TestRunAdd_File_Verbose_PerEntityLines(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Garage", ActorID: "test"})
	require.NoError(t, err)

	f := writeTempCSV(t, "Garage:Wrench,false,false,\nGarage:Hammer,false,false,\n")
	cmd := add.NewAddCmd(a)
	cmd.SetArgs([]string{"--file", f, "--verbose"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	require.NoError(t, cmd.Execute())

	out := stdout.String()
	assert.Contains(t, out, "Garage:Wrench")
	assert.Contains(t, out, "Garage:Hammer")
}

func TestRunAdd_File_Quiet_NoOutput(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{DisplayName: "Garage", ActorID: "test"})
	require.NoError(t, err)

	f := writeTempCSV(t, "Garage:Wrench,false,false,\n")
	cmd := add.NewAddCmd(a)
	cmd.SetContext(
		context.WithValue(t.Context(), config.ConfigKey, apptesting.NewTestConfig(t, apptesting.WithQuiet())),
	)
	cmd.SetArgs([]string{"--file", f})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	require.NoError(t, cmd.Execute())

	assert.Empty(t, stdout.String())
	assert.Empty(t, stderr.String())
}

func TestRunAdd_File_JSON_StructuredOutput(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	cmd := add.NewAddCmd(a)
	cmd.SetContext(
		context.WithValue(t.Context(), config.ConfigKey, apptesting.NewTestConfig(t, apptesting.WithJSON())),
	)

	f := writeTempCSV(t, "Garage,false,false,\n")
	cmd.SetArgs([]string{"--file", f})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	require.NoError(t, cmd.Execute())

	_ = ctx // used above
	out := stdout.String()
	assert.Contains(t, out, `"created"`)
	assert.Contains(t, out, `"skipped"`)
	assert.Contains(t, out, `"warnings"`)
}

func TestRunAdd_File_CreateParents_CreatesAncestors(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	f := writeTempCSV(t, "Garage:Toolbox:Wrench,false,false,\n")
	cmd := add.NewAddCmd(a)
	cmd.SetArgs([]string{"--file", f, "--create-parents"})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	require.NoError(t, cmd.Execute())

	entities, err := a.ListEntities(ctx)
	require.NoError(t, err)
	paths := make(map[string]bool)
	for _, e := range entities {
		paths[e.FullPathDisplay] = true
	}
	assert.True(t, paths["Garage"])
	assert.True(t, paths["Garage:Toolbox"])
	assert.True(t, paths["Garage:Toolbox:Wrench"])
}

// writeTempCSV writes content to a temp file and returns the path.
func writeTempCSV(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "bulk*.csv")
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}
