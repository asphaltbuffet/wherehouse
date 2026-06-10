package add

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// NewDefaultAddCmd returns the add command wired to a real database opened from context config.
func NewDefaultAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <path> [path...] | --file <csv>",
		Short: "Add one or more entities to the inventory",
		Long: `Add one or more entities to the inventory.

Examples:
  wherehouse add "Toolbox"
  wherehouse add "Garage:Toolbox:Wrench"
  wherehouse add --file items.csv
  wherehouse add --file items.csv --create-parents`,
		Args: validateAddArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, a, err := cli.OpenDatabase(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to open database: %w", err)
			}
			defer s.Close()
			return runAdd(cmd, args, a)
		},
	}
	cmd.Flags().Bool("locked", false, "Lock the entity (prevent direct reparenting)")
	cmd.Flags().Bool("discrete", false, "Mark the entity as discrete (no children allowed)")
	cmd.Flags().Bool("allow-duplicates", false, "Allow duplicate names within the batch")
	cmd.Flags().StringP("file", "f", "", "CSV file of entities to bulk-add")
	cmd.Flags().Bool("create-parents", false, "Create missing ancestor entities automatically")
	cmd.Flags().BoolP("verbose", "v", false, "Print one line per created entity")
	return cmd
}

// NewAddCmd returns the add command wired to the provided addApp (for testing).
func NewAddCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <path> [path...] | --file <csv>",
		Short: "Add one or more entities to the inventory",
		Args:  validateAddArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(cmd, args, a)
		},
	}
	cmd.Flags().Bool("locked", false, "Lock the entity (prevent direct reparenting)")
	cmd.Flags().Bool("discrete", false, "Mark the entity as discrete (no children allowed)")
	cmd.Flags().Bool("allow-duplicates", false, "Allow duplicate names within the batch")
	cmd.Flags().StringP("file", "f", "", "CSV file of entities to bulk-add")
	cmd.Flags().Bool("create-parents", false, "Create missing ancestor entities automatically")
	cmd.Flags().BoolP("verbose", "v", false, "Print one line per created entity")
	return cmd
}

func runAdd(cmd *cobra.Command, args []string, a *app.App) error {
	ctx := cmd.Context()
	filePath, _ := cmd.Flags().GetString("file")

	if filePath != "" {
		return runAddFile(cmd, a, filePath)
	}

	locked, _ := cmd.Flags().GetBool("locked")
	discrete, _ := cmd.Flags().GetBool("discrete")
	allowDupes, _ := cmd.Flags().GetBool("allow-duplicates")

	if !allowDupes {
		seen := make(map[string]struct{}, len(args))
		for _, arg := range args {
			p, parseErr := entitypath.Parse(arg)
			if parseErr != nil {
				return fmt.Errorf("failed to add %q: %w", arg, parseErr)
			}
			key := inventory.CanonicalizeString(p.Dir().String()) + ":" + inventory.CanonicalizeString(p.Base())
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate name %q in batch (use --allow-duplicates to override)", p.Base())
			}
			seen[key] = struct{}{}
		}
	}

	reqs := make([]app.CreateEntityRequest, 0, len(args))
	for _, arg := range args {
		p, parseErr := entitypath.Parse(arg)
		if parseErr != nil {
			return fmt.Errorf("failed to add %q: %w", arg, parseErr)
		}
		name := p.Base()
		if name == "" {
			return fmt.Errorf("cannot determine entity name from %q", arg)
		}
		reqs = append(reqs, app.CreateEntityRequest{
			DisplayName: name,
			Locked:      locked,
			Discrete:    discrete,
			ParentPath:  p.Dir().String(),
			ActorID:     cli.GetActorUserID(ctx),
		})
	}

	results, err := a.CreateEntities(ctx, reqs)
	if err != nil {
		return err
	}

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	if out.IsJSON() {
		return out.JSON(app.ToAddOutputs(results))
	}

	for _, result := range results {
		out.Success(fmt.Sprintf("Added %q ID: %s", result.FullPathDisplay, result.EntityID))
	}
	return nil
}

func runAddFile(cmd *cobra.Command, a *app.App, filePath string) error {
	ctx := cmd.Context()

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file %q: %w", filePath, err)
	}
	defer f.Close()

	allowDupes, _ := cmd.Flags().GetBool("allow-duplicates")

	rows, err := app.ParseBulkCSV(f, allowDupes)
	if err != nil {
		return fmt.Errorf("parse %q: %w", filePath, err)
	}
	createParents, _ := cmd.Flags().GetBool("create-parents")
	verbose, _ := cmd.Flags().GetBool("verbose")

	result, err := a.BulkAddEntities(ctx, rows, app.BulkAddOptions{
		AllowDuplicates: allowDupes,
		CreateParents:   createParents,
		ActorID:         cli.GetActorUserID(ctx),
	})
	if err != nil {
		return err
	}

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}

	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	if out.IsJSON() {
		return out.JSON(app.ToBulkAddOutput(result))
	}

	if cfg.IsQuiet() {
		return nil
	}

	for _, w := range result.Warnings {
		out.Warning(w)
	}

	if verbose {
		for _, e := range result.Created {
			out.Success(fmt.Sprintf("Added %q ID: %s", e.FullPathDisplay, e.EntityID))
		}
	}

	warningClause := ""
	if len(result.Warnings) > 0 {
		warningClause = fmt.Sprintf(" (%s)", pluralize(len(result.Warnings), "warning"))
	}
	out.Success(fmt.Sprintf("Created %s%s.", pluralize(len(result.Created), "entity", "entities"), warningClause))
	return nil
}

func validateAddArgs(cmd *cobra.Command, args []string) error {
	filePath, _ := cmd.Flags().GetString("file")
	hasFile := filePath != ""

	if hasFile && len(args) > 0 {
		return errors.New("--file cannot be combined with positional arguments")
	}
	if hasFile {
		locked, _ := cmd.Flags().GetBool("locked")
		discrete, _ := cmd.Flags().GetBool("discrete")
		if locked || discrete {
			return errors.New("--file cannot be combined with --locked or --discrete (set these per-row in the CSV)")
		}
		return nil
	}
	if len(args) == 0 {
		return errors.New("requires at least one path argument, or use --file <csv>")
	}
	return nil
}

func pluralize(n int, singular string, pluralOverride ...string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	plural := singular + "s"
	if len(pluralOverride) > 0 {
		plural = pluralOverride[0]
	}
	return fmt.Sprintf("%d %s", n, plural)
}
