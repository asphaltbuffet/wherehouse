package config

import (
	"fmt"
	"path/filepath"

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
	v.SetDefault("web.actor_header", defaults.Web.ActorHeader)

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

	err = toml.Unmarshal(data, &cfg)
	if err != nil {
		return fmt.Errorf("parsing config file: %w", err)
	}

	applyDefaults(&cfg)

	err = validate(&cfg)
	if err != nil {
		return fmt.Errorf("validating config: %w", err)
	}

	return nil
}
