package config

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

var editCmd *cobra.Command

// GetEditCmd returns the config edit subcommand, which opens
// the configuration file in the user's default editor.
func GetEditCmd() *cobra.Command {
	if editCmd != nil {
		return editCmd
	}

	editCmd = &cobra.Command{
		Use:   "edit",
		Short: "Edit configuration file in $EDITOR",
		Long: `Open the configuration file in your default editor ($EDITOR).

By default, edits the global configuration file.
Use --local to edit the local configuration file.
Use --global to explicitly edit the global configuration file.

The configuration is validated after editing.

Examples:
  wherehouse config edit           # Edit global config
  wherehouse config edit --local   # Edit local config`,
		RunE: runEdit,
	}

	// Add flags specific to this command
	editCmd.Flags().Bool("local", false, "edit local config")
	editCmd.Flags().Bool("global", false, "edit global config")

	return editCmd
}

func runEdit(cmd *cobra.Command, _ []string) error {
	local, _ := cmd.Flags().GetBool("local")
	global, _ := cmd.Flags().GetBool("global")

	cfg := cli.MustGetConfig(cmd.Context())
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	if local && global {
		out.Error("cannot use both --local and --global flags")
		return errors.New("cannot use both --local and --global flags")
	}

	// Determine target file
	var targetPath string
	if local {
		targetPath = config.GetLocalConfigPath()
	} else {
		// Default to global
		targetPath = config.GetGlobalConfigPath()
	}

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
		return errors.New("no global configuration file found")
	}

	// Get editor from environment
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi" // Fallback to vi
	}

	out.Info(fmt.Sprintf("Opening %s in %s...", expandedPath, editor))

	// Open editor
	editorCmd := exec.CommandContext(cmd.Context(), editor, expandedPath)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr

	err = editorCmd.Run()
	if err != nil {
		out.Error(fmt.Sprintf("editor failed: %v", err))
		return fmt.Errorf("editor failed: %w", err)
	}

	// Validate after editing
	err = config.Check(cmdFS, expandedPath)
	if err != nil {
		out.Warning(fmt.Sprintf("Configuration validation failed: %v", err))
		out.Info("Please fix the errors and run 'wherehouse config check' to verify.")
		// Don't fail - user can fix manually
		return nil
	}

	out.Success("Configuration validated successfully")
	return nil
}
