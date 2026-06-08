package config

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
	internalconfig "github.com/asphaltbuffet/wherehouse/internal/config"
)

func TestGetPathCmd_Singleton(t *testing.T) {
	cmd := NewPathCmd()
	require.NotNil(t, cmd)
}

func TestConfigPath_Quiet_SuppressesInfoHeader(t *testing.T) {
	cmd := NewPathCmd()
	cmd.SetContext(
		context.WithValue(t.Context(), internalconfig.ConfigKey, apptesting.NewTestConfig(t, apptesting.WithQuiet())),
	)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})

	require.NoError(t, cmd.Execute())
	assert.NotContains(t, stdout.String(), "Active configuration files:")
}
