package doctor_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/doctor"
	"github.com/asphaltbuffet/wherehouse/internal/app"
)

type fakeDoctorApp struct {
	eventIssues []app.DoctorIssue
	eventErr    error
	projIssues  []app.DoctorIssue
	projErr     error
	replayCount int
	replayErr   error
}

func (f *fakeDoctorApp) ValidateEventLog(_ context.Context) ([]app.DoctorIssue, error) {
	return f.eventIssues, f.eventErr
}

func (f *fakeDoctorApp) CheckProjectionConsistency(_ context.Context) ([]app.DoctorIssue, error) {
	return f.projIssues, f.projErr
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
		eventIssues: []app.DoctorIssue{
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
		projIssues: []app.DoctorIssue{
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
		eventIssues: []app.DoctorIssue{
			{Kind: app.DoctorKindEventLog, Description: "event 1 bad"},
		},
		projIssues: []app.DoctorIssue{
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
		eventIssues: []app.DoctorIssue{
			{Kind: app.DoctorKindEventLog, Description: "bad event"},
		},
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
		eventIssues: []app.DoctorIssue{
			{Kind: app.DoctorKindEventLog, Description: "bad event"},
		},
		replayCount: 3,
	}
	cmd := doctor.NewDoctorCmd(fake)
	cmd.SetArgs([]string{"--rebuild", "--force"})
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})

	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "Rebuilt")
	assert.Contains(t, out.String(), "3")
}

func TestRunDoctor_AppError_Propagates(t *testing.T) {
	t.Parallel()
	fake := &fakeDoctorApp{eventErr: errTest}
	cmd := doctor.NewDoctorCmd(fake)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validate event log")
}

var errTest = errors.New("db connection lost")
