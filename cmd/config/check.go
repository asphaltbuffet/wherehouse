package config

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

var checkCmd *cobra.Command

// GetCheckCmd returns the config check subcommand, which validates
// configuration files for errors.
func GetCheckCmd() *cobra.Command {
	if checkCmd != nil {
		return checkCmd
	}

	checkCmd = &cobra.Command{
		Use:   "check",
		Short: "Validate configuration files",
		Long: `Validate configuration files for errors.

Checks both global and local configuration files for:
  - Syntax errors (invalid TOML)
  - Validation errors (invalid values)
  - Missing required fields

Examples:
  wherehouse config check`,
		RunE: runCheck,
	}

	return checkCmd
}

func runCheck(cmd *cobra.Command, _ []string) error {
	// Create output writer
	jsonMode, _ := cmd.Flags().GetBool("json")
	quietMode, _ := cmd.Flags().GetBool("quiet")
	out := cli.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), jsonMode, quietMode)

	globalPath := config.GetGlobalConfigPath()
	localPath := config.GetLocalConfigPath()

	expandedGlobal, _ := config.ExpandPath(globalPath)
	expandedLocal, _ := config.ExpandPath(localPath)

	hasErrors := false

	// Check global config
	globalExists, _ := fileExists(cmdFS, expandedGlobal)
	if globalExists {
		if err := loadConfigFile(cmdFS, expandedGlobal); err != nil {
			out.Error(fmt.Sprintf("Configuration invalid: %s", expandedGlobal))
			out.Println(fmt.Sprintf("  Error: %v", err))
			hasErrors = true
		} else {
			out.Success(fmt.Sprintf("Global config valid: %s", expandedGlobal))
		}
	}

	// Check local config
	localExists, _ := fileExists(cmdFS, expandedLocal)
	if localExists {
		if err := loadConfigFile(cmdFS, expandedLocal); err != nil {
			out.Error(fmt.Sprintf("Configuration invalid: %s", expandedLocal))
			out.Println(fmt.Sprintf("  Error: %v", err))
			hasErrors = true
		} else {
			out.Success(fmt.Sprintf("Local config valid: %s", expandedLocal))
		}
	}

	if hasErrors {
		return errors.New("configuration validation failed")
	}

	if !globalExists && !localExists {
		out.Info("No configuration files found (will use defaults)")
	}

	return nil
}
