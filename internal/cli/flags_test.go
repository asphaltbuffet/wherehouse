package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestIsQuietMode(t *testing.T) {
	tests := []struct {
		name       string
		quietCount int
		want       bool
	}{
		{
			name:       "default no quiet flag",
			quietCount: 0,
			want:       false,
		},
		{
			name:       "quiet once (-q)",
			quietCount: 1,
			want:       true,
		},
		{
			name:       "quiet twice (-qq)",
			quietCount: 2,
			want:       true,
		},
		{
			name:       "quiet three times (-qqq)",
			quietCount: 3,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test command with quiet flag
			cmd := &cobra.Command{
				Use: "test",
				RunE: func(cmd *cobra.Command, _ []string) error {
					// Test IsQuietMode inside RunE where flags are parsed
					got := IsQuietMode(cmd)
					if got != tt.want {
						t.Errorf("IsQuietMode() = %v, want %v", got, tt.want)
					}
					return nil
				},
			}
			cmd.Flags().CountP("quiet", "q", "Quiet output")

			// Build command line args
			var cmdArgs []string
			for range tt.quietCount {
				cmdArgs = append(cmdArgs, "-q")
			}
			cmd.SetArgs(cmdArgs)

			// Execute command (this parses flags and runs RunE)
			if err := cmd.Execute(); err != nil {
				t.Errorf("Command execution failed: %v", err)
			}
		})
	}
}

func TestIsQuietMode_NoFlag(t *testing.T) {
	// Test behavior when quiet flag is not defined at all
	cmd := &cobra.Command{
		Use: "test",
	}
	// No quiet flag defined

	got := IsQuietMode(cmd)
	if got != false {
		t.Errorf("IsQuietMode() without quiet flag = %v, want false", got)
	}
}

func TestIsQuietMode_Integration(t *testing.T) {
	// Integration test simulating real command usage
	t.Run("simulated command with -q flag", func(t *testing.T) {
		cmd := &cobra.Command{
			Use: "move",
			RunE: func(cmd *cobra.Command, _ []string) error {
				if !IsQuietMode(cmd) {
					t.Error("Expected quiet mode to be enabled")
				}
				return nil
			},
		}
		cmd.Flags().CountP("quiet", "q", "Quiet output")

		// Simulate user running: command -q
		cmd.SetArgs([]string{"-q"})
		if err := cmd.Execute(); err != nil {
			t.Errorf("Command execution failed: %v", err)
		}
	})

	t.Run("simulated command with -qq flag", func(t *testing.T) {
		cmd := &cobra.Command{
			Use: "add",
			RunE: func(cmd *cobra.Command, _ []string) error {
				if !IsQuietMode(cmd) {
					t.Error("Expected quiet mode to be enabled")
				}
				return nil
			},
		}
		cmd.Flags().CountP("quiet", "q", "Quiet output")

		// Simulate user running: command -qq
		cmd.SetArgs([]string{"-qq"})
		if err := cmd.Execute(); err != nil {
			t.Errorf("Command execution failed: %v", err)
		}
	})

	t.Run("simulated command without quiet flag", func(t *testing.T) {
		cmd := &cobra.Command{
			Use: "list",
			RunE: func(cmd *cobra.Command, _ []string) error {
				if IsQuietMode(cmd) {
					t.Error("Expected quiet mode to be disabled")
				}
				return nil
			},
		}
		cmd.Flags().CountP("quiet", "q", "Quiet output")

		// Simulate user running: command (no flags)
		cmd.SetArgs([]string{})
		if err := cmd.Execute(); err != nil {
			t.Errorf("Command execution failed: %v", err)
		}
	})
}
