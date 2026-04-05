package config

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
)

// newViperForFile creates a configured viper instance for a single file.
// Sets the afero filesystem and config file path. Does not read the file.
func newViperForFile(fs afero.Fs, path string) *viper.Viper {
	v := viper.New()
	v.SetFs(fs)
	v.SetConfigFile(path)
	v.SetConfigType("toml")
	return v
}

// WriteDefault writes a new config file at path with all default values.
// Uses viper to produce TOML output - no comments, but all keys present.
// Returns error if file already exists and force is false.
// Creates parent directories as needed.
//
// Output format: clean TOML with four sections [database], [logging], [user], [output].
// All keys are written with their default values. Comments are not included.
// Users who want comments should run 'config edit' to annotate manually.
func WriteDefault(fs afero.Fs, path string, force bool) error {
	exists, existsErr := afero.Exists(fs, path)
	if existsErr != nil {
		return fmt.Errorf("checking config file: %w", existsErr)
	}

	if exists && !force {
		return fmt.Errorf("configuration file already exists: %s", path)
	}

	err := fs.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	v := newViperForFile(fs, path)
	defaults := GetDefaults()

	// Register all defaults with viper so AllSettings() includes them
	v.SetDefault("database.path", defaults.Database.Path)
	v.SetDefault("logging.file_path", defaults.Logging.FilePath)
	v.SetDefault("logging.level", defaults.Logging.Level)
	v.SetDefault("logging.max_size_mb", defaults.Logging.MaxSizeMB)
	v.SetDefault("logging.max_backups", defaults.Logging.MaxBackups)
	v.SetDefault("user.default_identity", defaults.User.DefaultIdentity)
	v.SetDefault("user.os_username_map", defaults.User.OSUsernameMap)
	v.SetDefault("output.default_format", defaults.Output.DefaultFormat)
	v.SetDefault("output.quiet", defaults.Output.Quiet)

	return v.WriteConfigAs(path)
}

// Set updates a single key-value pair in the config file at path.
// Reads the existing config via viper, sets the override, validates the full
// merged config, then rewrites via viper.WriteConfigAs.
//
// Returns error if key is unknown, value fails type conversion, file cannot be
// read/written, or the resulting config fails validation.
func Set(fs afero.Fs, path string, key string, value string) error {
	// Validate key is known and parse/type-check value
	parsedValue, parseErr := parseConfigValue(key, value)
	if parseErr != nil {
		return parseErr
	}

	v := newViperForFile(fs, path)
	err := v.ReadInConfig()
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	v.Set(key, parsedValue)

	// Validate the full merged config before writing
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	applyDefaults(&cfg)
	if validateErr := validate(&cfg); validateErr != nil {
		return fmt.Errorf("configuration validation failed: %w", validateErr)
	}

	return v.WriteConfigAs(path)
}

// Check validates the config file at path.
// Reads the file directly (not via viper) to test the raw TOML parse path.
// Returns nil if the file is valid TOML and passes all validation constraints.
func Check(fs afero.Fs, path string) error {
	data, err := afero.ReadFile(fs, path)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parsing config file: %w", err)
	}

	applyDefaults(&cfg)

	if validateErr := validate(&cfg); validateErr != nil {
		return fmt.Errorf("validating config: %w", validateErr)
	}

	return nil
}

// GetValue returns the value of a dot-separated config key from cfg.
// Supports all config keys including logging.* and user.os_username_map.
// Returns (value, nil) on success or (nil, error) for unknown keys.
func GetValue(cfg *Config, key string) (any, error) {
	const keyParts = 2
	parts := strings.SplitN(key, ".", keyParts)
	if len(parts) != keyParts {
		return nil, fmt.Errorf("invalid key format %q (expected section.key)", key)
	}

	section, field := parts[0], parts[1]

	switch section {
	case "database":
		if field == "path" {
			return cfg.Database.Path, nil
		}
	case "logging":
		switch field {
		case "file_path":
			return cfg.Logging.FilePath, nil
		case "level":
			return cfg.Logging.Level, nil
		case "max_size_mb":
			return cfg.Logging.MaxSizeMB, nil
		case "max_backups":
			return cfg.Logging.MaxBackups, nil
		}
	case "user":
		switch field {
		case "default_identity":
			return cfg.User.DefaultIdentity, nil
		case "os_username_map":
			return cfg.User.OSUsernameMap, nil
		}
	case "output":
		switch field {
		case "default_format":
			return cfg.Output.DefaultFormat, nil
		case "quiet":
			return cfg.Output.Quiet, nil
		}
	}

	return nil, fmt.Errorf("unknown configuration key %q", key)
}

// parseConfigValue validates and parses a string value for the given config key.
// Returns the typed value ready for viper.Set() or an error if invalid.
// This is the single source of truth for type coercion and per-key validation.
func parseConfigValue(key, value string) (any, error) {
	switch key {
	case "database.path":
		return value, nil
	case "logging.file_path":
		return value, nil
	case "logging.level":
		normalized := strings.ToLower(value)
		valid := map[string]bool{"debug": true, "info": true, "warn": true, "warning": true, "error": true}
		if !valid[normalized] {
			return nil, fmt.Errorf("logging.level must be one of [debug, info, warn, warning, error], got %q", value)
		}
		return normalized, nil
	case "logging.max_size_mb":
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("logging.max_size_mb must be a non-negative integer, got %q", value)
		}
		return n, nil
	case "logging.max_backups":
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("logging.max_backups must be a non-negative integer, got %q", value)
		}
		return n, nil
	case "user.default_identity":
		return value, nil
	case "output.default_format":
		if value != outputFormatHuman && value != outputFormatJSON {
			return nil, fmt.Errorf("output.default_format must be 'human' or 'json', got %q", value)
		}
		return value, nil
	case "output.quiet":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("output.quiet must be 'true' or 'false', got %q", value)
		}
		return b, nil
	default:
		return nil, fmt.Errorf("unknown configuration key %q", key)
	}
}
