package export_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	export "github.com/asphaltbuffet/wherehouse/cmd/export"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func seedOne(t *testing.T, a *app.App) {
	t.Helper()
	_, err := a.CreateEntity(t.Context(), app.CreateEntityRequest{
		DisplayName: "Garage",
		EntityType:  inventory.EntityTypePlace,
		ActorID:     "test",
	})
	require.NoError(t, err)
}

func TestRunExport_ZeroEvents(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := export.NewExportCmd(a)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	require.NoError(t, cmd.Execute())
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), "warning")
}

func TestRunExport_OneEvent_OutputsValidJSON(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedOne(t, a)

	cmd := export.NewExportCmd(a)
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
}

func TestRunExport_MultipleEvents_OrderedByEventID(t *testing.T) {
	a := apptesting.OpenApp(t)
	ctx := t.Context()
	// Seeding 3 entities = 3 entity.created events
	for _, tc := range []struct {
		name   string
		parent string
		et     inventory.EntityType
	}{
		{"Garage", "", inventory.EntityTypePlace},
		{"Toolbox", "Garage", inventory.EntityTypeContainer},
		{"Wrench", "Garage:Toolbox", inventory.EntityTypeLeaf},
	} {
		_, err := a.CreateEntity(ctx, app.CreateEntityRequest{
			DisplayName: tc.name,
			EntityType:  tc.et,
			ParentPath:  tc.parent,
			ActorID:     "test",
		})
		require.NoError(t, err)
	}

	cmd := export.NewExportCmd(a)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	lines := splitLines(stdout.String())
	require.GreaterOrEqual(t, len(lines), 3)

	var prev int64
	for _, line := range lines {
		var result app.ExportResult
		require.NoError(t, json.Unmarshal([]byte(line), &result))
		assert.Greater(t, result.EventID, prev)
		prev = result.EventID
	}
}

func TestRunExport_PayloadRoundTrip(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedOne(t, a)

	cmd := export.NewExportCmd(a)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	lines := splitLines(stdout.String())
	require.Len(t, lines, 1)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &raw))

	payloadRaw, ok := raw["payload"]
	require.True(t, ok, "payload field must be present")

	var payloadObj map[string]any
	require.NoError(t, json.Unmarshal(payloadRaw, &payloadObj), "payload must unmarshal as a JSON object, not a string")
}

func TestRunExport_ZeroEvents_QuietSuppressesWarning(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := export.NewExportCmd(a)
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

func TestRunExport_NoFlags_OutputsNDJSON(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedOne(t, a)

	cmd := export.NewExportCmd(a)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())
	lines := splitLines(stdout.String())
	require.Len(t, lines, 1)
	var result app.ExportResult
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &result))
	assert.Equal(t, int64(1), result.EventID)
}

func splitLines(s string) []string {
	var lines []string
	for line := range bytes.SplitSeq([]byte(s), []byte("\n")) {
		if len(bytes.TrimSpace(line)) > 0 {
			lines = append(lines, string(line))
		}
	}
	return lines
}
