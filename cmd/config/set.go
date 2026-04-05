package config

import (
	"errors"
	"fmt"

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
		Args: cobra.ExactArgs(2), //nolint:mnd // 2 is the exact number of required args: key and value
		RunE: runSet,
	}

	// Add flags specific to this command
	setCmd.Flags().Bool("local", false, "set in local config")

	return setCmd
}

func runSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]
	local, _ := cmd.Flags().GetBool("local")

	cfg := cli.MustGetConfig(cmd.Context())
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

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
	err = config.Set(cmdFS, expandedPath, key, value)
	if err != nil {
		out.Error(err.Error())
		return err
	}

	out.Success("Configuration updated")
	out.KeyValue(key, value)
	out.KeyValue("File", expandedPath)

	return nil
}
