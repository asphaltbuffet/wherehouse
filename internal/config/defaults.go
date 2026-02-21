package config

import (
	"os"
	"path/filepath"
)

// applyDefaults sets default values for any unspecified configuration fields.
// This ensures all required fields have sensible defaults.
func applyDefaults(cfg *Config) {
	// Database defaults
	if cfg.Database.Path == "" {
		cfg.Database.Path = getDefaultDatabasePath()
	}

	// User defaults - empty string means use OS username
	// OSUsernameMap defaults to empty map (no mappings)
	if cfg.User.OSUsernameMap == nil {
		cfg.User.OSUsernameMap = make(map[string]string)
	}

	// Output defaults
	if cfg.Output.DefaultFormat == "" {
		cfg.Output.DefaultFormat = "human"
	}
	// Quiet defaults to false (already zero value)
}

// getDefaultDatabasePath returns the default database file path.
// Uses ~/.wherehouse/inventory.db as the default location.
func getDefaultDatabasePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to relative path if home directory cannot be determined
		return "./wherehouse.db"
	}
	return filepath.Join(home, ".wherehouse", "inventory.db")
}

// GetDefaults returns a Config struct populated with default values.
// Useful for testing and documentation purposes.
func GetDefaults() *Config {
	cfg := &Config{}
	applyDefaults(cfg)
	return cfg
}
