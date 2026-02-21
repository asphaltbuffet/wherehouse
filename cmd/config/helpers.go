package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/afero"

	"github.com/asphaltbuffet/wherehouse/internal/config"
)

const (
	// configFilePerms is the file permission mode for config files (rw-r--r--).
	configFilePerms = 0o644
	// keyValueParts is the expected number of parts when splitting a key by ".".
	keyValueParts = 2
)

// cmdFS is the filesystem abstraction used by all config commands.
// By default it uses the OS filesystem, but can be injected with
// a different implementation (e.g., in-memory) for testing.
var cmdFS afero.Fs = afero.NewOsFs()

// SetFilesystem allows injecting a filesystem implementation for testing.
// This enables unit tests to use in-memory filesystems without touching
// the real filesystem.
func SetFilesystem(fs afero.Fs) {
	cmdFS = fs
}

// fileExists checks if a file exists and is accessible.
// Returns (true, nil) if the file exists.
// Returns (false, nil) if the file does not exist.
// Returns (false, err) if there was an error checking (e.g., permission denied).
func fileExists(fs afero.Fs, path string) (bool, error) {
	_, err := fs.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// ensureDir creates a directory and all parent directories if they don't exist.
// Permissions are set to 0755 (rwxr-xr-x).
func ensureDir(fs afero.Fs, path string) error {
	return fs.MkdirAll(path, 0755)
}

// atomicWrite writes data to a file atomically by writing to a temporary file
// first, then renaming it to the target path. This prevents corruption if the
// write operation is interrupted.
func atomicWrite(fs afero.Fs, path string, data []byte, _ os.FileMode) error {
	tempPath := path + ".tmp"

	// Write to temporary file
	if err := afero.WriteFile(fs, tempPath, data, configFilePerms); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	// Rename to final destination (atomic operation)
	if err := fs.Rename(tempPath, path); err != nil {
		// Clean up temp file on failure
		_ = fs.Remove(tempPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

// marshalConfigWithComments marshals a Config struct to TOML format with
// helpful comments explaining each section and field.
func marshalConfigWithComments(cfg *config.Config) []byte {
	var sb strings.Builder

	sb.WriteString("# Wherehouse Configuration File\n")
	sb.WriteString("# See documentation for more details\n\n")

	sb.WriteString("[database]\n")
	sb.WriteString("# Path to SQLite database file\n")
	sb.WriteString("# Supports ~ for home directory and environment variables\n")
	sb.WriteString(fmt.Sprintf("path = %q\n\n", cfg.Database.Path))

	sb.WriteString("[user]\n")
	sb.WriteString("# Default user identity for attribution\n")
	sb.WriteString("# Empty string means use OS username\n")
	if cfg.User.DefaultIdentity == "" {
		sb.WriteString("default_identity = \"\"\n")
	} else {
		sb.WriteString(fmt.Sprintf("default_identity = %q\n", cfg.User.DefaultIdentity))
	}
	sb.WriteString("\n# Map OS usernames to display names\n")
	sb.WriteString("# Example: os_username_map = { \"jdoe\" = \"John Doe\" }\n")
	sb.WriteString("os_username_map = {}\n\n")

	sb.WriteString("[output]\n")
	sb.WriteString("# Default output format (human or json)\n")
	sb.WriteString(fmt.Sprintf("default_format = %q\n", cfg.Output.DefaultFormat))
	sb.WriteString("\n# Enable quiet mode by default\n")
	sb.WriteString(fmt.Sprintf("quiet = %t\n", cfg.Output.Quiet))

	return []byte(sb.String())
}

// getConfigValue retrieves a configuration value by dot-separated key.
// Supported keys:
//   - database.path
//   - user.default_identity
//   - user.os_username_map
//   - output.default_format
//   - output.quiet
func getConfigValue(cfg *config.Config, key string) (any, error) {
	parts := strings.Split(key, ".")
	if len(parts) != keyValueParts {
		return nil, fmt.Errorf("invalid key format %q (expected section.key)", key)
	}

	section := parts[0]
	field := parts[1]

	switch section {
	case "database":
		switch field {
		case "path":
			return cfg.Database.Path, nil
		default:
			return nil, fmt.Errorf("unknown database key %q", field)
		}
	case "user":
		switch field {
		case "default_identity":
			return cfg.User.DefaultIdentity, nil
		case "os_username_map":
			return cfg.User.OSUsernameMap, nil
		default:
			return nil, fmt.Errorf("unknown user key %q", field)
		}
	case "output":
		switch field {
		case "default_format":
			return cfg.Output.DefaultFormat, nil
		case "quiet":
			return cfg.Output.Quiet, nil
		default:
			return nil, fmt.Errorf("unknown output key %q", field)
		}
	default:
		return nil, fmt.Errorf("unknown section %q", section)
	}
}

// setValueInMap sets a configuration value in a map by dot-separated key.
// The value is parsed and type-checked based on the key.
func setValueInMap(configMap map[string]any, key, value string) error {
	parts := strings.Split(key, ".")
	if len(parts) != keyValueParts {
		return fmt.Errorf("invalid key format %q (expected section.key)", key)
	}

	section := parts[0]
	field := parts[1]

	// Ensure section exists
	if _, ok := configMap[section]; !ok {
		configMap[section] = make(map[string]any)
	}

	sectionMap, ok := configMap[section].(map[string]any)
	if !ok {
		return fmt.Errorf("invalid section %q", section)
	}

	// Parse value based on field type
	var parsedValue any
	switch section + "." + field {
	case "database.path":
		parsedValue = value
	case "user.default_identity":
		parsedValue = value
	case "output.default_format":
		if value != "human" && value != "json" {
			return fmt.Errorf("output.default_format must be 'human' or 'json', got %q", value)
		}
		parsedValue = value
	case "output.quiet":
		switch value {
		case "true":
			parsedValue = true
		case "false":
			parsedValue = false
		default:
			return fmt.Errorf("output.quiet must be 'true' or 'false', got %q", value)
		}
	default:
		return fmt.Errorf("unknown configuration key %q", key)
	}

	sectionMap[field] = parsedValue
	return nil
}

// unsetValueInMap removes a configuration value from a map by dot-separated key.
// Returns true if the key was found and removed, false if it didn't exist.
func unsetValueInMap(configMap map[string]any, key string) bool {
	parts := strings.Split(key, ".")
	if len(parts) != keyValueParts {
		return false
	}

	section := parts[0]
	field := parts[1]

	sectionMap, ok := configMap[section].(map[string]any)
	if !ok {
		return false
	}

	if _, exists := sectionMap[field]; !exists {
		return false
	}

	delete(sectionMap, field)
	return true
}

// loadConfigFile loads and validates a configuration file from the filesystem.
func loadConfigFile(fs afero.Fs, path string) error {
	data, err := afero.ReadFile(fs, path)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	var cfg config.Config
	err = toml.Unmarshal(data, &cfg)
	if err != nil {
		return fmt.Errorf("parsing config file: %w", err)
	}

	err = config.Validate(&cfg)
	if err != nil {
		return fmt.Errorf("validating config: %w", err)
	}

	return nil
}

// determineConfigPath returns the appropriate config file path based on flags.
// If both local and global are false, returns the global config path.
func determineConfigPath(local, global bool) (string, error) {
	if local && global {
		return "", errors.New("cannot use both --local and --global flags")
	}

	if local {
		return config.GetLocalConfigPath(), nil
	}

	return config.GetGlobalConfigPath(), nil
}
