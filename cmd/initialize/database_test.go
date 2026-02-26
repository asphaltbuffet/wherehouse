package initialize

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/config"
)

// resetForTesting resets package-level command singletons between tests.
func resetForTesting() {
	databaseCmd = nil
	initializeCmd = nil
}

// makeTestConfig returns a *config.Config pointing the database path to the given path.
func makeTestConfig(dbPath string) *config.Config {
	cfg := config.GetDefaults()
	cfg.Database.Path = dbPath
	return cfg
}

// runDatabaseCmd executes `initialize database` with the given args and config.
// Returns stdout, stderr, and any error.
func runDatabaseCmd(t *testing.T, cfg *config.Config, args ...string) (string, string, error) {
	t.Helper()

	resetForTesting()

	cmd := GetDatabaseCmd()
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd.SetOut(outBuf)
	cmd.SetErr(errBuf)

	ctx := context.WithValue(context.Background(), config.ConfigKey, cfg)
	cmd.SetContext(ctx)

	cmd.SetArgs(args)
	err := cmd.Execute()

	return outBuf.String(), errBuf.String(), err
}

func TestRunInitializeDatabase_FreshInstall(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "sub", "wherehouse.db")
	cfg := makeTestConfig(dbPath)

	stdout, _, err := runDatabaseCmd(t, cfg)

	require.NoError(t, err)
	assert.Contains(t, stdout, "Database initialized at")
	assert.Contains(t, stdout, dbPath)

	_, statErr := os.Stat(dbPath)
	assert.NoError(t, statErr, "database file should exist")
}

func TestRunInitializeDatabase_DirExists_NoFile(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "wherehouse.db")
	cfg := makeTestConfig(dbPath)

	stdout, _, err := runDatabaseCmd(t, cfg)

	require.NoError(t, err)
	assert.Contains(t, stdout, dbPath)

	_, statErr := os.Stat(dbPath)
	assert.NoError(t, statErr, "database file should exist")
}

func TestRunInitializeDatabase_AlreadyExists_NoForce(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "wherehouse.db")

	// Create an existing file.
	require.NoError(t, os.WriteFile(dbPath, []byte("existing"), 0o600))

	cfg := makeTestConfig(dbPath)

	_, _, err := runDatabaseCmd(t, cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "database already exists")
	assert.Contains(t, err.Error(), "--force")

	// Original file must be untouched.
	data, readErr := os.ReadFile(dbPath)
	require.NoError(t, readErr)
	assert.Equal(t, "existing", string(data))
}

func TestRunInitializeDatabase_AlreadyExists_Force_BackupCreated(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "wherehouse.db")

	require.NoError(t, os.WriteFile(dbPath, []byte("old-db"), 0o600))

	cfg := makeTestConfig(dbPath)

	stdout, stderr, err := runDatabaseCmd(t, cfg, "--force")

	require.NoError(t, err)
	assert.Empty(t, stderr, "no warning expected on successful backup")
	assert.Contains(t, stdout, "Backed up existing database to")
	assert.Contains(t, stdout, "Database initialized at")

	dateSuffix := time.Now().Format("20060102")
	backupPath := dbPath + ".backup." + dateSuffix

	_, statErr := os.Stat(backupPath)
	require.NoError(t, statErr, "backup file should exist at %s", backupPath)

	data, readErr := os.ReadFile(backupPath)
	require.NoError(t, readErr)
	assert.Equal(t, "old-db", string(data), "backup should contain original content")

	_, statErr = os.Stat(dbPath)
	require.NoError(t, statErr, "new database should exist")
}

func TestRunInitializeDatabase_AlreadyExists_Force_BackupCollision(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "wherehouse.db")
	dateSuffix := time.Now().Format("20060102")

	// Create original and primary backup (simulate collision).
	require.NoError(t, os.WriteFile(dbPath, []byte("old-db"), 0o600))

	primaryBackup := dbPath + ".backup." + dateSuffix
	require.NoError(t, os.WriteFile(primaryBackup, []byte("earlier-backup"), 0o600))

	cfg := makeTestConfig(dbPath)

	stdout, _, err := runDatabaseCmd(t, cfg, "--force")

	require.NoError(t, err)

	// Counter-suffixed backup should exist.
	collisionBackup := fmt.Sprintf("%s.backup.%s.1", dbPath, dateSuffix)
	_, statErr := os.Stat(collisionBackup)
	require.NoError(t, statErr, "collision backup file should exist at %s", collisionBackup)

	assert.Contains(t, stdout, collisionBackup)
}

func TestRunInitializeDatabase_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "wherehouse.db")
	cfg := makeTestConfig(dbPath)
	cfg.Output.DefaultFormat = "json"

	stdout, _, err := runDatabaseCmd(t, cfg)

	require.NoError(t, err)

	var result initResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, "initialized", result.Status)
	assert.Equal(t, dbPath, result.Path)
	assert.Empty(t, result.BackupPath)
}

func TestRunInitializeDatabase_JSONOutput_WithBackup(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "wherehouse.db")
	require.NoError(t, os.WriteFile(dbPath, []byte("old"), 0o600))

	cfg := makeTestConfig(dbPath)
	cfg.Output.DefaultFormat = "json"

	stdout, _, err := runDatabaseCmd(t, cfg, "--force")

	require.NoError(t, err)

	var result initResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, "initialized", result.Status)
	assert.Equal(t, dbPath, result.Path)
	assert.NotEmpty(t, result.BackupPath)

	dateSuffix := time.Now().Format("20060102")
	assert.Contains(t, result.BackupPath, ".backup."+dateSuffix)
}

func TestRunInitializeDatabase_NoConfig(t *testing.T) {
	resetForTesting()

	cmd := GetDatabaseCmd()
	cmd.SetContext(context.Background()) // no config in context

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "configuration not found in context")
}

func TestRunInitializeDatabase_QuietMode_SuppressesOutput(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "wherehouse.db")
	cfg := makeTestConfig(dbPath)
	cfg.Output.Quiet = 1

	stdout, _, err := runDatabaseCmd(t, cfg)

	require.NoError(t, err)
	assert.Empty(t, stdout, "quiet mode should suppress human-readable output")

	_, statErr := os.Stat(dbPath)
	assert.NoError(t, statErr, "database file should still be created in quiet mode")
}

func TestBackupDatabase_NoCollision(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	require.NoError(t, os.WriteFile(dbPath, []byte("data"), 0o600))

	dateSuffix := time.Now().Format("20060102")
	expected := dbPath + ".backup." + dateSuffix

	result, err := backupDatabase(dbPath)

	require.NoError(t, err)
	assert.Equal(t, expected, result)

	_, statErr := os.Stat(result)
	assert.NoError(t, statErr)
}

func TestBackupDatabase_WithCollision(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	require.NoError(t, os.WriteFile(dbPath, []byte("data"), 0o600))

	dateSuffix := time.Now().Format("20060102")
	primaryBackup := dbPath + ".backup." + dateSuffix
	require.NoError(t, os.WriteFile(primaryBackup, []byte("prev"), 0o600))

	expected := fmt.Sprintf("%s.backup.%s.1", dbPath, dateSuffix)

	result, err := backupDatabase(dbPath)

	require.NoError(t, err)
	assert.Equal(t, expected, result)

	_, statErr := os.Stat(result)
	assert.NoError(t, statErr)
}

func TestBackupDatabase_SourceMissing(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "nonexistent.db")

	_, err := backupDatabase(dbPath)

	require.Error(t, err)
}

func TestBackupDatabase_SlotExhaustion(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	require.NoError(t, os.WriteFile(dbPath, []byte("data"), 0o600))

	dateSuffix := time.Now().Format("20060102")

	// Create the primary backup and all 99 counter-suffixed backups.
	primaryBackup := dbPath + ".backup." + dateSuffix
	require.NoError(t, os.WriteFile(primaryBackup, []byte("prev"), 0o600))

	for i := 1; i <= 99; i++ {
		slot := fmt.Sprintf("%s.backup.%s.%d", dbPath, dateSuffix, i)
		require.NoError(t, os.WriteFile(slot, []byte("prev"), 0o600))
	}

	_, err := backupDatabase(dbPath)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many backups")
}
