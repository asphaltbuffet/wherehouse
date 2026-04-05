package config

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

var initCmd *cobra.Command

// GetInitCmd returns the config init subcommand, which creates
// a new configuration file with default values.
func GetInitCmd() *cobra.Command {
	if initCmd != nil {
		return initCmd
	}

	initCmd = &cobra.Command{
		Use:   "init",
		Short: "Create a new configuration file",
		Long: `Create a new configuration file with default values.

By default, creates a global configuration file at ~/.config/wherehouse/wherehouse.toml.
Use --local to create a project-specific configuration file at ./wherehouse.toml.
Use --force to overwrite an existing file.

Examples:
  wherehouse config init              Create global config
  wherehouse config init --local      Create local config
  wherehouse config init --force      Overwrite existing global config
  wherehouse --config custom.toml config init  Create config at custom path`,
		RunE: runInit,
	}

	// Add flags specific to this command
	initCmd.Flags().Bool("local", false, "create local config (./wherehouse.toml)")
	initCmd.Flags().BoolP("force", "f", false, "overwrite existing file")

	return initCmd
}

func runInit(cmd *cobra.Command, _ []string) error {
	local, _ := cmd.Flags().GetBool("local")
	force, _ := cmd.Flags().GetBool("force")
	// Access persistent flag from root command - cobra automatically inherits persistent flags
	customPath, _ := cmd.Flags().GetString("config")

	cfg := cli.MustGetConfig(cmd.Context())
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	// Determine target path
	var targetPath string
	switch {
	case customPath != "":
		targetPath = customPath
	case local:
		targetPath = config.GetLocalConfigPath()
	default:
		targetPath = config.GetGlobalConfigPath()
	}

	// Expand path
	expandedPath, err := config.ExpandPath(targetPath)
	if err != nil {
		out.Error(fmt.Sprintf("invalid path %q: %v", targetPath, err))
		return fmt.Errorf("invalid path %q: %w", targetPath, err)
	}

	// Write default config (handles exists check, dir creation, and write atomically)
	err = config.WriteDefault(cmdFS, expandedPath, force)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			out.Error(err.Error())
			out.Info("Use --force to overwrite")
		} else {
			out.Error(fmt.Sprintf("failed to write configuration: %v", err))
		}
		return err
	}

	// Report success
	if force {
		out.Success("Configuration file created (overwritten)")
		out.KeyValue("Path", expandedPath)
	} else {
		out.Success("Configuration file created")
		out.KeyValue("Path", expandedPath)
	}

	return nil
}
