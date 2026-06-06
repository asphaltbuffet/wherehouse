package importcmd_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	importcmd "github.com/asphaltbuffet/wherehouse/cmd/import"
	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

func newTestRoot(a *app.App) *cobra.Command {
	root := &cobra.Command{Use: "wherehouse", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().CountP("quiet", "q", "")
	root.AddCommand(importcmd.NewImportCmd(a))
	return root
}

func makeNDJSON(events []app.ExportResult) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, ev := range events {
		_ = enc.Encode(ev)
	}
	return buf.String()
}

func oneCreatedEvent() []app.ExportResult {
	return []app.ExportResult{
		{
			EventID:      1,
			EventType:    "entity.created",
			TimestampUTC: "2020-01-01T00:00:00Z",
			ActorUserID:  "alice",
			Payload:      json.RawMessage(`{"display_name":"Garage","entity_type":"place","parent_path":""}`),
		},
	}
}

func seedOne(t *testing.T, a *app.App) {
	t.Helper()
	_, err := a.CreateEntity(context.Background(), app.CreateEntityRequest{
		DisplayName: "Garage",
		EntityType:  inventory.EntityTypePlace,
		ActorID:     "test",
	})
	require.NoError(t, err)
}

func TestRunImport_ValidInput_PrintsSummary(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := importcmd.NewImportCmd(a)
	cmd.SetIn(bytes.NewBufferString(makeNDJSON(oneCreatedEvent())))
	var stderr bytes.Buffer
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&stderr)

	require.NoError(t, cmd.Execute())
	assert.Contains(t, stderr.String(), "Replayed:")
}

func TestRunImport_EmptyInput_PrintsZeroSummary(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := importcmd.NewImportCmd(a)
	cmd.SetIn(bytes.NewBufferString(""))
	var stderr bytes.Buffer
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&stderr)

	require.NoError(t, cmd.Execute())
	assert.Contains(t, stderr.String(), "Replayed: 0")
}

func TestRunImport_MalformedJSON_ReturnsError(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := importcmd.NewImportCmd(a)
	cmd.SetIn(bytes.NewBufferString("not valid json\n"))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
}

func TestRunImport_QuietFlag_SuppressesSummary(t *testing.T) {
	a := apptesting.OpenApp(t)
	root := newTestRoot(a)
	root.SetIn(bytes.NewBufferString(makeNDJSON(oneCreatedEvent())))
	var stderr bytes.Buffer
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&stderr)
	root.SetArgs([]string{"import", "-q"})

	require.NoError(t, root.Execute())
	assert.Empty(t, stderr.String(), "summary should be suppressed with --quiet")
}

func TestRunImport_LongLine_ParsesWithoutScannerOverflow(t *testing.T) {
	a := apptesting.OpenApp(t)
	cmd := importcmd.NewImportCmd(a)

	bigNote := strings.Repeat("x", 256*1024)
	ev := oneCreatedEvent()[0]
	ev.Note = &bigNote
	cmd.SetIn(bytes.NewBufferString(makeNDJSON([]app.ExportResult{ev})))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	require.NoError(t, cmd.Execute())
}

func TestRunImport_NonEmptyDB_NoReplace_ReturnsError(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedOne(t, a)

	cmd := importcmd.NewImportCmd(a)
	cmd.SetIn(bytes.NewBufferString(makeNDJSON(oneCreatedEvent())))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not empty")
}

func TestRunImport_ReplaceWithoutYes_ReturnsError(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedOne(t, a)
	root := newTestRoot(a)
	root.SetIn(bytes.NewBufferString(""))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"import", "--replace"})

	err := root.Execute()
	require.Error(t, err)
}

func TestRunImport_ReplaceAndYes_Succeeds(t *testing.T) {
	a := apptesting.OpenApp(t)
	seedOne(t, a)
	root := newTestRoot(a)
	root.SetIn(bytes.NewBufferString(makeNDJSON(oneCreatedEvent())))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"import", "--replace", "--yes"})

	require.NoError(t, root.Execute())
}

func TestRunImport_YesWithoutReplace_AcceptedSilently(t *testing.T) {
	a := apptesting.OpenApp(t)
	root := newTestRoot(a)
	root.SetIn(bytes.NewBufferString(""))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"import", "--yes"})

	require.NoError(t, root.Execute())
}

func TestRunImport_ContinueFlag_ExitsZero(t *testing.T) {
	a := apptesting.OpenApp(t)
	root := newTestRoot(a)
	root.SetIn(bytes.NewBufferString(makeNDJSON(oneCreatedEvent())))
	var stderr bytes.Buffer
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&stderr)
	root.SetArgs([]string{"import", "--continue"})

	require.NoError(t, root.Execute(), "exit must be 0 with --continue")
}
