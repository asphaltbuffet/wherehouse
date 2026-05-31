package doctor_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/goccy/go-json"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/doctor"
	"github.com/asphaltbuffet/wherehouse/internal/app"
)

type fakeDoctorApp struct {
	issues      []app.DoctorIssue
	checksErr   error
	replayCount int
	replayErr   error
}

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

func (f *fakeDoctorApp) RunDoctorChecks(_ context.Context) ([]app.DoctorIssue, error) {
	return f.issues, f.checksErr
}

func (f *fakeDoctorApp) TruncateAndReplay(_ context.Context) (int, error) {
	return f.replayCount, f.replayErr
}

func TestRunDoctor_AllClean_PrintsOK(t *testing.T) {
	t.Parallel()
	fake := &fakeDoctorApp{}
	cmd := doctor.NewDoctorCmd(fake)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "OK")
}

func TestRunDoctor_EventLogIssues_PrintedAndNonZero(t *testing.T) {
	t.Parallel()
	fake := &fakeDoctorApp{
		issues: []app.DoctorIssue{
			{Kind: app.DoctorKindEventLog, Description: "event 3 missing entity_id"},
		},
	}
	cmd := doctor.NewDoctorCmd(fake)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, out.String(), "[event_log]")
	assert.Contains(t, out.String(), "event 3 missing entity_id")
}

func TestRunDoctor_ProjectionIssues_PrintedAndNonZero(t *testing.T) {
	t.Parallel()
	fake := &fakeDoctorApp{
		issues: []app.DoctorIssue{
			{Kind: app.DoctorKindProjection, Description: "phantom row: abc123"},
		},
	}
	cmd := doctor.NewDoctorCmd(fake)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, out.String(), "[projection]")
	assert.Contains(t, out.String(), "phantom row: abc123")
}

func TestRunDoctor_BothChecksRun_WhenEventLogHasIssues(t *testing.T) {
	t.Parallel()
	fake := &fakeDoctorApp{
		issues: []app.DoctorIssue{
			{Kind: app.DoctorKindEventLog, Description: "event 1 bad"},
			{Kind: app.DoctorKindProjection, Description: "phantom row: xyz"},
		},
	}
	cmd := doctor.NewDoctorCmd(fake)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, out.String(), "[event_log]")
	assert.Contains(t, out.String(), "[projection]")
}

func TestRunDoctor_Rebuild_Clean_PrintsCount(t *testing.T) {
	t.Parallel()
	fake := &fakeDoctorApp{replayCount: 7}
	cmd := doctor.NewDoctorCmd(fake)
	cmd.SetArgs([]string{"--rebuild"})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "7")
}

func TestRunDoctor_Rebuild_WithIssues_Skipped(t *testing.T) {
	t.Parallel()
	fake := &fakeDoctorApp{
		issues:      []app.DoctorIssue{{Kind: app.DoctorKindEventLog, Description: "bad event"}},
		replayCount: 5,
	}
	cmd := doctor.NewDoctorCmd(fake)
	cmd.SetArgs([]string{"--rebuild"})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.NotContains(t, out.String(), "Rebuilt")
}

func TestRunDoctor_RebuildForce_WithIssues_Runs(t *testing.T) {
	t.Parallel()
	fake := &fakeDoctorApp{
		issues:      []app.DoctorIssue{{Kind: app.DoctorKindEventLog, Description: "bad event"}},
		replayCount: 3,
	}
	cmd := doctor.NewDoctorCmd(fake)
	cmd.SetArgs([]string{"--rebuild", "--force"})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, out.String(), "Rebuilt")
	assert.Contains(t, out.String(), "3")
}

func TestRunDoctor_AppError_Propagates(t *testing.T) {
	t.Parallel()
	fake := &fakeDoctorApp{checksErr: errTest}
	cmd := doctor.NewDoctorCmd(fake)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.ErrorIs(t, err, errTest)
}

func TestRunDoctor_JSON_AllClean(t *testing.T) {
	t.Parallel()
	fake := &fakeDoctorApp{}
	cmd := doctor.NewDoctorCmd(fake)
	cmd.SetArgs([]string{"--json"})
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

func TestRunDoctor_JSON_WithIssues(t *testing.T) {
	t.Parallel()
	fake := &fakeDoctorApp{
		issues: []app.DoctorIssue{
			{Kind: app.DoctorKindEventLog, Description: "event 3 missing entity_id"},
		},
	}
	cmd := doctor.NewDoctorCmd(fake)
	cmd.SetArgs([]string{"--json"})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)

	result := unmarshalDoctorResult(t, out.Bytes())
	assert.False(t, result.Healthy)
	assert.Equal(t, 1, result.IssueCount)
	require.Len(t, result.Issues, 1)
	assert.Equal(t, "event_log", result.Issues[0].Kind)
	assert.Equal(t, "event 3 missing entity_id", result.Issues[0].Description)
}

func TestRunDoctor_JSON_EventID_NullWhenAbsent(t *testing.T) {
	t.Parallel()
	fake := &fakeDoctorApp{
		issues: []app.DoctorIssue{
			{Kind: app.DoctorKindEventLog, Description: "no event id"},
		},
	}
	cmd := doctor.NewDoctorCmd(fake)
	cmd.SetArgs([]string{"--json"})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.Execute()

	result := unmarshalDoctorResult(t, out.Bytes())
	assert.Nil(t, result.Issues[0].EventID)
}

func TestRunDoctor_JSON_EventID_PresentWhenSet(t *testing.T) {
	t.Parallel()
	eventID := int64(42)
	fake := &fakeDoctorApp{
		issues: []app.DoctorIssue{
			{Kind: app.DoctorKindEventLog, EventID: &eventID, Description: "bad payload"},
		},
	}
	cmd := doctor.NewDoctorCmd(fake)
	cmd.SetArgs([]string{"--json"})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.Execute()

	result := unmarshalDoctorResult(t, out.Bytes())
	require.NotNil(t, result.Issues[0].EventID)
	assert.Equal(t, int64(42), *result.Issues[0].EventID)
}

func TestRunDoctor_JSON_Rebuild_IncludesCount(t *testing.T) {
	t.Parallel()
	fake := &fakeDoctorApp{replayCount: 5}
	cmd := doctor.NewDoctorCmd(fake)
	cmd.SetArgs([]string{"--json", "--rebuild"})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})

	require.NoError(t, cmd.Execute())

	result := unmarshalDoctorResult(t, out.Bytes())
	require.NotNil(t, result.Rebuilt)
	assert.Equal(t, 5, *result.Rebuilt)
}

func TestRunDoctor_JSON_NoRebuild_OmitsKey(t *testing.T) {
	t.Parallel()
	fake := &fakeDoctorApp{}
	cmd := doctor.NewDoctorCmd(fake)
	cmd.SetArgs([]string{"--json"})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})

	require.NoError(t, cmd.Execute())

	result := unmarshalDoctorResult(t, out.Bytes())
	assert.Nil(t, result.Rebuilt)
}

var errTest = errors.New("db connection lost")
