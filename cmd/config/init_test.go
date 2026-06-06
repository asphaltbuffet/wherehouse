package config

import (
	"bytes"
	"context"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	internalconfig "github.com/asphaltbuffet/wherehouse/internal/config"
)

func TestGetInitCmd_Singleton(t *testing.T) {
	cmd := NewInitCmd()
	require.NotNil(t, cmd)
}

func TestConfigInit_Quiet_SuppressesSuccess(t *testing.T) {
	origFS := cmdFS
	cmdFS = afero.NewMemMapFs()
	t.Cleanup(func() { cmdFS = origFS })

	cmd := NewInitCmd()
	cmd.SetContext(
		context.WithValue(t.Context(), internalconfig.ConfigKey, apptesting.NewTestConfig(t, apptesting.WithQuiet())),
	)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	require.NoError(t, cmd.Execute())
	assert.Empty(t, stdout.String())
	assert.Empty(t, stderr.String())
}
