package doctor

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
)

// NewDefaultDoctorCmd returns the doctor command wired to the real database.
// NewDefaultDoctorCmd returns the doctor command wired to the real database.
type doctorResult struct {
	Healthy    bool          `json:"healthy"`
	IssueCount int           `json:"issue_count"`
	Issues     []issueResult `json:"issues"`
	Rebuilt    *int          `json:"rebuilt,omitempty"`
}

type issueResult struct {
	Kind        string `json:"kind"`
	EventID     *int64 `json:"event_id"`
	Description string `json:"description"`
}

// NewDefaultDoctorCmd returns the doctor command wired to the real database.
func NewDefaultDoctorCmd() *cobra.Command {
	cmd := buildDoctorCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		cfg, ok := cli.GetConfig(cmd.Context())
		if !ok {
			cfg = config.GetDefaults()
		}

		configIssues := runConfigCheck(cfg)

		s, a, dbErr := cli.OpenDatabase(cmd.Context())
		if dbErr != nil {
			configIssues = append(configIssues, app.DoctorIssue{
				Kind:        app.DoctorKindConfig,
				Description: fmt.Sprintf("cannot open database: %v", dbErr),
			})
			return runDoctor(cmd, args, configIssues, nil)
		}
		defer s.Close()

		return runDoctor(cmd, args, configIssues, a)
	}
	return cmd
}

// NewDoctorCmd returns the doctor command with a provided doctorApp for testing.
// NewDoctorCmd returns the doctor command with a provided doctorApp for testing.
func NewDoctorCmd(a *app.App) *cobra.Command {
	cmd := buildDoctorCmd()
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return runDoctor(cmd, args, nil, a) }
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
		Args:         cobra.NoArgs,
		SilenceUsage: true,
	}
	cmd.Flags().Bool("rebuild", false, "rebuild projection from event log after checks")
	cmd.Flags().BoolP("force", "f", false, "force rebuild even when issues are found (requires --rebuild)")
	return cmd
}

// runConfigCheck validates config files and DB path resolution.
// Returns true if any config issues were found.
// runConfigCheck validates config files and DB path resolution.
// Returns any config issues found.
func runConfigCheck(cfg *config.Config) []app.DoctorIssue {
	fs := afero.NewOsFs()
	var issues []app.DoctorIssue

	for _, path := range []string{config.GetGlobalConfigPath(), config.GetLocalConfigPath()} {
		if path == "" {
			continue
		}
		expanded, err := config.ExpandPath(path)
		if err != nil {
			issues = append(issues, app.DoctorIssue{
				Kind:        app.DoctorKindConfig,
				Description: fmt.Sprintf("cannot expand config path %q: %v", path, err),
			})
			continue
		}
		exists, err := afero.Exists(fs, expanded)
		if err != nil {
			issues = append(issues, app.DoctorIssue{
				Kind:        app.DoctorKindConfig,
				Description: fmt.Sprintf("cannot access config %s: %v", expanded, err),
			})
			continue
		}
		if !exists {
			continue
		}
		if err = config.Check(fs, expanded); err != nil {
			issues = append(issues, app.DoctorIssue{
				Kind:        app.DoctorKindConfig,
				Description: fmt.Sprintf("invalid config %s: %v", expanded, err),
			})
		}
	}

	if _, err := cfg.GetDatabasePath(); err != nil {
		issues = append(issues, app.DoctorIssue{
			Kind:        app.DoctorKindConfig,
			Description: fmt.Sprintf("cannot resolve database path: %v", err),
		})
	}

	return issues
}

func runDoctor(cmd *cobra.Command, _ []string, configIssues []app.DoctorIssue, a *app.App) error {
	ctx := cmd.Context()
	rebuild, _ := cmd.Flags().GetBool("rebuild")
	force, _ := cmd.Flags().GetBool("force")

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}
	out := cli.NewOutputWriter(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg.IsJSON(), cfg.IsQuiet())

	allIssues, err := collectIssues(ctx, configIssues, a)
	if err != nil {
		return err
	}

	var replayCount *int
	if rebuild && (force || len(allIssues) == 0) && a != nil {
		count, rebuildErr := a.TruncateAndReplay(ctx)
		if rebuildErr != nil {
			return fmt.Errorf("doctor: rebuild: %w", rebuildErr)
		}
		replayCount = &count
	}

	return emitResult(out, allIssues, replayCount)
}

func collectIssues(ctx context.Context, configIssues []app.DoctorIssue, a *app.App) ([]app.DoctorIssue, error) {
	all := make([]app.DoctorIssue, 0, len(configIssues))
	all = append(all, configIssues...)
	if a == nil {
		return all, nil
	}
	checks, err := a.RunDoctorChecks(ctx)
	if err != nil {
		return nil, fmt.Errorf("doctor: %w", err)
	}
	return append(all, checks...), nil
}

func emitResult(out *cli.OutputWriter, allIssues []app.DoctorIssue, replayCount *int) error {
	hasIssues := len(allIssues) > 0

	if out.IsJSON() {
		issues := make([]issueResult, len(allIssues))
		for i, issue := range allIssues {
			issues[i] = issueResult{Kind: issue.Kind.String(), EventID: issue.EventID, Description: issue.Description}
		}
		if err := out.JSON(doctorResult{
			Healthy:    !hasIssues,
			IssueCount: len(allIssues),
			Issues:     issues,
			Rebuilt:    replayCount,
		}); err != nil {
			return err
		}
	} else {
		for _, issue := range allIssues {
			out.Issue(issue.Kind.String(), issue.Description)
		}
		if replayCount != nil {
			out.Success(fmt.Sprintf("Rebuilt projection from %d events.", *replayCount))
		} else if !hasIssues {
			out.Success("OK")
		}
	}

	if hasIssues {
		return errors.New("doctor found issues")
	}
	return nil
}
