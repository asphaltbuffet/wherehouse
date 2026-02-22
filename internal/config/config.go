// Package config provides configuration loading and management for wherehouse.
// It supports loading from multiple sources with proper precedence handling.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

type configKeyType int

// ConfigKey provides a unique context key for config.
const ConfigKey configKeyType = iota

// Config represents the complete wherehouse configuration.
type Config struct {
	Database DatabaseConfig `mapstructure:"database"`
	User     UserConfig     `mapstructure:"user"`
	Output   OutputConfig   `mapstructure:"output"`
}

// DatabaseConfig holds database-related configuration.
type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

// UserConfig holds user identity configuration.
type UserConfig struct {
	DefaultIdentity string            `mapstructure:"default_identity"`
	OSUsernameMap   map[string]string `mapstructure:"os_username_map"`
}

// OutputConfig holds output formatting configuration.
type OutputConfig struct {
	DefaultFormat string `mapstructure:"default_format"`
	Quiet         bool   `mapstructure:"quiet"`
}

// New creates a Config instance with the given optional filepath.
// If filepath is empty, checks WHEREHOUSE_CONFIG environment variable.
// If neither is set, searches default locations (global + local).
// If filepath is non-empty, loads only that file.
// Returns error if explicit filepath not found or if validation fails.
//
// Environment variable WHEREHOUSE_CONFIG can specify custom config file path.
// This is checked when filepath parameter is empty.
func New(filepath string) (*Config, error) {
	return NewWithFS(afero.NewOsFs(), filepath)
}

// NewWithFS creates a Config instance with a custom filesystem.
// This is primarily used for testing with in-memory filesystems.
// If filepath is empty, checks WHEREHOUSE_CONFIG environment variable.
// If neither is set, searches default locations (global + local).
// If filepath is non-empty, loads only that file.
//
// Filepath resolution priority:
//  1. Explicit filepath parameter (from --config flag)
//  2. WHEREHOUSE_CONFIG environment variable
//  3. Default locations (global + local config files)
func NewWithFS(fs afero.Fs, filepath string) (*Config, error) {
	// If no explicit filepath provided, check WHEREHOUSE_CONFIG environment variable
	if filepath == "" {
		if envPath := os.Getenv("WHEREHOUSE_CONFIG"); envPath != "" {
			filepath = envPath
		}
	}

	// Load configuration from files using viper
	cfg, err := loadConfig(fs, filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Apply defaults
	applyDefaults(cfg)

	// Validate configuration
	if validationErr := validate(cfg); validationErr != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", validationErr)
	}

	return cfg, nil
}

// GetCurrentUsername returns the current OS username.
// This is used for determining the default identity when not configured.
func GetCurrentUsername() string {
	if username := os.Getenv("USER"); username != "" {
		return username
	}

	if username := os.Getenv("USERNAME"); username != "" {
		return username
	}

	return "unknown"
}

// ExpandPath expands ~ and environment variables in the given path.
// Returns absolute path or error if expansion fails.
// Only supports ~ or ~/ (not ~username).
func ExpandPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	// Expand tilde to home directory (only ~ or ~/, not ~username)
	if path == "~" || (len(path) > 1 && path[0] == '~' && path[1] == '/') {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot expand ~: %w", err)
		}

		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, path[2:])
		}
	} else if len(path) > 0 && path[0] == '~' {
		// Reject ~username patterns
		return "", fmt.Errorf("~username expansion not supported (path: %q)", path)
	}

	// Expand environment variables
	path = os.ExpandEnv(path)

	// Make absolute
	return filepath.Abs(path)
}
