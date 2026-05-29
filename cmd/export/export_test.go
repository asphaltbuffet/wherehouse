package export_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	export "github.com/asphaltbuffet/wherehouse/cmd/export"
	"github.com/asphaltbuffet/wherehouse/internal/app"
)

// newTestRoot returns a minimal root command with the same persistent flags
// that cmd/root.go registers, so inherited-flag behaviour is realistic.
func newTestRoot(fake *fakeExportApp) *cobra.Command {
	root := &cobra.Command{Use: "wherehouse", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().Bool("json", false, "")
	root.PersistentFlags().CountP("quiet", "q", "")
	root.AddCommand(export.NewExportCmd(fake))
	return root
}

type fakeExportApp struct {
	resp []app.ExportResult
	err  error
}

func (f *fakeExportApp) GetAllEvents(_ context.Context) ([]app.ExportResult, error) {
	return f.resp, f.err
}

func TestRunExport_ZeroEvents(t *testing.T) {
	t.Parallel()
	fake := &fakeExportApp{}
	cmd := export.NewExportCmd(fake)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	require.NoError(t, cmd.Execute())
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), "warning")
}

func TestRunExport_OneEvent_OutputsValidJSON(t *testing.T) {
	t.Parallel()
	entityID := "abc123"
	fake := &fakeExportApp{
		resp: []app.ExportResult{
			{
				EventID:      1,
				EventType:    "entity.created",
				TimestampUTC: "2026-05-28T00:00:00Z",
				ActorUserID:  "user@example.com",
				EntityID:     &entityID,
				Payload:      json.RawMessage(`{"name":"Garage"}`),
			},
		},
	}
	cmd := export.NewExportCmd(fake)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	lines := splitLines(stdout.String())
	require.Len(t, lines, 1)

	var result app.ExportResult
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &result))
	assert.Equal(t, int64(1), result.EventID)
	assert.Equal(t, "entity.created", result.EventType)
	assert.Equal(t, "2026-05-28T00:00:00Z", result.TimestampUTC)
	assert.Equal(t, "user@example.com", result.ActorUserID)
}

func TestRunExport_MultipleEvents_OrderedByEventID(t *testing.T) {
	t.Parallel()
	fake := &fakeExportApp{
		resp: []app.ExportResult{
			{
				EventID:      1,
				EventType:    "entity.created",
				TimestampUTC: "2026-05-28T00:00:00Z",
				ActorUserID:  "u",
				Payload:      json.RawMessage(`{}`),
			},
			{
				EventID:      2,
				EventType:    "entity.moved",
				TimestampUTC: "2026-05-28T00:01:00Z",
				ActorUserID:  "u",
				Payload:      json.RawMessage(`{}`),
			},
			{
				EventID:      3,
				EventType:    "entity.renamed",
				TimestampUTC: "2026-05-28T00:02:00Z",
				ActorUserID:  "u",
				Payload:      json.RawMessage(`{}`),
			},
		},
	}
	cmd := export.NewExportCmd(fake)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	lines := splitLines(stdout.String())
	require.Len(t, lines, 3)

	var prev int64
	for _, line := range lines {
		var result app.ExportResult
		require.NoError(t, json.Unmarshal([]byte(line), &result))
		assert.Greater(t, result.EventID, prev)
		prev = result.EventID
	}
}

func TestRunExport_PayloadRoundTrip(t *testing.T) {
	t.Parallel()
	fake := &fakeExportApp{
		resp: []app.ExportResult{
			{
				EventID:      1,
				EventType:    "entity.created",
				TimestampUTC: "2026-05-28T00:00:00Z",
				ActorUserID:  "u",
				Payload:      json.RawMessage(`{"key":"value","num":42}`),
			},
		},
	}
	cmd := export.NewExportCmd(fake)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	lines := splitLines(stdout.String())
	require.Len(t, lines, 1)

	// Parse the raw JSON line into a generic map to inspect the payload field type.
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &raw))

	payloadRaw, ok := raw["payload"]
	require.True(t, ok, "payload field must be present")

	// payload must be a JSON object, not a string or base64
	var payloadObj map[string]any
	require.NoError(t, json.Unmarshal(payloadRaw, &payloadObj), "payload must unmarshal as a JSON object, not a string")
	assert.Equal(t, "value", payloadObj["key"])
}

func TestRunExport_PropagatesError(t *testing.T) {
	t.Parallel()
	fake := &fakeExportApp{err: errors.New("db failure")}
	cmd := export.NewExportCmd(fake)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db failure")
}

func TestRunExport_ZeroEvents_QuietSuppressesWarning(t *testing.T) {
	t.Parallel()
	fake := &fakeExportApp{}
	root := newTestRoot(fake)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"export", "-q"})
	require.NoError(t, root.Execute())
	assert.Empty(t, stdout.String())
	assert.Empty(t, stderr.String())
}

func TestRunExport_JsonFlagIsNoOp(t *testing.T) {
	t.Parallel()
	fake := &fakeExportApp{
		resp: []app.ExportResult{
			{
				EventID:      1,
				EventType:    "entity.created",
				TimestampUTC: "2026-05-28T00:00:00Z",
				ActorUserID:  "u",
				Payload:      json.RawMessage(`{}`),
			},
		},
	}
	root := newTestRoot(fake)
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"export", "--json"})
	require.NoError(t, root.Execute())
	lines := splitLines(stdout.String())
	require.Len(t, lines, 1)
	var result app.ExportResult
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &result))
	assert.Equal(t, int64(1), result.EventID)
}

// splitLines returns non-empty lines from s.
func splitLines(s string) []string {
	var lines []string
	for line := range bytes.SplitSeq([]byte(s), []byte("\n")) {
		if len(bytes.TrimSpace(line)) > 0 {
			lines = append(lines, string(line))
		}
	}
	return lines
}
