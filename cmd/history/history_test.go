package history

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/config"
)

func TestHistoryCommand_HasAllFlags(t *testing.T) {
	cmd := GetHistoryCmd()

	assert.NotNil(t, cmd.Flags().Lookup("limit"))
	assert.NotNil(t, cmd.Flags().Lookup("since"))
	assert.NotNil(t, cmd.Flags().Lookup("oldest-first"))
	assert.NotNil(t, cmd.Flags().Lookup("id"))
}

func TestHistoryCommand_LimitDefaultsToZero(t *testing.T) {
	cmd := GetHistoryCmd()

	limit, err := cmd.Flags().GetInt("limit")

	require.NoError(t, err)
	assert.Zero(t, limit)
}

func TestHistoryCommand_OldestFirstDefaultsFalse(t *testing.T) {
	cmd := GetHistoryCmd()

	oldestFirst, err := cmd.Flags().GetBool("oldest-first")

	require.NoError(t, err)
	assert.False(t, oldestFirst)
}

func TestHistoryCommand_SinceDefaultsEmpty(t *testing.T) {
	cmd := GetHistoryCmd()

	since, err := cmd.Flags().GetString("since")

	require.NoError(t, err)
	assert.Empty(t, since)
}

func TestOpenDatabase_RequiresConfig(t *testing.T) {
	ctx := t.Context()

	_, err := openDatabase(ctx)

	require.ErrorContains(t, err, "configuration not found")
}

func TestOpenDatabase_WithValidConfig_Works(t *testing.T) {
	cfg := &config.Config{}
	ctx := context.WithValue(context.Background(), config.ConfigKey, cfg)

	_, err := openDatabase(ctx)

	// cfg default will work here
	require.NoError(t, err)
}

func TestHistoryCommand_ContextPassing(t *testing.T) {
	// Verify context is properly passed through the command
	cmd := GetHistoryCmd()

	ctx := t.Context()
	cmd.SetContext(ctx)

	assert.Equal(t, ctx, cmd.Context())
}

func TestHistoryCommand_OutputWriter(t *testing.T) {
	// Verify the command can write to a custom output writer
	cmd := GetHistoryCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)

	assert.NotNil(t, cmd.OutOrStdout())
}
