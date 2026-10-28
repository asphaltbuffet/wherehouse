package attr_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/attr"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

func seed(t *testing.T, a *app.App) {
	t.Helper()
	for _, tc := range []struct{ name, parent string }{
		{"Garage", ""}, {"Shelf", "Garage"}, {"Nails", "Garage"},
	} {
		_, err := a.CreateEntity(t.Context(), app.CreateEntityRequest{
			DisplayName: tc.name, ParentID: apptesting.MustLookupID(t, a, tc.parent), ActorID: "test",
		})
		require.NoError(t, err)
	}
}

func run(t *testing.T, a *app.App, args ...string) (string, error) {
	t.Helper()
	cmd := attr.NewAttrCmd(a)
	cmd.SetArgs(args)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	return out.String(), err
}

func runWithConfig(t *testing.T, a *app.App, cfg *config.Config, args ...string) (string, error) {
	t.Helper()
	cmd := attr.NewAttrCmd(a)
	cmd.SetContext(context.WithValue(t.Context(), config.ConfigKey, cfg))
	cmd.SetArgs(args)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	return out.String(), err
}

func entity(t *testing.T, a *app.App, path string) app.EntityResult {
	t.Helper()
	e, err := a.LookupEntityByPath(t.Context(), path)
	require.NoError(t, err)
	return e
}

func TestAttr_LockMultiple(t *testing.T) {
	a := apptesting.OpenApp(t)
	seed(t, a)

	out, err := run(t, a, "Garage:Shelf", "Garage:Nails", "--locked")
	require.NoError(t, err)
	assert.True(t, entity(t, a, "Garage:Shelf").Locked)
	assert.True(t, entity(t, a, "Garage:Nails").Locked)
	assert.Contains(t, out, `"Garage:Shelf" locked`)
	assert.Contains(t, out, `"Garage:Nails" locked`)
}

func TestAttr_ClearWithEqualsFalse_AndCombined(t *testing.T) {
	a := apptesting.OpenApp(t)
	seed(t, a)
	_, err := run(t, a, "Garage:Nails", "--locked")
	require.NoError(t, err)

	out, err := run(t, a, "Garage:Nails", "--discrete", "--locked=false", "-n", "single unit")
	require.NoError(t, err)
	e := entity(t, a, "Garage:Nails")
	assert.False(t, e.Locked)
	assert.True(t, e.Discrete)
	assert.Contains(t, out, `"Garage:Nails" marked discrete, unlocked`)

	hist, err := a.GetHistory(t.Context(), app.GetHistoryRequest{EntityID: e.EntityID})
	require.NoError(t, err)
	assert.Equal(t, "single unit", hist[0].Note)
}

func TestAttr_ReadMode(t *testing.T) {
	a := apptesting.OpenApp(t)
	seed(t, a)
	_, err := run(t, a, "Garage:Shelf", "--locked")
	require.NoError(t, err)
	before, err := a.GetHistory(t.Context(), app.GetHistoryRequest{EntityID: entity(t, a, "Garage:Shelf").EntityID})
	require.NoError(t, err)

	out, err := run(t, a, "Garage:Shelf", "Garage")
	require.NoError(t, err)
	assert.Equal(t,
		"Garage:Shelf  locked=true  discrete=false\n"+
			"Garage        locked=false  discrete=false\n", out)

	after, err := a.GetHistory(t.Context(), app.GetHistoryRequest{EntityID: entity(t, a, "Garage:Shelf").EntityID})
	require.NoError(t, err)
	assert.Len(t, after, len(before), "read mode writes no events")
}

func TestAttr_NoteWithoutAttributeFlag(t *testing.T) {
	a := apptesting.OpenApp(t)
	seed(t, a)
	_, err := run(t, a, "Garage", "-n", "why")
	assert.ErrorContains(t, err, "--note requires --locked or --discrete")
}

func TestAttr_JSONSameShapeBothModes(t *testing.T) {
	a := apptesting.OpenApp(t)
	seed(t, a)
	cfg := config.GetDefaults()
	cfg.Output.DefaultFormat = config.OutputFormatJSON

	for _, args := range [][]string{
		{"Garage:Shelf", "--locked"}, // write
		{"Garage:Shelf"},             // read
	} {
		out, err := runWithConfig(t, a, cfg, args...)
		require.NoError(t, err)
		var got []app.AttrOutput
		require.NoError(t, json.Unmarshal([]byte(out), &got), out)
		require.Len(t, got, 1)
		assert.Equal(t, "Garage:Shelf", got[0].Path)
		assert.True(t, got[0].Locked)
	}
}

func TestAttr_QuietSuppressesSuccess(t *testing.T) {
	a := apptesting.OpenApp(t)
	seed(t, a)
	cfg := config.GetDefaults()
	cfg.Output.Quiet = 1

	out, err := runWithConfig(t, a, cfg, "Garage:Shelf", "--locked")
	require.NoError(t, err)
	assert.Empty(t, out)
	assert.True(t, entity(t, a, "Garage:Shelf").Locked)
}

func TestAttr_AtomicOnFailure(t *testing.T) {
	a := apptesting.OpenApp(t)
	seed(t, a)
	_, err := run(t, a, "Garage:Nails", "--locked")
	require.NoError(t, err)

	_, err = run(t, a, "Garage:Shelf", "Garage:Nails", "--locked")
	require.ErrorIs(t, err, app.ErrNoChange)
	assert.False(t, entity(t, a, "Garage:Shelf").Locked)
}

func TestAttr_SpaceSeparatedFalseIsAPath(t *testing.T) {
	a := apptesting.OpenApp(t)
	seed(t, a)

	_, err := run(t, a, "Garage:Shelf", "--locked", "false")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"false"`)
	assert.False(t, entity(t, a, "Garage:Shelf").Locked)
}

func TestAttr_DuplicatePath(t *testing.T) {
	a := apptesting.OpenApp(t)
	seed(t, a)

	_, err := run(t, a, "Garage:Shelf", "Garage:Shelf", "--locked")
	require.ErrorIs(t, err, app.ErrDuplicateEntity)
	assert.False(t, entity(t, a, "Garage:Shelf").Locked)
}
