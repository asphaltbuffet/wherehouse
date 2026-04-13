package load

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/internal/config"
)

// ctxWithConfig returns a context with a default (zero-value) Config injected.
// This satisfies MustGetConfig without requiring a real config file.
func ctxWithConfig() context.Context {
	return context.WithValue(context.Background(), config.ConfigKey, &config.Config{})
}

func TestNewLoadCmd(t *testing.T) {
	cmd := NewLoadCmd()
	require.NotNil(t, cmd)
}

func TestRunLoadCore_NoArgs(t *testing.T) {
	// cobra MinimumNArgs(1) should reject zero args before RunE is called.
	cmd := NewLoadCmd()
	cmd.SetArgs([]string{})
	err := cmd.ExecuteContext(ctxWithConfig())
	require.Error(t, err)
}

func TestRunLoadCore_FileNotFound(t *testing.T) {
	cmd := NewLoadCmd()
	cmd.SetArgs([]string{"/nonexistent/file.csv"})
	err := cmd.ExecuteContext(ctxWithConfig())
	require.Error(t, err)
}

func TestRunLoadCore_WrongExtension(t *testing.T) {
	f := filepath.Join(t.TempDir(), "data.txt")
	require.NoError(t, os.WriteFile(f, []byte(""), 0o600))
	cmd := NewLoadCmd()
	cmd.SetArgs([]string{f})
	err := cmd.ExecuteContext(ctxWithConfig())
	require.Error(t, err)
}
