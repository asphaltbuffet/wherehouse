package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/config"
)

// IsQuietMode checks if quiet mode is enabled.
// Returns true if the quiet flag count is greater than 0.
// Quiet levels:
//   - 0: Normal output (default)
//   - 1: Minimal output (-q)
//   - 2+: Silent output (-qq or more)
func IsQuietMode(cmd *cobra.Command) bool {
	quiet, _ := cmd.Flags().GetCount("quiet")
	return quiet > 0
}

// GetConfig retrieves the Config from the context.
// Returns the config and true if found, or nil and false if not present.
func GetConfig(ctx context.Context) (*config.Config, bool) {
	v := ctx.Value(config.ConfigKey)
	cfg, ok := v.(*config.Config)
	return cfg, ok
}

// MustGetConfig retrieves the Config from the context or panics if not found.
// Use this when the config is guaranteed to be present (e.g., after PersistentPreRunE).
func MustGetConfig(ctx context.Context) *config.Config {
	cfg, ok := GetConfig(ctx)
	if !ok {
		panic("wherehouse: Config not found in context -- was PersistentPreRunE bypassed?")
	}
	return cfg
}
