package importcmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/config"
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
func NewImportCmd(a *app.App) *cobra.Command {
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
	cmd.Flags().Bool("continue", false, "continue on per-event errors, accumulating failures")
	return cmd
}

func runImport(cmd *cobra.Command, a *app.App) error {
	ctx := cmd.Context()

	filePath, _ := cmd.Flags().GetString("file")
	replace, _ := cmd.Flags().GetBool("replace")
	yes, _ := cmd.Flags().GetBool("yes")
	cont, _ := cmd.Flags().GetBool("continue")

	if err := checkReplaceFlags(ctx, a, replace, yes); err != nil {
		return err
	}

	r, closer, err := openInput(cmd, filePath)
	if err != nil {
		return err
	}
	defer closer()

	events, err := parseNDJSON(r)
	if err != nil {
		return err
	}

	result, err := a.ImportEvents(ctx, events, app.ImportOptions{
		Continue: cont,
		Replace:  replace && yes,
	})
	if err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	cfg, ok := cli.GetConfig(ctx)
	if !ok {
		cfg = config.GetDefaults()
	}

	if !cfg.IsQuiet() {
		out := cmd.ErrOrStderr()
		fmt.Fprintf(out, "Replayed: %d, Failed: %d, Warnings: %d\n",
			result.Replayed, result.Failed, result.WarningCount)
		for _, w := range result.Warnings {
			fmt.Fprintf(out, "  warning: %s\n", w)
		}
	}

	return nil
}

func checkReplaceFlags(ctx context.Context, a *app.App, replace, yes bool) error {
	if replace && !yes {
		return errors.New("--replace requires --yes to confirm the destructive operation")
	}
	if !replace {
		has, err := a.HasEvents(ctx)
		if err != nil {
			return fmt.Errorf("check existing data: %w", err)
		}
		if has {
			return errors.New("database is not empty; use --replace --yes to overwrite")
		}
	}
	return nil
}

func openInput(cmd *cobra.Command, filePath string) (io.Reader, func(), error) {
	if filePath == "" {
		return cmd.InOrStdin(), func() {}, nil
	}
	f, err := os.Open(filePath)
	if err != nil {
		return nil, func() {}, fmt.Errorf("open %q: %w", filePath, err)
	}
	return f, func() { _ = f.Close() }, nil
}

func parseNDJSON(r io.Reader) ([]app.ExportResult, error) {
	var events []app.ExportResult
	scanner := bufio.NewScanner(r)
	// Per-line cap: 16 MiB. The default 64 KiB is too tight — a single Event
	// with a long human-typed Note can exceed it. 16 MiB is 4-5 orders of
	// magnitude above realistic events while still failing loudly on garbage
	// files (binary blobs masquerading as NDJSON). See CONTEXT.md "Note".
	const (
		initialBuf = 64 << 10 // 64 KiB — Scanner default; grows on demand up to maxLine.
		maxLine    = 16 << 20 // 16 MiB.
	)
	scanner.Buffer(make([]byte, initialBuf), maxLine)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev app.ExportResult
		if err := json.Unmarshal(line, &ev); err != nil {
			return nil, fmt.Errorf("malformed JSON: %w", err)
		}
		events = append(events, ev)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}
	return events, nil
}
