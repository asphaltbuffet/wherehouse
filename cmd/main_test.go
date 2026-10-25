package cmd

import (
	"os"
	"testing"
)

// TestMain isolates tests from the developer's configuration: no real config
// files or WHEREHOUSE_* variables leak in. Tests set their own via t.Setenv.
func TestMain(m *testing.M) {
	for _, k := range []string{
		"WHEREHOUSE_CONFIG",
		"WHEREHOUSE_DATABASE_PATH",
		"WHEREHOUSE_LOG_PATH",
		"WHEREHOUSE_LOGGING_FILE_PATH",
		"WHEREHOUSE_LOGGING_LEVEL",
		"WHEREHOUSE_LOGGING_MAX_BACKUPS",
		"WHEREHOUSE_LOGGING_MAX_SIZE_MB",
		"WHEREHOUSE_OUTPUT_DEFAULT_FORMAT",
	} {
		_ = os.Unsetenv(k)
	}

	dir, err := os.MkdirTemp("", "wherehouse-test-xdg")
	if err != nil {
		panic(err)
	}
	_ = os.Setenv("XDG_CONFIG_HOME", dir)

	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
