package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/viper"
)

// loadConfig loads configuration from files using viper.
// If filepath is empty, searches default locations (global + local).
// If filepath is non-empty, loads only that file.
// Returns error if explicit filepath not found or if parsing fails.
//
// Supported environment variables (WHEREHOUSE_* prefix):
//   - WHEREHOUSE_CONFIG - custom config file path (overrides default file lookup)
//   - WHEREHOUSE_DATABASE_PATH - database file path
//   - WHEREHOUSE_USER_DEFAULT_IDENTITY - default user identity
//   - WHEREHOUSE_USER_OS_USERNAME_MAP - OS username mapping (not commonly used)
//   - WHEREHOUSE_OUTPUT_DEFAULT_FORMAT - default output format (human or json)
//   - WHEREHOUSE_OUTPUT_QUIET - quiet mode (true or false)
//
// Precedence order (highest to lowest):
//  1. Command-line flags (handled by caller)
//  2. Environment variables (WHEREHOUSE_*)
//  3. Local configuration file (./wherehouse.toml)
//  4. Global configuration file (~/.config/wherehouse/wherehouse.toml)
//  5. Built-in defaults
func loadConfig(fs afero.Fs, filepath string) (*Config, error) {
	v := viper.New()

	// Set filesystem for viper
	v.SetFs(fs)

	// Configure viper for environment variables with WHEREHOUSE_ prefix
	// This enables automatic environment variable binding where:
	//   - Config key "database.path" maps to WHEREHOUSE_DATABASE_PATH
	//   - Config key "user.default_identity" maps to WHEREHOUSE_USER_DEFAULT_IDENTITY
	v.SetEnvPrefix("WHEREHOUSE")
	v.AutomaticEnv()

	// Set key replacer for environment variables: "." → "_"
	// Example: "output.default_format" → "WHEREHOUSE_OUTPUT_DEFAULT_FORMAT"
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Set config file type
	v.SetConfigType("toml")

	var cfg Config

	if filepath != "" {
		// Explicit filepath provided - load only that file
		v.SetConfigFile(filepath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file %q: %w", filepath, err)
		}
	} else {
		// Load from default locations (global + local)
		if err := loadDefaultConfigs(v, fs); err != nil {
			return nil, err
		}
	}

	// Unmarshal into Config struct
	// Environment variables automatically override config file values due to AutomaticEnv
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	return &cfg, nil
}

// loadDefaultConfigs loads configuration from default locations.
// Loads global config first, then local config (local overrides global).
// Does not error if default config files are missing.
func loadDefaultConfigs(v *viper.Viper, fs afero.Fs) error {
	globalPath := GetGlobalConfigPath()
	localPath := GetLocalConfigPath()

	// Try to load global config
	if globalPath != "" {
		if exists, _ := afero.Exists(fs, globalPath); exists {
			v.SetConfigFile(globalPath)
			if err := v.ReadInConfig(); err != nil {
				return fmt.Errorf("failed to read global config %q: %w", globalPath, err)
			}
		}
	}

	// Try to load local config (merges with/overrides global)
	if exists, _ := afero.Exists(fs, localPath); exists {
		v.SetConfigFile(localPath)
		if err := v.MergeInConfig(); err != nil {
			return fmt.Errorf("failed to read local config %q: %w", localPath, err)
		}
	}

	return nil
}

// GetGlobalConfigPath returns the global configuration file path.
// Checks XDG_CONFIG_HOME first, falls back to ~/.config/wherehouse/wherehouse.toml.
func GetGlobalConfigPath() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "wherehouse", "wherehouse.toml")
}

// GetLocalConfigPath returns the local (project-specific) configuration file path.
// Always returns ./wherehouse.toml in the current directory.
func GetLocalConfigPath() string {
	return "./wherehouse.toml"
}
