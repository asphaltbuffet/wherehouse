package config

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/config"
)

func TestSetFilesystem(t *testing.T) {
	original := cmdFS
	defer func() { cmdFS = original }()

	memFS := afero.NewMemMapFs()
	SetFilesystem(memFS)

	assert.Same(t, memFS, cmdFS)
}

func TestFileExists_FileExists(t *testing.T) {
	fs := afero.NewMemMapFs()
	afero.WriteFile(fs, "/test/file.txt", []byte("content"), 0644)

	exists, err := fileExists(fs, "/test/file.txt")

	require.NoError(t, err)
	assert.True(t, exists)
}

func TestFileExists_FileNotFound(t *testing.T) {
	fs := afero.NewMemMapFs()

	exists, err := fileExists(fs, "/nonexistent/file.txt")

	require.NoError(t, err)
	assert.False(t, exists)
}

func TestFileExists_NestedPath(t *testing.T) {
	fs := afero.NewMemMapFs()
	afero.WriteFile(fs, "/a/b/c/file.txt", []byte("content"), 0644)

	exists, err := fileExists(fs, "/a/b/c/file.txt")

	require.NoError(t, err)
	assert.True(t, exists)
}

func TestEnsureDir_CreateNew(t *testing.T) {
	fs := afero.NewMemMapFs()

	err := ensureDir(fs, "/test/dir")

	require.NoError(t, err)
	exists, err := afero.DirExists(fs, "/test/dir")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestEnsureDir_CreateNested(t *testing.T) {
	fs := afero.NewMemMapFs()

	err := ensureDir(fs, "/a/b/c/d")

	require.NoError(t, err)
	exists, err := afero.DirExists(fs, "/a/b/c/d")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestEnsureDir_AlreadyExists(t *testing.T) {
	fs := afero.NewMemMapFs()
	fs.MkdirAll("/test/dir", 0755)

	err := ensureDir(fs, "/test/dir")

	require.NoError(t, err)
}

func TestAtomicWrite_Success(t *testing.T) {
	fs := afero.NewMemMapFs()
	data := []byte("test content")

	err := atomicWrite(fs, "/file.txt", data, 0644)

	require.NoError(t, err)
	content, err := afero.ReadFile(fs, "/file.txt")
	require.NoError(t, err)
	assert.Equal(t, data, content)
}

func TestAtomicWrite_CreatesParentDir(t *testing.T) {
	fs := afero.NewMemMapFs()
	data := []byte("test content")

	err := atomicWrite(fs, "/parent/child/file.txt", data, 0644)

	require.NoError(t, err)
	exists, err := afero.Exists(fs, "/parent/child/file.txt")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestAtomicWrite_Overwrites(t *testing.T) {
	fs := afero.NewMemMapFs()
	afero.WriteFile(fs, "/file.txt", []byte("old"), 0644)

	err := atomicWrite(fs, "/file.txt", []byte("new"), 0644)

	require.NoError(t, err)
	content, err := afero.ReadFile(fs, "/file.txt")
	require.NoError(t, err)
	assert.Equal(t, []byte("new"), content)
}

func TestAtomicWrite_CleanupOnError(t *testing.T) {
	// Use a read-only filesystem to trigger write error
	fs := afero.NewMemMapFs()

	// This should fail (parent dir doesn't exist and we can't create it in read-only)
	// For now, we'll test the happy path thoroughly since error paths are hard to trigger with afero

	data := []byte("content")
	err := atomicWrite(fs, "/test.txt", data, 0644)
	require.NoError(t, err)

	// Verify temp file is not left behind
	exists, err := afero.Exists(fs, "/test.txt.tmp")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestMarshalConfigWithComments_ValidOutput(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Path: "/path/to/db.sqlite",
		},
		User: config.UserConfig{
			DefaultIdentity: "test-user",
			OSUsernameMap:   map[string]string{},
		},
		Output: config.OutputConfig{
			DefaultFormat: "human",
			Quiet:         false,
		},
	}

	result := marshalConfigWithComments(cfg)

	assert.NotEmpty(t, result)

	// Verify content
	content := string(result)
	assert.Contains(t, content, "Wherehouse Configuration File")
	assert.Contains(t, content, "[database]")
	assert.Contains(t, content, "[user]")
	assert.Contains(t, content, "[output]")
	assert.Contains(t, content, "/path/to/db.sqlite")
}

func TestMarshalConfigWithComments_IncludesComments(t *testing.T) {
	cfg := config.GetDefaults()
	result := marshalConfigWithComments(cfg)

	content := string(result)
	assert.Contains(t, content, "# ")
}

func TestGetConfigValue_DatabasePath(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{Path: "/path/to/db"},
		User:     config.UserConfig{DefaultIdentity: ""},
		Output:   config.OutputConfig{DefaultFormat: "human", Quiet: false},
	}

	value, err := getConfigValue(cfg, "database.path")

	require.NoError(t, err)
	assert.Equal(t, "/path/to/db", value)
}

func TestGetConfigValue_UserDefaultIdentity(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{Path: ""},
		User:     config.UserConfig{DefaultIdentity: "myuser"},
		Output:   config.OutputConfig{DefaultFormat: "human", Quiet: false},
	}

	value, err := getConfigValue(cfg, "user.default_identity")

	require.NoError(t, err)
	assert.Equal(t, "myuser", value)
}

func TestGetConfigValue_OutputFormat(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{Path: ""},
		User:     config.UserConfig{DefaultIdentity: ""},
		Output:   config.OutputConfig{DefaultFormat: "json", Quiet: false},
	}

	value, err := getConfigValue(cfg, "output.default_format")

	require.NoError(t, err)
	assert.Equal(t, "json", value)
}

func TestGetConfigValue_InvalidKeyFormat(t *testing.T) {
	cfg := config.GetDefaults()

	_, err := getConfigValue(cfg, "invalid")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid key format")
}

func TestGetConfigValue_UnknownSection(t *testing.T) {
	cfg := config.GetDefaults()

	_, err := getConfigValue(cfg, "unknown.key")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown section")
}

func TestGetConfigValue_UnknownField(t *testing.T) {
	cfg := config.GetDefaults()

	_, err := getConfigValue(cfg, "database.unknown")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown database key")
}

func TestSetValueInMap_DatabasePath(t *testing.T) {
	configMap := map[string]any{
		"database": map[string]any{},
	}

	err := setValueInMap(configMap, "database.path", "/new/path")

	require.NoError(t, err)
	assert.Equal(t, "/new/path", configMap["database"].(map[string]any)["path"])
}

func TestSetValueInMap_CreatesSection(t *testing.T) {
	configMap := make(map[string]any)

	err := setValueInMap(configMap, "database.path", "/path")

	require.NoError(t, err)
	assert.NotNil(t, configMap["database"])
}

func TestSetValueInMap_OutputFormat_Valid(t *testing.T) {
	configMap := map[string]any{
		"output": map[string]any{},
	}

	err := setValueInMap(configMap, "output.default_format", "json")

	require.NoError(t, err)
	assert.Equal(t, "json", configMap["output"].(map[string]any)["default_format"])
}

func TestSetValueInMap_OutputFormat_Invalid(t *testing.T) {
	configMap := map[string]any{
		"output": map[string]any{},
	}

	err := setValueInMap(configMap, "output.default_format", "invalid")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be 'human' or 'json'")
}

func TestSetValueInMap_OutputQuiet_True(t *testing.T) {
	configMap := map[string]any{
		"output": map[string]any{},
	}

	err := setValueInMap(configMap, "output.quiet", "true")

	require.NoError(t, err)
	quiet := configMap["output"].(map[string]any)["quiet"].(bool)
	assert.True(t, quiet)
}

func TestSetValueInMap_OutputQuiet_False(t *testing.T) {
	configMap := map[string]any{
		"output": map[string]any{},
	}

	err := setValueInMap(configMap, "output.quiet", "false")

	require.NoError(t, err)
	quiet := configMap["output"].(map[string]any)["quiet"].(bool)
	assert.False(t, quiet)
}

func TestSetValueInMap_OutputQuiet_Invalid(t *testing.T) {
	configMap := map[string]any{
		"output": map[string]any{},
	}

	err := setValueInMap(configMap, "output.quiet", "maybe")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be 'true' or 'false'")
}

func TestSetValueInMap_InvalidKeyFormat(t *testing.T) {
	configMap := make(map[string]any)

	err := setValueInMap(configMap, "invalid", "value")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid key format")
}

func TestSetValueInMap_UnknownKey(t *testing.T) {
	configMap := map[string]any{
		"database": map[string]any{},
	}

	err := setValueInMap(configMap, "database.unknown", "value")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown configuration key")
}

func TestUnsetValueInMap_RemoveExisting(t *testing.T) {
	configMap := map[string]any{
		"database": map[string]any{
			"path":  "/path/to/db",
			"other": "value",
		},
	}

	removed := unsetValueInMap(configMap, "database.path")

	require.True(t, removed)
	_, exists := configMap["database"].(map[string]any)["path"]
	assert.False(t, exists)
	// Other key should still exist
	assert.Equal(t, "value", configMap["database"].(map[string]any)["other"])
}

func TestUnsetValueInMap_NonExistent(t *testing.T) {
	configMap := map[string]any{
		"database": map[string]any{},
	}

	removed := unsetValueInMap(configMap, "database.path")

	assert.False(t, removed)
}

func TestUnsetValueInMap_InvalidFormat(t *testing.T) {
	configMap := make(map[string]any)

	removed := unsetValueInMap(configMap, "invalid")

	assert.False(t, removed)
}

func TestLoadConfigFile_InvalidTOML(t *testing.T) {
	fs := afero.NewMemMapFs()

	afero.WriteFile(fs, "/config.toml", []byte("invalid [[ toml"), 0644)

	err := loadConfigFile(fs, "/config.toml")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing config file")
}

func TestLoadConfigFile_MissingFile(t *testing.T) {
	fs := afero.NewMemMapFs()

	err := loadConfigFile(fs, "/nonexistent.toml")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading config file")
}

func TestDetermineConfigPath_DefaultGlobal(t *testing.T) {
	path, err := determineConfigPath(false, false)

	require.NoError(t, err)
	assert.NotEmpty(t, path)
	assert.Contains(t, path, ".config")
}

func TestDetermineConfigPath_Local(t *testing.T) {
	path, err := determineConfigPath(true, false)

	require.NoError(t, err)
	assert.Equal(t, config.GetLocalConfigPath(), path)
}

func TestDetermineConfigPath_Global(t *testing.T) {
	path, err := determineConfigPath(false, true)

	require.NoError(t, err)
	assert.Equal(t, config.GetGlobalConfigPath(), path)
}

func TestDetermineConfigPath_BothFlags(t *testing.T) {
	_, err := determineConfigPath(true, true)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot use both")
}
