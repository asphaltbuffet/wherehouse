package tag_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/tag"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func seedForTagCmd(t *testing.T, a *app.App) {
	t.Helper()
	ctx := context.Background()
	_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Garage", EntityType: inventory.EntityTypePlace, ActorID: "alice",
	})
	require.NoError(t, err)
	_, err = a.CreateEntity(ctx, app.CreateEntityRequest{
		DisplayName: "Wrench", EntityType: inventory.EntityTypeLeaf, ParentPath: "Garage", ActorID: "alice",
	})
	require.NoError(t, err)
}

func runTagCmd(t *testing.T, a *app.App, args ...string) (string, string, error) {
	t.Helper()
	outBuf, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := tag.NewTagCmd(a)
	cmd.SetOut(outBuf)
	cmd.SetErr(errBuf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

func TestTagCmd_NoFlags_ListsEmptyTags(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForTagCmd(t, a)

	stdout, _, err := runTagCmd(t, a, "Garage:Wrench")
	require.NoError(t, err)
	assert.Empty(t, strings.TrimSpace(stdout))
}

func TestTagCmd_Add(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForTagCmd(t, a)

	_, _, err := runTagCmd(t, a, "Garage:Wrench", "--add", "tool", "--add", "hand_tool")
	require.NoError(t, err)

	tags, err := a.ListTags(context.Background(), app.ListTagsRequest{EntityPath: "Garage:Wrench"})
	require.NoError(t, err)
	assert.Equal(t, []string{"hand_tool", "tool"}, tags)
}

func TestTagCmd_Remove(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForTagCmd(t, a)

	require.NoError(t, a.TagEntity(context.Background(), app.TagEntityRequest{
		EntityPath: "Garage:Wrench", ActorID: "alice", Add: []string{"tool", "hand_tool"},
	}))

	_, _, err := runTagCmd(t, a, "Garage:Wrench", "--remove", "hand_tool")
	require.NoError(t, err)

	tags, err := a.ListTags(context.Background(), app.ListTagsRequest{EntityPath: "Garage:Wrench"})
	require.NoError(t, err)
	assert.Equal(t, []string{"tool"}, tags)
}

func TestTagCmd_MixedFlags(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForTagCmd(t, a)

	require.NoError(t, a.TagEntity(context.Background(), app.TagEntityRequest{
		EntityPath: "Garage:Wrench", ActorID: "alice", Add: []string{"tool"},
	}))

	_, _, err := runTagCmd(t, a, "Garage:Wrench", "--add", "hand_tool", "--remove", "tool")
	require.NoError(t, err)

	tags, err := a.ListTags(context.Background(), app.ListTagsRequest{EntityPath: "Garage:Wrench"})
	require.NoError(t, err)
	assert.Equal(t, []string{"hand_tool"}, tags)
}

func TestTagCmd_JSON_List(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForTagCmd(t, a)

	require.NoError(t, a.TagEntity(context.Background(), app.TagEntityRequest{
		EntityPath: "Garage:Wrench", ActorID: "alice", Add: []string{"tool"},
	}))

	stdout, _, err := runTagCmd(t, a, "Garage:Wrench", "--json")
	require.NoError(t, err)

	var out app.TagOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, "Garage:Wrench", out.Path)
	assert.Equal(t, []string{"tool"}, out.Tags)
}

func TestTagCmd_JSON_Mutation(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedForTagCmd(t, a)

	stdout, _, err := runTagCmd(t, a, "Garage:Wrench", "--add", "tool", "--json")
	require.NoError(t, err)

	var out app.TagOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, "Garage:Wrench", out.Path)
	assert.Equal(t, []string{"tool"}, out.Tags)
}

func TestTagCmd_UnknownEntity(t *testing.T) {
	a := apptesting.OpenApp(t)

	_, _, err := runTagCmd(t, a, "Nope:DoesNotExist", "--add", "tool")
	require.Error(t, err)
}
