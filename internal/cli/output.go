package cli

import (
	"fmt"
	"io"

	"github.com/goccy/go-json"

	"charm.land/lipgloss/v2"

	"github.com/asphaltbuffet/wherehouse/internal/config"
)

// OutputWriter handles formatted output with styling support.
// It respects JSON mode and quiet mode flags for consistent output behavior.
type OutputWriter struct {
	out       io.Writer
	err       io.Writer
	jsonMode  bool
	quietMode bool
	styles    *OutputStyles
}

// OutputStyles contains lipgloss styles for consistent terminal formatting.
type OutputStyles struct {
	Success lipgloss.Style
	Error   lipgloss.Style
	Warning lipgloss.Style
	Info    lipgloss.Style
	Key     lipgloss.Style
	Value   lipgloss.Style
}

// NewOutputWriterFromConfig creates an output writer using settings from a Config.
// Delegates to NewOutputWriter with cfg.IsJSON() and cfg.IsQuiet().
func NewOutputWriterFromConfig(out, err io.Writer, cfg *config.Config) *OutputWriter {
	return NewOutputWriter(out, err, cfg.IsJSON(), cfg.IsQuiet())
}

// NewOutputWriter creates an output writer with default lipgloss styles.
//
// Parameters:
//   - out: Standard output writer (typically cmd.OutOrStdout())
//   - err: Error output writer (typically cmd.ErrOrStderr())
//   - jsonMode: If true, output is formatted as JSON
//   - quietMode: If true, suppress non-essential output
func NewOutputWriter(out, err io.Writer, jsonMode, quietMode bool) *OutputWriter {
	return &OutputWriter{
		out:       out,
		err:       err,
		jsonMode:  jsonMode,
		quietMode: quietMode,
		styles:    defaultStyles(),
	}
}

// defaultStyles creates the default lipgloss style set for terminal output.
func defaultStyles() *OutputStyles {
	return &OutputStyles{
		Success: lipgloss.NewStyle().Foreground(lipgloss.Color("10")),            // Green
		Error:   lipgloss.NewStyle().Foreground(lipgloss.Color("9")),             // Red
		Warning: lipgloss.NewStyle().Foreground(lipgloss.Color("11")),            // Yellow
		Info:    lipgloss.NewStyle().Foreground(lipgloss.Color("12")),            // Blue
		Key:     lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true), // Cyan bold
		Value:   lipgloss.NewStyle().Foreground(lipgloss.Color("15")),            // White
	}
}

// Success prints a success message to stdout.
// Suppressed in quiet mode. In JSON mode, outputs {"status":"success","message":"..."}.
func (w *OutputWriter) Success(msg string) {
	if w.quietMode {
		return
	}
	if w.jsonMode {
		_ = w.printJSON(map[string]string{"status": "success", "message": msg})
		return
	}
	fmt.Fprintln(w.out, w.styles.Success.Render(msg))
}

// Error prints an error message to stderr.
// Always shown (ignores quiet mode). In JSON mode, outputs {"status":"error","message":"..."}.
func (w *OutputWriter) Error(msg string) {
	if w.jsonMode {
		_ = w.printJSON(map[string]string{"status": "error", "message": msg})
		return
	}
	fmt.Fprintln(w.err, w.styles.Error.Render("Error: "+msg))
}

// Warning prints a warning message to stderr.
// Suppressed in quiet mode. In JSON mode, outputs {"status":"warning","message":"..."}.
func (w *OutputWriter) Warning(msg string) {
	if w.quietMode {
		return
	}
	if w.jsonMode {
		_ = w.printJSON(map[string]string{"status": "warning", "message": msg})
		return
	}
	fmt.Fprintln(w.err, w.styles.Warning.Render("Warning: "+msg))
}

// Info prints an informational message to stdout.
// Suppressed in quiet mode. Not shown in JSON mode.
func (w *OutputWriter) Info(msg string) {
	if w.quietMode {
		return
	}
	if w.jsonMode {
		return // Don't print info in JSON mode
	}
	fmt.Fprintln(w.out, w.styles.Info.Render(msg))
}

// KeyValue prints a key-value pair with styled formatting.
// Suppressed in quiet mode. In JSON mode, outputs {key:value}.
func (w *OutputWriter) KeyValue(key, value string) {
	if w.quietMode {
		return
	}
	if w.jsonMode {
		_ = w.printJSON(map[string]string{key: value})
		return
	}
	fmt.Fprintf(w.out, "%s: %s\n",
		w.styles.Key.Render(key),
		w.styles.Value.Render(value))
}

// JSON prints arbitrary JSON data to stdout.
// Always outputs JSON regardless of mode flags.
func (w *OutputWriter) JSON(data any) error {
	return w.printJSON(data)
}

// Print prints plain text to stdout without styling or newline.
// Bypasses quiet mode (always prints). Use for scripting-friendly output.
func (w *OutputWriter) Print(msg string) {
	fmt.Fprint(w.out, msg)
}

// Println prints plain text to stdout without styling, with newline.
// Bypasses quiet mode (always prints). Use for scripting-friendly output.
func (w *OutputWriter) Println(msg string) {
	fmt.Fprintln(w.out, msg)
}

// printJSON encodes and writes JSON data to stdout with indentation.
func (w *OutputWriter) printJSON(data any) error {
	enc := json.NewEncoder(w.out)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}
