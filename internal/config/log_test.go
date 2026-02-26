package config

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultLogPath_XDGSet verifies path resolution when XDG_STATE_HOME is set.
func TestDefaultLogPath_XDGSet(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/custom/state")
	t.Setenv("HOME", "/home/user")

	result := DefaultLogPath()

	assert.Equal(t, "/custom/state/wherehouse/wherehouse.log", result)
}

// TestDefaultLogPath_XDGFallback verifies fallback to HOME/.local/state when XDG_STATE_HOME is unset.
func TestDefaultLogPath_XDGFallback(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "/home/user")

	result := DefaultLogPath()

	assert.Equal(t, "/home/user/.local/state/wherehouse/wherehouse.log", result)
}

// TestDefaultLogPath_NeitherSet verifies fallback to relative path when both XDG_STATE_HOME and HOME are unset.
func TestDefaultLogPath_NeitherSet(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "")

	result := DefaultLogPath()

	// Should return a non-empty path, not panic
	assert.NotEmpty(t, result)
	// On Linux/BSD, should be relative path when no env vars set
	assert.True(t, filepath.IsAbs(result) || result == filepath.Join(".", "wherehouse.log"))
}

// TestGetLogPath_ConfigTakesPrecedence verifies that explicit config FilePath takes precedence.
func TestGetLogPath_ConfigTakesPrecedence(t *testing.T) {
	cfg := &Config{
		Logging: LoggingConfig{
			FilePath: "/explicit/path.log",
		},
	}
	t.Setenv("WHEREHOUSE_LOG_PATH", "/env/path.log")

	result, err := cfg.GetLogPath()

	require.NoError(t, err)
	assert.Equal(t, "/explicit/path.log", result)
}

// TestGetLogPath_EnvVarFallback verifies that WHEREHOUSE_LOG_PATH env var is used when config FilePath is empty.
func TestGetLogPath_EnvVarFallback(t *testing.T) {
	cfg := &Config{
		Logging: LoggingConfig{
			FilePath: "",
		},
	}
	t.Setenv("WHEREHOUSE_LOG_PATH", "/env/path.log")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "")

	result, err := cfg.GetLogPath()

	require.NoError(t, err)
	assert.Equal(t, "/env/path.log", result)
}

// TestGetLogPath_DefaultFallback verifies that DefaultLogPath is used when config and env var are both empty.
func TestGetLogPath_DefaultFallback(t *testing.T) {
	cfg := &Config{
		Logging: LoggingConfig{
			FilePath: "",
		},
	}
	t.Setenv("WHEREHOUSE_LOG_PATH", "")
	t.Setenv("XDG_STATE_HOME", "/custom/state")
	t.Setenv("HOME", "/home/user")

	result, err := cfg.GetLogPath()

	require.NoError(t, err)
	assert.Equal(t, "/custom/state/wherehouse/wherehouse.log", result)
}

// TestGetLogPath_TildeExpansion verifies that ~ in config FilePath is expanded correctly.
func TestGetLogPath_TildeExpansion(t *testing.T) {
	cfg := &Config{
		Logging: LoggingConfig{
			FilePath: "~/logs/test.log",
		},
	}

	result, err := cfg.GetLogPath()

	require.NoError(t, err)
	assert.NotEmpty(t, result)
	// Result should be absolute
	assert.True(t, filepath.IsAbs(result))
	// Result should contain logs/test.log
	assert.Contains(t, result, "logs")
	assert.True(t, strings.HasSuffix(result, "test.log"))
}

// TestGetLogPath_EnvVarExpansion verifies that environment variables in paths are expanded.
func TestGetLogPath_EnvVarExpansion(t *testing.T) {
	cfg := &Config{
		Logging: LoggingConfig{
			FilePath: "$HOME/logs/test.log",
		},
	}
	t.Setenv("HOME", "/home/user")

	result, err := cfg.GetLogPath()

	require.NoError(t, err)
	assert.NotEmpty(t, result)
	// Result should be absolute
	assert.True(t, filepath.IsAbs(result))
}
