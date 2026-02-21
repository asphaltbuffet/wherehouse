package config

import (
	"errors"
	"fmt"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

var getCmd *cobra.Command

// GetGetCmd returns the config get subcommand, which displays
// configuration values from the merged configuration.
func GetGetCmd() *cobra.Command {
	if getCmd != nil {
		return getCmd
	}

	getCmd = &cobra.Command{
		Use:   "get [key]",
		Short: "Display configuration values",
		Long: `Display configuration values (merged from all sources).

Without arguments, shows all configuration in TOML format.
With a key argument, shows just that value.
Use --json for machine-readable output.
Use --sources to show where each value comes from.

Examples:
  wherehouse config get                    Show all configuration
  wherehouse config get database.path      Show specific value
  wherehouse config get --json             JSON output
  wherehouse config get --sources          Show value sources`,
		Args: cobra.MaximumNArgs(1),
		RunE: runGet,
	}

	// Add flags specific to this command
	getCmd.Flags().Bool("json", false, "output in JSON format")
	getCmd.Flags().Bool("sources", false, "show where each value comes from")

	return getCmd
}

func runGet(cmd *cobra.Command, args []string) error {
	jsonOutput, _ := cmd.Flags().GetBool("json")
	showSources, _ := cmd.Flags().GetBool("sources")
	quietMode, _ := cmd.Flags().GetBool("quiet")

	// Create output writer
	out := cli.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), jsonOutput, quietMode)

	// Get the global config from command context
	cfg := cmd.Context().Value("config")
	if cfg == nil {
		return errors.New("configuration not loaded")
	}
	globalConfig, ok := cfg.(*config.Config)
	if !ok {
		return errors.New("invalid configuration type in context")
	}

	// If specific key requested
	if len(args) == 1 {
		key := args[0]
		value, err := getConfigValue(globalConfig, key)
		if err != nil {
			out.Error(err.Error())
			return err
		}

		if jsonOutput {
			return out.JSON(map[string]any{key: value})
		}

		out.Println(fmt.Sprint(value))
		return nil
	}

	// Show all configuration
	if jsonOutput {
		return out.JSON(globalConfig)
	}

	if showSources {
		// TODO: Implement source tracking in future enhancement
		out.Info("Note: Source tracking not yet implemented")
		out.Println("")
	}

	// Output as TOML
	data, err := toml.Marshal(globalConfig)
	if err != nil {
		out.Error(fmt.Sprintf("failed to marshal configuration: %v", err))
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	out.Print(string(data))
	return nil
}
