package db_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbcmd "github.com/asphaltbuffet/wherehouse/cmd/db"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

func TestDbNameCmd_SetsName(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := dbcmd.NewDBCmd(a)
	cmd.SetArgs([]string{"name", "My Workshop"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	info, err := a.GetInfo(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "My Workshop", info.Name)
}

func TestDbNameCmd_ClearsName(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()

	require.NoError(t, a.SetWherehouseName(ctx, "Old Name"))

	cmd := dbcmd.NewDBCmd(a)
	cmd.SetArgs([]string{"name", "--clear"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	info, err := a.GetInfo(ctx)
	require.NoError(t, err)
	assert.Equal(t, "(unnamed)", info.Name)
}

func TestDbNameCmd_EmptyArgRejected(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := dbcmd.NewDBCmd(a)
	cmd.SetArgs([]string{"name", ""})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	assert.Error(t, cmd.Execute())
}

func TestDbNameCmd_ClearAndArgMutuallyExclusive(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := dbcmd.NewDBCmd(a)
	cmd.SetArgs([]string{"name", "--clear", "Some Name"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	assert.Error(t, cmd.Execute())
}

func TestDbNameCmd_JSON(t *testing.T) {
	a := apptesting.OpenApp(t)
	out := &bytes.Buffer{}
	cmd := dbcmd.NewDBCmd(a)
	cmd.SetArgs([]string{"name", "Test Name"})
	cmd.SetContext(context.WithValue(t.Context(), config.ConfigKey, apptesting.NewTestConfig(t, apptesting.WithJSON())))
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	var result map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &result))
	assert.Equal(t, "Test Name", result["name"])
}
