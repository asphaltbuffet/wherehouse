package export_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	export "github.com/asphaltbuffet/wherehouse/cmd/export"
	"github.com/asphaltbuffet/wherehouse/internal/app"
)

type fakeExportApp struct {
	resp []app.ExportResult
	err  error
}

func (f *fakeExportApp) GetAllEvents(_ context.Context) ([]app.ExportResult, error) {
	return f.resp, f.err
}

func TestRunExport_RunsWithoutError(t *testing.T) {
	t.Parallel()
	fake := &fakeExportApp{}
	cmd := export.NewExportCmd(fake)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())
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
