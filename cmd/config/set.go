package config

import (
	"errors"
	"fmt"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

var setCmd *cobra.Command

// GetSetCmd returns the config set subcommand, which sets
// a configuration value in the configuration file.
func GetSetCmd() *cobra.Command {
	if setCmd != nil {
		return setCmd
	}

	setCmd = &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long: `Set a configuration value in the configuration file.

By default, sets the value in the global configuration file.
Use --local to set in the local (project-specific) configuration file.

The configuration file must already exist (use 'config init' to create it).

Examples:
  wherehouse config set database.path /custom/inventory.db
  wherehouse config set --local output.default_format json
  wherehouse config set user.default_identity alice`,
		Args: cobra.ExactArgs(keyValueParts),
		RunE: runSet,
	}

	// Add flags specific to this command
	setCmd.Flags().Bool("local", false, "set in local config")

	return setCmd
}

// updateConfigValue reads a config file, updates a key-value pair, validates, and writes it back.
func updateConfigValue(fs afero.Fs, path, key, value string) error {
	// Load existing config
	data, err := afero.ReadFile(fs, path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse as map for flexible key setting
	var configMap map[string]any
	err = toml.Unmarshal(data, &configMap)
	if err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set the value in the map
	err = setValueInMap(configMap, key, value)
	if err != nil {
		return err
	}

	// Marshal back to TOML
	newData, err := toml.Marshal(configMap)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	// Validate by unmarshaling into Config struct
	var testCfg config.Config
	err = toml.Unmarshal(newData, &testCfg)
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Perform full validation of the configuration
	err = config.Validate(&testCfg)
	if err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Write atomically
	err = atomicWrite(fs, path, newData, configFilePerms)
	if err != nil {
		return fmt.Errorf("failed to write configuration: %w", err)
	}

	return nil
}

func runSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]
	local, _ := cmd.Flags().GetBool("local")

	// Create output writer
	jsonMode, _ := cmd.Flags().GetBool("json")
	quietMode, _ := cmd.Flags().GetBool("quiet")
	out := cli.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), jsonMode, quietMode)

	// Determine target file
	var targetPath string
	if local {
		targetPath = config.GetLocalConfigPath()
	} else {
		targetPath = config.GetGlobalConfigPath()
	}

	// Expand path
	expandedPath, err := config.ExpandPath(targetPath)
	if err != nil {
		out.Error(fmt.Sprintf("invalid path %q: %v", targetPath, err))
		return fmt.Errorf("invalid path %q: %w", targetPath, err)
	}

	// Check if file exists
	exists, err := fileExists(cmdFS, expandedPath)
	if err != nil {
		out.Error(fmt.Sprintf("checking config file: %v", err))
		return fmt.Errorf("checking config file: %w", err)
	}

	if !exists {
		if local {
			out.Error("no local configuration file found")
			out.Info("Run 'wherehouse config init --local' to create one")
			return errors.New("no local configuration file found")
		}
		out.Error("no global configuration file found")
		out.Info("Run 'wherehouse config init' to create one")
		out.Info("Or use 'wherehouse config set --local ...' for project-specific config")
		return errors.New("no global configuration file found")
	}

	// Update the configuration value
	err = updateConfigValue(cmdFS, expandedPath, key, value)
	if err != nil {
		out.Error(err.Error())
		return err
	}

	out.Success("Configuration updated")
	out.KeyValue(key, value)
	out.KeyValue("File", expandedPath)

	return nil
}
