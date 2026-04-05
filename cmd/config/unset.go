package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

var unsetCmd *cobra.Command

// GetUnsetCmd returns the config unset subcommand, which removes
// a configuration value from the configuration file.
func GetUnsetCmd() *cobra.Command {
	if unsetCmd != nil {
		return unsetCmd
	}

	unsetCmd = &cobra.Command{
		Use:   "unset <key>",
		Short: "Remove a configuration value",
		Long: `Remove a configuration value from the configuration file.

By default, removes from whichever file contains the key (local first, then global).
Use --local to remove only from local configuration.
Use --global to remove only from global configuration.

Examples:
  wherehouse config unset user.default_identity
  wherehouse config unset --local output.default_format
  wherehouse config unset --global database.path`,
		Args: cobra.ExactArgs(1),
		RunE: runUnset,
	}

	// Add flags specific to this command
	unsetCmd.Flags().Bool("local", false, "unset only from local config")
	unsetCmd.Flags().Bool("global", false, "unset only from global config")

	return unsetCmd
}

// unsetFromFile removes a configuration value from a single file.
// Returns (true, nil) if the key was found and removed successfully.
// Returns (false, nil) if the file doesn't exist or key not found (when strictErrors=false).
// Returns (false, err) on errors (when strictErrors=true) or unrecoverable errors.
func unsetFromFile(
	fs afero.Fs,
	path string,
	key string,
	strictErrors bool,
	out *cli.OutputWriter,
) (bool, error) {
	expandedPath, err := config.ExpandPath(path)
	if err != nil {
		if strictErrors {
			out.Error(fmt.Sprintf("invalid path %q: %v", path, err))
			return false, fmt.Errorf("invalid path %q: %w", path, err)
		}
		return false, nil
	}

	// Check if file exists
	exists, err := fileExists(fs, expandedPath)
	if err != nil {
		if strictErrors {
			out.Error(fmt.Sprintf("checking config file: %v", err))
			return false, fmt.Errorf("checking config file: %w", err)
		}
		return false, nil
	}

	if !exists {
		if strictErrors {
			out.Error(fmt.Sprintf("configuration file not found: %s", expandedPath))
			return false, fmt.Errorf("configuration file not found: %s", expandedPath)
		}
		return false, nil
	}

	// Load existing config
	data, err := afero.ReadFile(fs, expandedPath)
	if err != nil {
		if strictErrors {
			out.Error(fmt.Sprintf("failed to read config file: %v", err))
			return false, fmt.Errorf("failed to read config file: %w", err)
		}
		return false, nil
	}

	// Parse as map
	var configMap map[string]any
	err = toml.Unmarshal(data, &configMap)
	if err != nil {
		if strictErrors {
			out.Error(fmt.Sprintf("failed to parse config file: %v", err))
			return false, fmt.Errorf("failed to parse config file: %w", err)
		}
		return false, nil
	}

	// Try to unset the value
	if !unsetValueInMap(configMap, key) {
		return false, nil
	}

	// Marshal back to TOML
	newData, err := toml.Marshal(configMap)
	if err != nil {
		out.Error(fmt.Sprintf("failed to marshal configuration: %v", err))
		return false, fmt.Errorf("failed to marshal configuration: %w", err)
	}

	// Write atomically
	err = atomicWrite(fs, expandedPath, newData, configFilePerms)
	if err != nil {
		out.Error(fmt.Sprintf("failed to write configuration: %v", err))
		return false, fmt.Errorf("failed to write configuration: %w", err)
	}

	out.Success("Configuration updated")
	out.Info(fmt.Sprintf("Removed: %s", key))
	out.KeyValue("File", expandedPath)

	// Determine what happens next
	if strings.HasPrefix(path, "./") {
		out.Info("Will use value from global config (if set) or default behavior")
	} else {
		out.Info("Will use default behavior")
	}

	return true, nil
}

func runUnset(cmd *cobra.Command, args []string) error {
	key := args[0]
	localFlag, _ := cmd.Flags().GetBool("local")
	globalFlag, _ := cmd.Flags().GetBool("global")

	// Create output writer
	jsonMode, _ := cmd.Flags().GetBool("json")
	quietMode, _ := cmd.Flags().GetBool("quiet")
	out := cli.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), jsonMode, quietMode)

	// Determine which file(s) to check
	var targetPaths []string
	if localFlag && globalFlag {
		out.Error("cannot use both --local and --global flags")
		return errors.New("cannot use both --local and --global flags")
	}

	switch {
	case localFlag:
		targetPaths = []string{config.GetLocalConfigPath()}
	case globalFlag:
		targetPaths = []string{config.GetGlobalConfigPath()}
	default:
		// Default: check local first, then global
		targetPaths = []string{config.GetLocalConfigPath(), config.GetGlobalConfigPath()}
	}

	// Try each path
	for _, targetPath := range targetPaths {
		found, err := unsetFromFile(cmdFS, targetPath, key, localFlag || globalFlag, out)
		if err != nil {
			// If a specific file was targeted, return error
			if localFlag || globalFlag {
				return err
			}
			continue
		}
		if found {
			return nil
		}
	}

	out.Error(fmt.Sprintf("key %q not found in configuration", key))
	return fmt.Errorf("key %q not found in configuration", key)
}
