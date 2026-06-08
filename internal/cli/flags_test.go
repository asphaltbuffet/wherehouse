package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/config"
)

// TestGetConfig_NotInContext tests GetConfig returns (nil, false) when not in context.
func TestGetConfig_NotInContext(t *testing.T) {
	ctx := context.Background()

	cfg, ok := GetConfig(ctx)

	assert.Nil(t, cfg)
	assert.False(t, ok)
}

// TestGetConfig_InContext tests GetConfig returns (cfg, true) when in context.
func TestGetConfig_InContext(t *testing.T) {
	testCfg := &config.Config{
		Database: config.DatabaseConfig{Path: "/test/db.sqlite"},
		Output:   config.OutputConfig{DefaultFormat: "json", Quiet: 1},
	}
	ctx := context.WithValue(context.Background(), config.ConfigKey, testCfg)

	cfg, ok := GetConfig(ctx)

	require.True(t, ok)
	require.NotNil(t, cfg)
	assert.Equal(t, "/test/db.sqlite", cfg.Database.Path)
	assert.Equal(t, "json", cfg.Output.DefaultFormat)
	assert.Equal(t, 1, cfg.Output.Quiet)
}

// TestMustGetConfig_NotInContext tests MustGetConfig panics when not in context.
func TestMustGetConfig_NotInContext(t *testing.T) {
	ctx := context.Background()

	assert.Panics(t, func() {
		MustGetConfig(ctx)
	})
}

// TestMustGetConfig_InContext tests MustGetConfig returns cfg when in context.
func TestMustGetConfig_InContext(t *testing.T) {
	testCfg := &config.Config{
		Database: config.DatabaseConfig{Path: "/test/db.sqlite"},
		Output:   config.OutputConfig{DefaultFormat: "human", Quiet: 0},
	}
	ctx := context.WithValue(context.Background(), config.ConfigKey, testCfg)

	cfg := MustGetConfig(ctx)

	require.NotNil(t, cfg)
	assert.Equal(t, "/test/db.sqlite", cfg.Database.Path)
	assert.Equal(t, "human", cfg.Output.DefaultFormat)
	assert.Equal(t, 0, cfg.Output.Quiet)
}
