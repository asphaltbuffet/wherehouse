package config

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

var pathCmd *cobra.Command

// GetPathCmd returns the config path subcommand, which shows
// the locations of configuration files.
func GetPathCmd() *cobra.Command {
	if pathCmd != nil {
		return pathCmd
	}

	pathCmd = &cobra.Command{
		Use:   "path",
		Short: "Show configuration file locations",
		Long: `Show the locations of configuration files.

By default, shows active configuration file(s) that exist.
Use --all to show all possible configuration file locations.

Examples:
  wherehouse config path          Show active config files
  wherehouse config path --all    Show all possible locations`,
		RunE: runPath,
	}

	// Add flags specific to this command
	pathCmd.Flags().Bool("all", false, "show all possible locations")

	return pathCmd
}

// showAllPaths displays all possible configuration file locations regardless of existence.
func showAllPaths(out *cli.OutputWriter, customPath, expandedGlobal, expandedLocal string) {
	out.Info("Configuration file locations (in precedence order):")
	out.Println("")

	// Show custom path if set
	if customPath != "" {
		expanded, _ := config.ExpandPath(customPath)
		exists, _ := fileExists(cmdFS, expanded)
		if exists {
			out.Println(fmt.Sprintf("  --config flag: %s (exists) ✓", expanded))
		} else {
			out.Println(fmt.Sprintf("  --config flag: %s (not found)", expanded))
		}
	} else {
		out.Println("  --config flag: (not set)")
	}

	// Show local
	exists, _ := fileExists(cmdFS, expandedLocal)
	if exists {
		out.Println(fmt.Sprintf("  Local:  %s (exists) ✓", expandedLocal))
	} else {
		out.Println(fmt.Sprintf("  Local:  %s (not found)", expandedLocal))
	}

	// Show global
	exists, _ = fileExists(cmdFS, expandedGlobal)
	if exists {
		out.Println(fmt.Sprintf("  Global: %s (exists) ✓", expandedGlobal))
	} else {
		out.Println(fmt.Sprintf("  Global: %s (not found)", expandedGlobal))
	}
}

// showActivePaths displays only configuration files that currently exist.
func showActivePaths(out *cli.OutputWriter, customPath, expandedGlobal, expandedLocal string) {
	// Show only active files
	if customPath != "" {
		expanded, _ := config.ExpandPath(customPath)
		exists, _ := fileExists(cmdFS, expanded)
		if exists {
			out.Info("Active configuration file:")
			out.Println(fmt.Sprintf("  Custom: %s (exists)", expanded))
		} else {
			out.Info("Custom configuration file specified but not found:")
			out.Println(fmt.Sprintf("  %s", expanded))
		}
		return
	}

	hasActive := false
	out.Info("Active configuration files:")

	globalExists, _ := fileExists(cmdFS, expandedGlobal)
	if globalExists {
		out.Println(fmt.Sprintf("  Global: %s (exists)", expandedGlobal))
		hasActive = true
	}

	localExists, _ := fileExists(cmdFS, expandedLocal)
	if localExists {
		out.Println(fmt.Sprintf("  Local:  %s (exists)", expandedLocal))
		hasActive = true
	}

	if !hasActive {
		out.Println("  (none - using defaults)")
	}
}

func runPath(cmd *cobra.Command, _ []string) error {
	showAll, _ := cmd.Flags().GetBool("all")
	noConfig, _ := cmd.Flags().GetBool("no-config")

	// Create output writer
	jsonMode, _ := cmd.Flags().GetBool("json")
	quietMode, _ := cmd.Flags().GetBool("quiet")
	out := cli.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), jsonMode, quietMode)

	if noConfig {
		out.Info("No configuration files loaded (--no-config flag set)")
		return nil
	}

	globalPath := config.GetGlobalConfigPath()
	localPath := config.GetLocalConfigPath()

	expandedGlobal, _ := config.ExpandPath(globalPath)
	expandedLocal, _ := config.ExpandPath(localPath)

	// Access persistent flag from root command - cobra automatically inherits persistent flags
	customPath, _ := cmd.Flags().GetString("config")

	if showAll {
		showAllPaths(out, customPath, expandedGlobal, expandedLocal)
	} else {
		showActivePaths(out, customPath, expandedGlobal, expandedLocal)
	}

	return nil
}
