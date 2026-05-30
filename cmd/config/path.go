package config

import (
	"fmt"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

type pathStatus string

const (
	pathExists       pathStatus = "exists"
	pathNotFound     pathStatus = "not_found"
	pathInaccessible pathStatus = "inaccessible"
	pathNotSet       pathStatus = "not_set"
)

type pathEntry struct {
	Path   string     `json:"path"`
	Status pathStatus `json:"status"`
	Error  string     `json:"error,omitempty"`
}

type pathResult struct {
	Custom pathEntry `json:"custom"`
	Global pathEntry `json:"global"`
	Local  pathEntry `json:"local"`
}

func statPath(fs afero.Fs, path string) pathEntry {
	if path == "" {
		return pathEntry{Path: "", Status: pathNotSet}
	}
	exists, err := afero.Exists(fs, path)
	if err != nil {
		return pathEntry{Path: path, Status: pathInaccessible, Error: err.Error()}
	}
	if exists {
		return pathEntry{Path: path, Status: pathExists}
	}
	return pathEntry{Path: path, Status: pathNotFound}
}

// NewPathCmd returns the config path subcommand, which shows
// the locations of configuration files.
func NewPathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "path",
		Short: "Show configuration file locations",
		Long: `Show the locations of configuration files.

By default, shows active configuration file(s) that exist.
Use --all to show all possible configuration file locations.

Examples:
  wherehouse config path          # Show active config files
  wherehouse config path --all    # Show all possible locations`,
		RunE: runPath,
	}

	// Add flags specific to this command
	cmd.Flags().Bool("all", false, "show all possible locations")

	return cmd
}

func formatEntry(label, path string, e pathEntry) string {
	switch e.Status {
	case pathExists:
		return fmt.Sprintf("  %s: %s (exists) ✓", label, path)
	case pathNotFound:
		return fmt.Sprintf("  %s: %s (not found)", label, path)
	case pathInaccessible:
		return fmt.Sprintf("  %s: %s (inaccessible: %s)", label, path, e.Error)
	case pathNotSet:
		return fmt.Sprintf("  %s: (not set)", label)
	}
	return fmt.Sprintf("  %s: (unknown)", label)
}

func showAllPaths(out *cli.OutputWriter, result pathResult) {
	out.Info("Configuration file locations (in precedence order):")
	out.Println("")
	if result.Custom.Status == pathNotSet {
		out.Println("  --config flag: (not set)")
	} else {
		out.Println(formatEntry("--config flag", result.Custom.Path, result.Custom))
	}
	out.Println(formatEntry("Local ", result.Local.Path, result.Local))
	out.Println(formatEntry("Global", result.Global.Path, result.Global))
}

func showActivePaths(out *cli.OutputWriter, result pathResult) {
	if result.Custom.Status != pathNotSet {
		if result.Custom.Status == pathExists {
			out.Info("Active configuration file:")
			out.Println(fmt.Sprintf("  Custom: %s (exists)", result.Custom.Path))
		} else {
			out.Info("Custom configuration file specified but not found:")
			out.Println(fmt.Sprintf("  %s", result.Custom.Path))
		}
		return
	}

	hasActive := false
	out.Info("Active configuration files:")

	switch result.Global.Status {
	case pathExists:
		out.Println(fmt.Sprintf("  Global: %s (exists)", result.Global.Path))
		hasActive = true
	case pathInaccessible:
		out.Println(formatEntry("Global", result.Global.Path, result.Global))
	case pathNotFound, pathNotSet:
	}

	switch result.Local.Status {
	case pathExists:
		out.Println(fmt.Sprintf("  Local:  %s (exists)", result.Local.Path))
		hasActive = true
	case pathInaccessible:
		out.Println(formatEntry("Local ", result.Local.Path, result.Local))
	case pathNotFound, pathNotSet:
	}

	if !hasActive {
		out.Println("  (none - using defaults)")
	}
}

func buildPathResult(customPath, expandedGlobal, expandedLocal string) pathResult {
	var custom pathEntry
	if customPath != "" {
		expanded, _ := config.ExpandPath(customPath)
		custom = statPath(cmdFS, expanded)
	} else {
		custom = pathEntry{Status: pathNotSet}
	}
	return pathResult{
		Custom: custom,
		Global: statPath(cmdFS, expandedGlobal),
		Local:  statPath(cmdFS, expandedLocal),
	}
}

func runPath(cmd *cobra.Command, _ []string) error {
	showAll, _ := cmd.Flags().GetBool("all")
	noConfig, _ := cmd.Flags().GetBool("no-config")

	cfg := cli.MustGetConfig(cmd.Context())
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	if noConfig {
		out.Info("No configuration files loaded (--no-config flag set)")
		return nil
	}

	globalPath := config.GetGlobalConfigPath()
	localPath := config.GetLocalConfigPath()

	expandedGlobal, _ := config.ExpandPath(globalPath)
	expandedLocal, _ := config.ExpandPath(localPath)

	customPath, _ := cmd.Flags().GetString("config")

	result := buildPathResult(customPath, expandedGlobal, expandedLocal)

	if cfg.IsJSON() {
		return out.JSON(result)
	}

	if showAll {
		showAllPaths(out, result)
	} else {
		showActivePaths(out, result)
	}

	return nil
}
