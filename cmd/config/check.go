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
  wherehouse config check   # Validate all configuration files`,
		RunE: runCheck,
	}

	return checkCmd
}

func runCheck(cmd *cobra.Command, _ []string) error {
	cfg := cli.MustGetConfig(cmd.Context())
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	globalPath := config.GetGlobalConfigPath()
	localPath := config.GetLocalConfigPath()

	expandedGlobal, err := config.ExpandPath(globalPath)
	if err != nil {
		out.Error(fmt.Sprintf("failed to expand global config path %q: %v", globalPath, err))
		return fmt.Errorf("failed to expand global config path: %w", err)
	}

	expandedLocal, err := config.ExpandPath(localPath)
	if err != nil {
		out.Error(fmt.Sprintf("failed to expand local config path %q: %v", localPath, err))
		return fmt.Errorf("failed to expand local config path: %w", err)
	}

	hasErrors := false

	// Check global config
	globalExists, err := fileExists(cmdFS, expandedGlobal)
	if err != nil {
		out.Error(fmt.Sprintf("cannot access global config %s: %v", expandedGlobal, err))
		return fmt.Errorf("cannot access global config: %w", err)
	}
	if globalExists {
		err = config.Check(cmdFS, expandedGlobal)
		if err != nil {
			out.Error(fmt.Sprintf("Configuration invalid: %s", expandedGlobal))
			out.Println(fmt.Sprintf("  Error: %v", err))
			hasErrors = true
		} else {
			out.Success(fmt.Sprintf("Global config valid: %s", expandedGlobal))
		}
	}

	// Check local config
	localExists, err := fileExists(cmdFS, expandedLocal)
	if err != nil {
		out.Error(fmt.Sprintf("cannot access local config %s: %v", expandedLocal, err))
		return fmt.Errorf("cannot access local config: %w", err)
	}
	if localExists {
		err = config.Check(cmdFS, expandedLocal)
		if err != nil {
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
