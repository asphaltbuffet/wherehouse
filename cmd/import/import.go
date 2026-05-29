package importcmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
)

// NewDefaultImportCmd returns the import command wired to the real database.
func NewDefaultImportCmd() *cobra.Command {
	cmd := buildImportCmd()
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		s, a, err := cli.OpenDatabase(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer s.Close()
		return runImport(cmd, a)
	}
	return cmd
}

// NewImportCmd returns the import command using the supplied importApp (for testing).
func NewImportCmd(a importApp) *cobra.Command {
	cmd := buildImportCmd()
	cmd.RunE = func(cmd *cobra.Command, _ []string) error { return runImport(cmd, a) }
	return cmd
}

func buildImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import events from an NDJSON file",
		Long: `Import events from an NDJSON export file into the database.

Reads from stdin by default; use --file to read from a named file.

Examples:
  wherehouse import --file backup.ndjson
  cat backup.ndjson | wherehouse import`,
	}
	cmd.Flags().StringP("file", "f", "", "path to NDJSON file (default: stdin)")
	cmd.Flags().Bool("replace", false, "clear all existing data before import")
	cmd.Flags().Bool("yes", false, "confirm destructive --replace operation")
	return cmd
}

func runImport(cmd *cobra.Command, a importApp) error {
	ctx := cmd.Context()

	filePath, _ := cmd.Flags().GetString("file")
	replace, _ := cmd.Flags().GetBool("replace")
	yes, _ := cmd.Flags().GetBool("yes")

	var r = cmd.InOrStdin()
	if filePath != "" {
		f, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("open %q: %w", filePath, err)
		}
		defer f.Close()
		r = f
	}

	if replace && !yes {
		has, err := a.HasEvents(ctx)
		if err != nil {
			return fmt.Errorf("check existing data: %w", err)
		}
		if has {
			return errors.New("database is not empty; pass --yes to confirm replacement")
		}
	}

	var events []app.ExportResult
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev app.ExportResult
		if err := json.Unmarshal(line, &ev); err != nil {
			return fmt.Errorf("malformed JSON: %w", err)
		}
		events = append(events, ev)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	result, err := a.ImportEvents(ctx, events, app.ImportOptions{
		Replace: replace && yes,
	})
	if err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	if !cli.IsQuietMode(cmd) {
		fmt.Fprintf(cmd.ErrOrStderr(), "Replayed: %d, Failed: %d, Warnings: %d\n",
			result.Replayed, result.Failed, result.Warnings)
	}

	return nil
}
