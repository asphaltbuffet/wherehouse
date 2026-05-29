package doctor

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

// NewDefaultDoctorCmd returns the doctor command wired to the real database.
func NewDefaultDoctorCmd() *cobra.Command {
	cmd := buildDoctorCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		cfg, ok := cli.GetConfig(cmd.Context())
		if !ok {
			cfg = config.GetDefaults()
		}

		out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

		hasIssues := runConfigCheck(out, cfg)
		if hasIssues {
			return errors.New("doctor found issues")
		}

		s, a, err := cli.OpenDatabase(cmd.Context())
		if err != nil {
			out.Issue("config", fmt.Sprintf("cannot open database: %v", err))
			return errors.New("doctor found issues")
		}
		defer s.Close()

		return runDoctor(cmd, args, a)
	}
	return cmd
}

// NewDoctorCmd returns the doctor command with a provided doctorApp for testing.
func NewDoctorCmd(a doctorApp) *cobra.Command {
	cmd := buildDoctorCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return runDoctor(cmd, args, a) }
	return cmd
}

func buildDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check inventory health",
		Long: `Run a series of checks to diagnose configuration and database integrity issues.

Checks (in order):
  1. Config file validation and DB path resolution
  2. Event log structural validation
  3. Projection consistency check

Use --rebuild to rebuild the projection from the event log after all checks pass.
Use --rebuild --force to rebuild even when issues are found.

Examples:
  wherehouse doctor
  wherehouse doctor --rebuild
  wherehouse doctor --rebuild --force`,
		Args: cobra.NoArgs,
	}
	cmd.Flags().Bool("rebuild", false, "rebuild projection from event log after checks")
	cmd.Flags().BoolP("force", "f", false, "force rebuild even when issues are found (requires --rebuild)")
	return cmd
}

// runConfigCheck validates config files and DB path resolution.
// Returns true if any config issues were found.
func runConfigCheck(out *cli.OutputWriter, cfg *config.Config) bool {
	fs := afero.NewOsFs()
	hasIssues := false

	for _, path := range []string{config.GetGlobalConfigPath(), config.GetLocalConfigPath()} {
		if path == "" {
			continue
		}
		expanded, err := config.ExpandPath(path)
		if err != nil {
			out.Issue("config", fmt.Sprintf("cannot expand config path %q: %v", path, err))
			hasIssues = true
			continue
		}
		exists, err := aferoFileExists(fs, expanded)
		if err != nil {
			out.Issue("config", fmt.Sprintf("cannot access config %s: %v", expanded, err))
			hasIssues = true
			continue
		}
		if !exists {
			continue
		}
		if err = config.Check(fs, expanded); err != nil {
			out.Issue("config", fmt.Sprintf("invalid config %s: %v", expanded, err))
			hasIssues = true
		}
	}

	if _, err := cfg.GetDatabasePath(); err != nil {
		out.Issue("config", fmt.Sprintf("cannot resolve database path: %v", err))
		hasIssues = true
	}

	return hasIssues
}

func aferoFileExists(fs afero.Fs, path string) (bool, error) {
	_, err := fs.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func runDoctor(cmd *cobra.Command, _ []string, a doctorApp) error {
	ctx := cmd.Context()
	rebuild, _ := cmd.Flags().GetBool("rebuild")
	force, _ := cmd.Flags().GetBool("force")

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	hasIssues := false

	eventIssues, err := a.ValidateEventLog(ctx)
	if err != nil {
		return fmt.Errorf("doctor: validate event log: %w", err)
	}
	for _, issue := range eventIssues {
		out.Issue(issue.Kind.String(), issue.Description)
		hasIssues = true
	}

	projIssues, err := a.CheckProjectionConsistency(ctx)
	if err != nil {
		return fmt.Errorf("doctor: check projection: %w", err)
	}
	for _, issue := range projIssues {
		out.Issue(issue.Kind.String(), issue.Description)
		hasIssues = true
	}

	if rebuild && (force || !hasIssues) {
		var replayCount int
		replayCount, err = a.TruncateAndReplay(ctx)
		if err != nil {
			return fmt.Errorf("doctor: rebuild: %w", err)
		}
		out.Success(fmt.Sprintf("Rebuilt projection from %d events.", replayCount))
		return nil
	}

	if hasIssues {
		return errors.New("doctor found issues")
	}

	out.Success("OK")
	return nil
}
