package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/config"
)

// TestGetRootCmd_Singleton verifies GetRootCmd returns same instance on multiple calls.
func TestGetRootCmd_Singleton(t *testing.T) {
	// Reset global state for this test
	rootCmd = nil

	cmd1 := GetRootCmd()
	cmd2 := GetRootCmd()

	// Must be same instance (pointer equality)
	assert.Same(t, cmd1, cmd2)
}

// TestGetRootCmd_InitializesOnce verifies initialization happens only once.
func TestGetRootCmd_InitializesOnce(t *testing.T) {
	// Reset global state
	rootCmd = nil

	first := GetRootCmd()
	require.NotNil(t, first)

	// Manually modify the returned command
	oldUse := first.Use
	first.Use = "modified"

	// Get again and verify it's the exact same instance with modifications
	second := GetRootCmd()
	assert.Equal(t, "modified", second.Use)
	assert.Same(t, first, second)

	// Restore for other tests
	first.Use = oldUse
}

// TestGetRootCmd_HasSubcommands verifies root command has expected subcommands registered.
func TestGetRootCmd_HasSubcommands(t *testing.T) {
	rootCmd = nil
	cmd := GetRootCmd()

	subcommands := cmd.Commands()
	require.NotEmpty(t, subcommands)

	subcommandNames := make(map[string]bool)
	for _, sub := range subcommands {
		subcommandNames[sub.Name()] = true
	}

	assert.True(t, subcommandNames["config"], "config subcommand should be registered")
	assert.True(t, subcommandNames["add"], "add subcommand should be registered")
	assert.True(t, subcommandNames["find"], "find subcommand should be registered")
}

// TestGetRootCmd_HasPersistentFlags verifies root command has expected flags.
func TestGetRootCmd_HasPersistentFlags(t *testing.T) {
	rootCmd = nil
	cmd := GetRootCmd()

	// Check for specific flags
	require.NotNil(t, cmd.PersistentFlags().Lookup("config"))
	require.NotNil(t, cmd.PersistentFlags().Lookup("no-config"))
	require.NotNil(t, cmd.PersistentFlags().Lookup("db"))
	require.NotNil(t, cmd.PersistentFlags().Lookup("as"))
	require.NotNil(t, cmd.PersistentFlags().Lookup("json"))
	require.NotNil(t, cmd.PersistentFlags().Lookup("quiet"))
}

// TestInitConfig_NoConfigFlagLogic tests the --no-config flag business logic.
// Verifies that when noConfig is true, defaults are used regardless of other inputs.
func TestInitConfig_NoConfigFlagLogic(t *testing.T) {
	rootCmd = nil
	globalConfig = nil

	cmd := GetRootCmd()

	testCmd := &cobra.Command{}
	testCmd.SetContext(t.Context())
	testCmd.PersistentFlags().AddFlag(cmd.PersistentFlags().Lookup("no-config"))
	testCmd.PersistentFlags().AddFlag(cmd.PersistentFlags().Lookup("config"))

	require.NoError(t, testCmd.PersistentFlags().Set("no-config", "true"))

	// Call initConfig
	require.NoError(t, initConfig(testCmd, []string{}))

	// Verify defaults are used
	require.NotNil(t, globalConfig)
	defaults := config.GetDefaults()
	assert.Equal(t, defaults.Output.DefaultFormat, globalConfig.Output.DefaultFormat)
}

// TestInitConfig_SetsContextWithConfig verifies config is placed in context.
// This is critical for commands to access configuration.
func TestInitConfig_SetsContextWithConfig(t *testing.T) {
	rootCmd = nil
	globalConfig = nil

	cmd := GetRootCmd()

	testCmd := &cobra.Command{}
	testCmd.SetContext(t.Context())
	testCmd.PersistentFlags().AddFlag(cmd.PersistentFlags().Lookup("no-config"))
	testCmd.PersistentFlags().AddFlag(cmd.PersistentFlags().Lookup("config"))

	require.NoError(t, testCmd.PersistentFlags().Set("no-config", "true"))

	// Call initConfig
	require.NoError(t, initConfig(testCmd, []string{}))

	// Verify context contains config
	resultCtx := testCmd.Context()
	require.NotNil(t, resultCtx)

	cfg := resultCtx.Value(config.ConfigKey)
	require.NotNil(t, cfg)
	assert.IsType(t, (*config.Config)(nil), cfg)
}

// TestInitConfig_ContextPreservesExistingValues verifies other context values are preserved.
func TestInitConfig_ContextPreservesExistingValues(t *testing.T) {
	rootCmd = nil
	globalConfig = nil

	cmd := GetRootCmd()

	testCmd := &cobra.Command{}
	testCmd.PersistentFlags().AddFlag(cmd.PersistentFlags().Lookup("no-config"))
	testCmd.PersistentFlags().AddFlag(cmd.PersistentFlags().Lookup("config"))

	// Set context with existing value using a typed key
	type ctxKey int
	const customKeyType ctxKey = 0
	ctx := context.WithValue(t.Context(), customKeyType, "customValue")
	testCmd.SetContext(ctx)

	require.NoError(t, testCmd.PersistentFlags().Set("no-config", "true"))

	// Call initConfig
	require.NoError(t, initConfig(testCmd, []string{}))

	// Verify both old and new values accessible
	resultCtx := testCmd.Context()
	assert.Equal(t, "customValue", resultCtx.Value(customKeyType))

	assert.NotNil(t, resultCtx.Value(config.ConfigKey))
}

// TestInitConfig_DefaultsApplied verifies defaults are consistently applied.
func TestInitConfig_DefaultsApplied(t *testing.T) {
	rootCmd = nil
	globalConfig = nil

	cmd := GetRootCmd()

	testCmd := &cobra.Command{}
	testCmd.SetContext(context.Background())
	testCmd.PersistentFlags().AddFlag(cmd.PersistentFlags().Lookup("no-config"))
	testCmd.PersistentFlags().AddFlag(cmd.PersistentFlags().Lookup("config"))

	require.NoError(t, testCmd.PersistentFlags().Set("no-config", "true"))

	require.NoError(t, initConfig(testCmd, []string{}))

	// Verify globalConfig matches defaults
	defaults := config.GetDefaults()
	assert.Equal(t, defaults.Database.Path, globalConfig.Database.Path)
	assert.Equal(t, defaults.Output.DefaultFormat, globalConfig.Output.DefaultFormat)
	assert.Equal(t, defaults.Output.Quiet, globalConfig.Output.Quiet)
}

// TestInitConfig_FallsBackToDefaultsWithoutFlags tests fallback when no flags and no config files exist.
func TestInitConfig_FallsBackToDefaultsWithoutFlags(t *testing.T) {
	rootCmd = nil
	globalConfig = nil

	cmd := GetRootCmd()

	testCmd := &cobra.Command{}
	testCmd.SetContext(context.Background())
	testCmd.PersistentFlags().AddFlag(cmd.PersistentFlags().Lookup("no-config"))
	testCmd.PersistentFlags().AddFlag(cmd.PersistentFlags().Lookup("config"))

	// Don't set any flags - should fall back to defaults
	require.NoError(t, initConfig(testCmd, []string{}))

	require.NotNil(t, globalConfig)
	assert.NotEmpty(t, globalConfig.Database.Path)
}

// TestInitConfig_EnvironmentVariableConfig tests WHEREHOUSE_CONFIG environment variable usage.
func TestInitConfig_EnvironmentVariableConfig(t *testing.T) {
	tmpDir := t.TempDir()
	envConfigPath := filepath.Join(tmpDir, "env-config.toml")

	envConfigContent := `[database]
path = "/env/db.sqlite"

[user]
default_identity = "envuser"
`

	require.NoError(t, os.WriteFile(envConfigPath, []byte(envConfigContent), 0o644))

	t.Setenv("WHEREHOUSE_CONFIG", envConfigPath)

	rootCmd = nil
	globalConfig = nil

	cmd := GetRootCmd()

	testCmd := &cobra.Command{}
	testCmd.SetContext(context.Background())
	testCmd.PersistentFlags().AddFlag(cmd.PersistentFlags().Lookup("no-config"))
	testCmd.PersistentFlags().AddFlag(cmd.PersistentFlags().Lookup("config"))

	// Call initConfig with no config flag - should use WHEREHOUSE_CONFIG
	require.NoError(t, initConfig(testCmd, []string{}))

	require.NotNil(t, globalConfig)
	assert.Equal(t, "/env/db.sqlite", globalConfig.Database.Path)
	assert.Equal(t, "envuser", globalConfig.User.DefaultIdentity)
}

// TestInitConfig_ReturnsNilError tests that initConfig returns nil for success cases.
func TestInitConfig_ReturnsNilError(t *testing.T) {
	rootCmd = nil
	globalConfig = nil

	cmd := GetRootCmd()

	testCmd := &cobra.Command{}
	testCmd.SetContext(context.Background())
	testCmd.PersistentFlags().AddFlag(cmd.PersistentFlags().Lookup("no-config"))
	testCmd.PersistentFlags().AddFlag(cmd.PersistentFlags().Lookup("config"))

	require.NoError(t, testCmd.PersistentFlags().Set("no-config", "true"))

	// Call initConfig
	assert.NoError(t, initConfig(testCmd, []string{}))
}
