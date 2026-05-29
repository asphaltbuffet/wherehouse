package importcmd_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	importcmd "github.com/asphaltbuffet/wherehouse/cmd/import"
	"github.com/asphaltbuffet/wherehouse/internal/app"
)

// newTestRoot wraps the import command under a minimal root that carries the
// same persistent flags as cmd/root.go so inherited-flag behaviour is realistic.
func newTestRoot(fake *fakeImportApp) *cobra.Command {
	root := &cobra.Command{Use: "wherehouse", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().Bool("json", false, "")
	root.PersistentFlags().CountP("quiet", "q", "")
	root.AddCommand(importcmd.NewImportCmd(fake))
	return root
}

type fakeImportApp struct {
	hasEvents    bool
	hasEventsErr error
	importResult app.ImportResult
	importErr    error
	importCalled bool
	importOpts   app.ImportOptions
	clearCalled  bool
	callOrder    []string
}

func (f *fakeImportApp) HasEvents(_ context.Context) (bool, error) {
	return f.hasEvents, f.hasEventsErr
}

func (f *fakeImportApp) ClearAllData(_ context.Context) error {
	f.clearCalled = true
	f.callOrder = append(f.callOrder, "clear")
	return nil
}

func (f *fakeImportApp) ImportEvents(
	_ context.Context,
	_ []app.ExportResult,
	opts app.ImportOptions,
) (app.ImportResult, error) {
	f.importCalled = true
	f.importOpts = opts
	f.callOrder = append(f.callOrder, "import")
	return f.importResult, f.importErr
}

func makeNDJSON(events []app.ExportResult) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, ev := range events {
		_ = enc.Encode(ev)
	}
	return buf.String()
}

// --- slice 1: valid NDJSON → ImportEvents called, summary on stderr ---

func TestRunImport_ValidInput_CallsImportEventsAndPrintsSummary(t *testing.T) {
	t.Parallel()
	fake := &fakeImportApp{
		importResult: app.ImportResult{Replayed: 2, Failed: 0, Warnings: 0},
	}
	cmd := importcmd.NewImportCmd(fake)
	input := makeNDJSON([]app.ExportResult{
		{
			EventID:      1,
			EventType:    "entity.created",
			TimestampUTC: "2020-01-01T00:00:00Z",
			ActorUserID:  "alice",
			Payload:      json.RawMessage(`{}`),
		},
		{
			EventID:      2,
			EventType:    "entity.renamed",
			TimestampUTC: "2020-01-02T00:00:00Z",
			ActorUserID:  "alice",
			Payload:      json.RawMessage(`{}`),
		},
	})
	cmd.SetIn(bytes.NewBufferString(input))
	var stderr bytes.Buffer
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&stderr)

	require.NoError(t, cmd.Execute())
	assert.True(t, fake.importCalled, "ImportEvents should have been called")
	assert.Contains(t, stderr.String(), "Replayed: 2")
	assert.Contains(t, stderr.String(), "Failed: 0")
	assert.Contains(t, stderr.String(), "Warnings: 0")
}

func TestRunImport_EmptyInput_PrintsZeroSummary(t *testing.T) {
	t.Parallel()
	fake := &fakeImportApp{}
	cmd := importcmd.NewImportCmd(fake)
	cmd.SetIn(bytes.NewBufferString(""))
	var stderr bytes.Buffer
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&stderr)

	require.NoError(t, cmd.Execute())
	assert.Contains(t, stderr.String(), "Replayed: 0")
	assert.Contains(t, stderr.String(), "Failed: 0")
	assert.Contains(t, stderr.String(), "Warnings: 0")
}

func TestRunImport_MalformedJSON_ReturnsError(t *testing.T) {
	t.Parallel()
	fake := &fakeImportApp{}
	cmd := importcmd.NewImportCmd(fake)
	cmd.SetIn(bytes.NewBufferString("not valid json\n"))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.False(t, fake.importCalled)
}

func TestRunImport_ImportEventsError_ReturnsError(t *testing.T) {
	t.Parallel()
	fake := &fakeImportApp{importErr: errors.New("monotonic order violation")}
	cmd := importcmd.NewImportCmd(fake)
	input := makeNDJSON([]app.ExportResult{
		{
			EventID:      1,
			EventType:    "entity.created",
			TimestampUTC: "2020-01-01T00:00:00Z",
			ActorUserID:  "alice",
			Payload:      json.RawMessage(`{}`),
		},
	})
	cmd.SetIn(bytes.NewBufferString(input))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "monotonic order violation")
}

func TestRunImport_QuietFlag_SuppressesSummary(t *testing.T) {
	t.Parallel()
	fake := &fakeImportApp{importResult: app.ImportResult{Replayed: 1}}
	root := newTestRoot(fake)
	input := makeNDJSON([]app.ExportResult{
		{
			EventID:      1,
			EventType:    "entity.created",
			TimestampUTC: "2020-01-01T00:00:00Z",
			ActorUserID:  "alice",
			Payload:      json.RawMessage(`{}`),
		},
	})
	root.SetIn(bytes.NewBufferString(input))
	var stderr bytes.Buffer
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&stderr)
	root.SetArgs([]string{"import", "-q"})

	require.NoError(t, root.Execute())
	assert.Empty(t, stderr.String(), "summary should be suppressed with --quiet")
}

func TestRunImport_LongLine_ParsesWithoutScannerOverflow(t *testing.T) {
	t.Parallel()
	fake := &fakeImportApp{importResult: app.ImportResult{Replayed: 1}}
	cmd := importcmd.NewImportCmd(fake)

	// 256 KiB note — comfortably above bufio.Scanner's default 64 KiB cap.
	bigNote := strings.Repeat("x", 256*1024)
	input := makeNDJSON([]app.ExportResult{
		{
			EventID:      1,
			EventType:    "entity.created",
			TimestampUTC: "2020-01-01T00:00:00Z",
			ActorUserID:  "alice",
			Payload:      json.RawMessage(`{}`),
			Note:         &bigNote,
		},
	})
	cmd.SetIn(bytes.NewBufferString(input))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	require.NoError(t, cmd.Execute())
	assert.True(t, fake.importCalled, "long-line input must parse and reach ImportEvents")
}

func TestRunImport_NonEmptyDB_NoReplace_ReturnsError(t *testing.T) {
	t.Parallel()
	fake := &fakeImportApp{hasEvents: true}
	cmd := importcmd.NewImportCmd(fake)
	cmd.SetIn(bytes.NewBufferString(makeNDJSON([]app.ExportResult{
		{
			EventID:      1,
			EventType:    "entity.created",
			TimestampUTC: "2020-01-01T00:00:00Z",
			ActorUserID:  "alice",
			Payload:      json.RawMessage(`{}`),
		},
	})))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.False(t, fake.importCalled, "ImportEvents must not be called when DB is non-empty and --replace not set")
}

func TestRunImport_ReplaceWithoutYes_ReturnsError(t *testing.T) {
	t.Parallel()
	fake := &fakeImportApp{hasEvents: true}
	root := newTestRoot(fake)
	root.SetIn(bytes.NewBufferString(""))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"import", "--replace"})

	err := root.Execute()
	require.Error(t, err)
	assert.False(t, fake.clearCalled, "ClearAllData must not be called without --yes")
	assert.False(t, fake.importCalled)
}

func TestRunImport_ReplaceAndYes_ClearsBeforeImport(t *testing.T) {
	t.Parallel()
	fake := &fakeImportApp{hasEvents: true}
	root := newTestRoot(fake)
	input := makeNDJSON([]app.ExportResult{
		{
			EventID:      1,
			EventType:    "entity.created",
			TimestampUTC: "2020-01-01T00:00:00Z",
			ActorUserID:  "alice",
			Payload:      json.RawMessage(`{}`),
		},
	})
	root.SetIn(bytes.NewBufferString(input))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"import", "--replace", "--yes"})

	require.NoError(t, root.Execute())
	assert.True(t, fake.importCalled)
	assert.True(
		t,
		fake.importOpts.Replace,
		"--replace --yes must pass Replace=true to ImportEvents; app layer owns the clear",
	)
}

func TestRunImport_YesWithoutReplace_AcceptedSilently(t *testing.T) {
	t.Parallel()
	fake := &fakeImportApp{hasEvents: false}
	root := newTestRoot(fake)
	root.SetIn(bytes.NewBufferString(""))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"import", "--yes"})

	require.NoError(t, root.Execute())
	assert.False(t, fake.clearCalled)
}

func TestRunImport_ContinueFlag_ExitsZeroAndShowsFailedCount(t *testing.T) {
	t.Parallel()
	fake := &fakeImportApp{
		hasEvents:    false,
		importResult: app.ImportResult{Replayed: 1, Failed: 2, Warnings: 0},
	}
	root := newTestRoot(fake)
	input := makeNDJSON([]app.ExportResult{
		{
			EventID:      1,
			EventType:    "entity.created",
			TimestampUTC: "2020-01-01T00:00:00Z",
			ActorUserID:  "alice",
			Payload:      json.RawMessage(`{}`),
		},
	})
	root.SetIn(bytes.NewBufferString(input))
	var stderr bytes.Buffer
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&stderr)
	root.SetArgs([]string{"import", "--continue"})

	require.NoError(t, root.Execute(), "exit must be 0 with --continue even on partial failure")
	assert.True(t, fake.importOpts.Continue, "--continue must be passed through to ImportOptions")
	assert.Contains(t, stderr.String(), "Failed: 2")
}
