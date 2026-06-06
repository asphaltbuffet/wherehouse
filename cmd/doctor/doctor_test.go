package doctor_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/doctor"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

type jsonDoctorResult struct {
	Healthy    bool        `json:"healthy"`
	IssueCount int         `json:"issue_count"`
	Issues     []jsonIssue `json:"issues"`
	Rebuilt    *int        `json:"rebuilt"`
}

type jsonIssue struct {
	Kind        string `json:"kind"`
	EventID     *int64 `json:"event_id"`
	Description string `json:"description"`
}

func unmarshalDoctorResult(t *testing.T, b []byte) jsonDoctorResult {
	t.Helper()
	var r jsonDoctorResult
	require.NoError(t, json.Unmarshal(b, &r))
	return r
}

func TestRunDoctor_AllClean_PrintsOK(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := doctor.NewDoctorCmd(a)
	cmd.SetContext(context.WithValue(t.Context(), config.ConfigKey, apptesting.NewTestConfig(t)))
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "OK")
}

func TestRunDoctor_Rebuild_Clean_PrintsCount(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := doctor.NewDoctorCmd(a)
	cmd.SetContext(context.WithValue(t.Context(), config.ConfigKey, apptesting.NewTestConfig(t)))
	cmd.SetArgs([]string{"--rebuild"})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "Rebuilt")
}

func TestRunDoctor_JSON_AllClean(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := doctor.NewDoctorCmd(a)
	cmd.SetContext(context.WithValue(t.Context(), config.ConfigKey, apptesting.NewTestConfig(t, apptesting.WithJSON())))
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})

	require.NoError(t, cmd.Execute())

	result := unmarshalDoctorResult(t, out.Bytes())
	assert.True(t, result.Healthy)
	assert.Equal(t, 0, result.IssueCount)
	assert.NotNil(t, result.Issues)
	assert.Empty(t, result.Issues)
	assert.Nil(t, result.Rebuilt)
}

func TestRunDoctor_JSON_Rebuild_IncludesCount(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := doctor.NewDoctorCmd(a)
	cmd.SetContext(context.WithValue(t.Context(), config.ConfigKey, apptesting.NewTestConfig(t, apptesting.WithJSON())))
	cmd.SetArgs([]string{"--rebuild"})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})

	require.NoError(t, cmd.Execute())

	result := unmarshalDoctorResult(t, out.Bytes())
	require.NotNil(t, result.Rebuilt)
}

func TestRunDoctor_JSON_NoRebuild_OmitsKey(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := doctor.NewDoctorCmd(a)
	cmd.SetContext(context.WithValue(t.Context(), config.ConfigKey, apptesting.NewTestConfig(t, apptesting.WithJSON())))
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})

	require.NoError(t, cmd.Execute())

	result := unmarshalDoctorResult(t, out.Bytes())
	assert.Nil(t, result.Rebuilt)
}

func TestRunDoctor_Quiet_SuppressesOK(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := doctor.NewDoctorCmd(a)
	cmd.SetContext(
		context.WithValue(t.Context(), config.ConfigKey, apptesting.NewTestConfig(t, apptesting.WithQuiet())),
	)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	require.NoError(t, cmd.Execute())
	assert.Empty(t, stdout.String())
	assert.Empty(t, stderr.String())
}
