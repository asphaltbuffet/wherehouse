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

const (
	outputFormatJSON  = "json"
	outputFormatHuman = "human"
)

// Config represents the complete wherehouse configuration.
type Config struct {
	Database DatabaseConfig `mapstructure:"database" toml:"database"`
	Logging  LoggingConfig  `mapstructure:"logging"  toml:"logging"`
	User     UserConfig     `mapstructure:"user"     toml:"user"`
	Output   OutputConfig   `mapstructure:"output"   toml:"output"`
}

// LoggingConfig holds logging-related configuration.
type LoggingConfig struct {
	// FilePath is the absolute path to the log file.
	// Empty string causes logging.Init() to use DefaultLogPath().
	FilePath string `mapstructure:"file_path" toml:"file_path"`

	// Level is the minimum log level to record.
	// Valid values (case-insensitive): "debug", "info", "warn", "warning", "error".
	// Any unrecognized value defaults to slog.LevelWarn.
	Level string `mapstructure:"level" toml:"level"`

	// MaxSizeMB is the maximum log file size in megabytes before rotation.
	// 0 (default) disables rotation and uses a plain append-mode file.
	MaxSizeMB int `mapstructure:"max_size_mb" toml:"max_size_mb"`

	// MaxBackups is the number of old rotated log files to retain.
	// Only meaningful when MaxSizeMB > 0. If 0 when rotation is active,
	// Init() defaults this to 3.
	MaxBackups int `mapstructure:"max_backups" toml:"max_backups"`
}

// DatabaseConfig holds database-related configuration.
type DatabaseConfig struct {
	Path string `mapstructure:"path" toml:"path"`
}

// UserConfig holds user identity configuration.
type UserConfig struct {
	DefaultIdentity string            `mapstructure:"default_identity" toml:"default_identity"`
	OSUsernameMap   map[string]string `mapstructure:"os_username_map"  toml:"os_username_map"`
}

// OutputConfig holds output formatting configuration.
type OutputConfig struct {
	DefaultFormat string `mapstructure:"default_format" toml:"default_format"`
	Quiet         int    `mapstructure:"quiet"          toml:"quiet"`
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

// IsQuiet returns true if quiet mode is enabled (at any level).
// Corresponds to the user passing -q or setting output.quiet >= 1 in config.
func (c *Config) IsQuiet() bool {
	return c.Output.Quiet > 0
}

// QuietLevel returns the quiet suppression level.
// 0 = normal output, 1 = minimal (-q), 2+ = silent (-qq).
func (c *Config) QuietLevel() int {
	return c.Output.Quiet
}

// IsJSON returns true if JSON output format is active.
// Corresponds to the user passing --json or setting output.default_format = "json" in config.
func (c *Config) IsJSON() bool {
	return c.Output.DefaultFormat == outputFormatJSON
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
