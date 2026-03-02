package migrate_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/migrate"
)

func TestGetDatabaseCmd_RegisteredUnderMigrateCmd(t *testing.T) {
	migrateCmd := migrate.GetMigrateCmd()
	require.NotNil(t, migrateCmd, "migrate command should not be nil")

	found := false
	for _, sub := range migrateCmd.Commands() {
		if sub.Use == "database" {
			found = true
			break
		}
	}
	assert.True(t, found, "migrate command should have 'database' subcommand")
}

func TestGetDatabaseCmd_HasDryRunFlag(t *testing.T) {
	cmd := migrate.GetDatabaseCmd()
	require.NotNil(t, cmd, "database command should not be nil")

	flag := cmd.Flags().Lookup("dry-run")
	assert.NotNil(t, flag, "database command should have --dry-run flag")
}

func TestGetDatabaseCmd_DryRunDefaultFalse(t *testing.T) {
	cmd := migrate.GetDatabaseCmd()
	require.NotNil(t, cmd, "database command should not be nil")

	flag := cmd.Flags().Lookup("dry-run")
	require.NotNil(t, flag, "database command should have --dry-run flag")

	assert.Equal(t, "false", flag.DefValue, "--dry-run default should be false")
}

func TestGetDatabaseCmd_ShortHelp(t *testing.T) {
	cmd := migrate.GetDatabaseCmd()
	require.NotNil(t, cmd, "database command should not be nil")

	assert.NotEmpty(t, cmd.Short, "database command should have short help text")
	assert.Contains(t, cmd.Short, "migrate", "short help should mention migration")
}

func TestGetDatabaseCmd_LongHelp(t *testing.T) {
	cmd := migrate.GetDatabaseCmd()
	require.NotNil(t, cmd, "database command should not be nil")

	assert.NotEmpty(t, cmd.Long, "database command should have long help text")
	assert.Contains(t, cmd.Long, "UUID", "long help should reference UUID to ID migration")
}

func TestGetMigrateCmd_HasExpectedFields(t *testing.T) {
	cmd := migrate.GetMigrateCmd()
	require.NotNil(t, cmd, "migrate command should not be nil")

	assert.Equal(t, "migrate", cmd.Use, "command Use should be 'migrate'")
	assert.NotEmpty(t, cmd.Short, "migrate command should have short help")
	assert.NotEmpty(t, cmd.Long, "migrate command should have long help")
}
