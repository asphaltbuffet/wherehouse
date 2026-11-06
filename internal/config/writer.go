package config

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/afero"
)

// WriteDefault writes a new config file at path containing every key with its
// default value, encoded directly from GetDefaults(). Comments are not included.
// Returns error if file already exists and force is false.
// Creates parent directories as needed.
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

	data, err := toml.Marshal(GetDefaults())
	if err != nil {
		return fmt.Errorf("encoding default config: %w", err)
	}

	// go-toml emits an empty table (user.os_username_map) with no blank line
	// before the next header; add one so every table is visually separated.
	data = bytes.ReplaceAll(data, []byte("]\n["), []byte("]\n\n["))

	err = afero.WriteFile(fs, path, data, 0o644)
	if err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
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
