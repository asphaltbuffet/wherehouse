package cli

import "github.com/spf13/cobra"

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
