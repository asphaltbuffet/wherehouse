package config

import (
	"errors"
	"fmt"
)

// Validate checks the configuration for errors.
// Returns error if any validation constraint is violated.
// This is exported for use by CLI commands that modify config files.
func Validate(cfg *Config) error {
	return validate(cfg)
}

// validate checks the configuration for errors (internal).
// Returns error if any validation constraint is violated.
func validate(cfg *Config) error {
	if err := validateDatabase(&cfg.Database); err != nil {
		return fmt.Errorf("database configuration: %w", err)
	}

	if err := validateUser(&cfg.User); err != nil {
		return fmt.Errorf("user configuration: %w", err)
	}

	if err := validateOutput(&cfg.Output); err != nil {
		return fmt.Errorf("output configuration: %w", err)
	}

	return nil
}

// validateDatabase validates database configuration.
func validateDatabase(cfg *DatabaseConfig) error {
	if cfg.Path == "" {
		return errors.New("path is required")
	}

	return nil
}

// validateUser validates user configuration.
func validateUser(_ *UserConfig) error {
	// default_identity can be empty (means use OS username)
	// os_username_map can be empty (no mappings)
	return nil
}

// validateOutput validates output configuration.
func validateOutput(cfg *OutputConfig) error {
	// Validate default_format is one of allowed values
	validFormats := map[string]bool{
		outputFormatHuman: true,
		outputFormatJSON:  true,
	}

	if !validFormats[cfg.DefaultFormat] {
		return fmt.Errorf("default_format must be one of [human, json], got %q", cfg.DefaultFormat)
	}

	return nil
}
