package listcmd_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	listcmd "github.com/asphaltbuffet/wherehouse/cmd/list"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// seedThree creates Garage (place) → Garage:Toolbox (container) → Garage:Toolbox:Wrench (leaf).
func seedThree(t *testing.T, a *app.App) {
	t.Helper()
	ctx := t.Context()
	for _, tc := range []struct {
		name   string
		parent string
	}{
		{"Garage", ""},
		{"Toolbox", "Garage"},
		{"Wrench", "Garage:Toolbox"},
	} {
		_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: tc.name,
			ParentPath:  tc.parent,
			ActorID:     "test",
		})
		require.NoError(t, err)
	}
}

func runCmd(t *testing.T, a *app.App, args ...string) string {
	t.Helper()
	cmd := listcmd.NewListCmd(a)
	cmd.SetArgs(args)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())
	return stdout.String()
}

// TestRunList_RendersTree: default output uses leaf DisplayName only, not full path.
func TestRunList_RendersTree(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedThree(t, a)

	out := runCmd(t, a)

	assert.Contains(t, out, "Garage")
	assert.Contains(t, out, "Toolbox")
	assert.Contains(t, out, "Wrench")
	// Full colon-paths must not appear in tree output.
	assert.NotContains(t, out, "Garage:Toolbox")
	assert.NotContains(t, out, "Garage:Toolbox:Wrench")
}

// TestRunList_EmptyDB_ReturnsNoOutput: empty inventory produces no output.
func TestRunList_EmptyDB_ReturnsNoOutput(t *testing.T) {
	a := apptesting.OpenApp(t)
	assert.Empty(t, runCmd(t, a))
}

// TestRunList_ShowsStatusBadge: non-ok status appears as [STATUS], ok is silent.
func TestRunList_ShowsStatusBadge(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	seedThree(t, a)
	wrench, err := a.LookupEntityByPath(ctx, "Garage:Toolbox:Wrench")
	require.NoError(t, err)
	_, err = a.ChangeStatus(ctx, app.ChangeStatusRequest{
		EntityID: wrench.EntityID,
		Status:   inventory.EntityStatusMissing,
		ActorID:  "test",
	})
	require.NoError(t, err)

	out := runCmd(t, a)

	assert.Contains(t, out, "[MISSING]")
	assert.NotContains(t, out, "[OK]")
}

// TestRunList_FilterPrunesUnrelatedBranches: filtering removes branches with no matches.
func TestRunList_FilterPrunesUnrelatedBranches(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	// Garage:Toolbox:Wrench (leaf, ok) + Garage:Toolbox:Drill (leaf, missing)
	seedThree(t, a)
	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Drill",
		ParentPath:  "Garage:Toolbox",
		ActorID:     "test",
	})
	require.NoError(t, err)
	drill, err := a.LookupEntityByPath(ctx, "Garage:Toolbox:Drill")
	require.NoError(t, err)
	_, err = a.ChangeStatus(ctx, app.ChangeStatusRequest{
		EntityID: drill.EntityID,
		Status:   inventory.EntityStatusMissing,
		ActorID:  "test",
	})
	require.NoError(t, err)

	out := runCmd(t, a, "--status", "missing")

	assert.Contains(t, out, "Drill")
	assert.NotContains(t, out, "Wrench")
}

// TestRunList_FilterKeepsAncestor: ancestor of a match is shown even if it doesn't match.
func TestRunList_FilterKeepsAncestor(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	seedThree(t, a)
	wrench, err := a.LookupEntityByPath(ctx, "Garage:Toolbox:Wrench")
	require.NoError(t, err)
	_, err = a.ChangeStatus(ctx, app.ChangeStatusRequest{
		EntityID: wrench.EntityID,
		Status:   inventory.EntityStatusMissing,
		ActorID:  "test",
	})
	require.NoError(t, err)

	out := runCmd(t, a, "--status", "missing")

	// Wrench matched; Garage and Toolbox are ancestors and must appear as scaffolding.
	assert.Contains(t, out, "Wrench")
	assert.Contains(t, out, "Toolbox")
	assert.Contains(t, out, "Garage")
}

// TestRunList_UnderIncludesRoot: --under X includes X itself in output (not just its descendants).
func TestRunList_UnderIncludesRoot(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedThree(t, a)

	out := runCmd(t, a, "--under", "Garage:Toolbox")

	// Toolbox is the scoped root — it must appear (not just its children).
	assert.Contains(t, out, "Toolbox")
	assert.Contains(t, out, "Wrench")
}

// TestRunList_VerboseShowsIDAndTags: --verbose prepends entity ID and shows tags.
func TestRunList_VerboseShowsIDAndTags(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	seedThree(t, a)
	wrench, err := a.LookupEntityByPath(ctx, "Garage:Toolbox:Wrench")
	require.NoError(t, err)
	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityID: wrench.EntityID,
		ActorID:  "test",
		Add:      []string{"dewalt"},
	}))

	out := runCmd(t, a, "--verbose")

	assert.Contains(t, out, "#dewalt")
}

// TestRunList_NoTagsWithoutVerbose: tags are absent from non-verbose output.
func TestRunList_NoTagsWithoutVerbose(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	seedThree(t, a)
	wrench, err := a.LookupEntityByPath(ctx, "Garage:Toolbox:Wrench")
	require.NoError(t, err)
	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityID: wrench.EntityID,
		ActorID:  "test",
		Add:      []string{"dewalt"},
	}))

	out := runCmd(t, a)

	assert.NotContains(t, out, "#dewalt")
}

// TestRunList_JSONIncludesTags: --json output includes tags field.
func TestRunList_JSONIncludesTags(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	seedThree(t, a)
	wrench, err := a.LookupEntityByPath(ctx, "Garage:Toolbox:Wrench")
	require.NoError(t, err)
	require.NoError(t, a.TagEntity(ctx, app.TagEntityRequest{
		EntityID: wrench.EntityID,
		ActorID:  "test",
		Add:      []string{"dewalt"},
	}))

	cmd := listcmd.NewListCmd(a)
	cmd.SetContext(context.WithValue(ctx, config.ConfigKey, apptesting.NewTestConfig(t, apptesting.WithJSON())))
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	var items []map[string]any
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &items))

	var wrenchItem map[string]any
	for _, item := range items {
		if item["path"] == "Garage:Toolbox:Wrench" {
			wrenchItem = item
			break
		}
	}
	require.NotNil(t, wrenchItem, "Wrench not found in JSON output")

	tags, ok := wrenchItem["tags"].([]any)
	require.True(t, ok, "tags field must be an array")
	require.Len(t, tags, 1)
	assert.Equal(t, "dewalt", tags[0])
}
